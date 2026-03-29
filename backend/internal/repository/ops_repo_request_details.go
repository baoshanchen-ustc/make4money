package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *opsRepository) ListRequestDetails(ctx context.Context, filter *service.OpsRequestDetailFilter) ([]*service.OpsRequestDetail, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("nil ops repository")
	}

	page, pageSize, startTime, endTime := filter.Normalize()
	offset := (page - 1) * pageSize

	conditions := make([]string, 0, 16)
	args := make([]any, 0, 24)

	// Placeholders $1/$2 reserved for time window inside the CTE.
	args = append(args, startTime.UTC(), endTime.UTC())

	addCondition := func(condition string, values ...any) {
		conditions = append(conditions, condition)
		args = append(args, values...)
	}

	// Determine anomaly thresholds — used both in WHERE and in the computed column.
	slowMs := int64(20000)
	timeoutMs := int64(60000)
	if filter != nil && filter.AnomalySettingsForFilter != nil {
		slowMs = filter.AnomalySettingsForFilter.SlowRequestThresholdMs
		timeoutMs = filter.AnomalySettingsForFilter.TimeoutThresholdMs
	}

	if filter != nil {
		if kind := strings.TrimSpace(strings.ToLower(filter.Kind)); kind != "" && kind != "all" {
			if kind != string(service.OpsRequestKindSuccess) && kind != string(service.OpsRequestKindError) {
				return nil, 0, fmt.Errorf("invalid kind")
			}
			addCondition(fmt.Sprintf("kind = $%d", len(args)+1), kind)
		}

		if platform := strings.TrimSpace(strings.ToLower(filter.Platform)); platform != "" {
			addCondition(fmt.Sprintf("platform = $%d", len(args)+1), platform)
		}
		if filter.GroupID != nil && *filter.GroupID > 0 {
			addCondition(fmt.Sprintf("group_id = $%d", len(args)+1), *filter.GroupID)
		}

		if filter.UserID != nil && *filter.UserID > 0 {
			addCondition(fmt.Sprintf("user_id = $%d", len(args)+1), *filter.UserID)
		}
		if filter.APIKeyID != nil && *filter.APIKeyID > 0 {
			addCondition(fmt.Sprintf("api_key_id = $%d", len(args)+1), *filter.APIKeyID)
		}
		if filter.AccountID != nil && *filter.AccountID > 0 {
			addCondition(fmt.Sprintf("account_id = $%d", len(args)+1), *filter.AccountID)
		}

		if model := strings.TrimSpace(filter.Model); model != "" {
			addCondition(fmt.Sprintf("model = $%d", len(args)+1), model)
		}
		if requestID := strings.TrimSpace(filter.RequestID); requestID != "" {
			addCondition(fmt.Sprintf("request_id = $%d", len(args)+1), requestID)
		}
		if q := strings.TrimSpace(filter.Query); q != "" {
			like := "%" + strings.ToLower(q) + "%"
			startIdx := len(args) + 1
			addCondition(
				fmt.Sprintf("(LOWER(COALESCE(request_id,'')) LIKE $%d OR LOWER(COALESCE(model,'')) LIKE $%d OR LOWER(COALESCE(message,'')) LIKE $%d)",
					startIdx, startIdx+1, startIdx+2,
				),
				like, like, like,
			)
		}

		if filter.MinDurationMs != nil {
			addCondition(fmt.Sprintf("duration_ms >= $%d", len(args)+1), *filter.MinDurationMs)
		}
		if filter.MaxDurationMs != nil {
			addCondition(fmt.Sprintf("duration_ms <= $%d", len(args)+1), *filter.MaxDurationMs)
		}

		// AnomalyTypes filter: rows must match at least one of the given anomaly types (OR logic).
		if len(filter.AnomalyTypes) > 0 {
			anomalyConditions := make([]string, 0, len(filter.AnomalyTypes))
			for _, at := range filter.AnomalyTypes {
				switch service.AnomalyType(at) {
				case service.AnomalyZeroToken:
					anomalyConditions = append(anomalyConditions, "(input_tokens = 0 AND output_tokens = 0)")
				case service.AnomalySlowRequest:
					anomalyConditions = append(anomalyConditions,
						fmt.Sprintf("(duration_ms > %d AND duration_ms <= %d)", slowMs, timeoutMs))
				case service.AnomalyTimeout:
					anomalyConditions = append(anomalyConditions,
						fmt.Sprintf("(duration_ms > %d)", timeoutMs))
				case service.AnomalyError:
					anomalyConditions = append(anomalyConditions, "(status_code >= 500)")
				}
			}
			if len(anomalyConditions) > 0 {
				conditions = append(conditions, "("+strings.Join(anomalyConditions, " OR ")+")")
			}
		}
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	cte := `
WITH combined AS (
  SELECT
    'success'::TEXT AS kind,
    ul.created_at AS created_at,
    ul.request_id AS request_id,
    COALESCE(NULLIF(g.platform, ''), NULLIF(a.platform, ''), '') AS platform,
    ul.model AS model,
    ul.duration_ms AS duration_ms,
    200 AS status_code,
    NULL::BIGINT AS error_id,
    NULL::TEXT AS phase,
    NULL::TEXT AS severity,
    NULL::TEXT AS message,
    ul.user_id AS user_id,
    ul.api_key_id AS api_key_id,
    ul.account_id AS account_id,
    ul.group_id AS group_id,
    ul.stream AS stream,
    ul.request_body_bytes AS request_body_bytes,
    ul.auth_latency_ms AS auth_latency_ms,
    ul.routing_latency_ms AS routing_latency_ms,
    ul.upstream_latency_ms AS upstream_latency_ms,
    ul.response_latency_ms AS response_latency_ms,
    u.username AS user_name,
    CASE WHEN ak.key IS NOT NULL THEN '***' || RIGHT(ak.key, 4) ELSE NULL END AS api_key_label,
    COALESCE(g.name, '') AS group_name,
    COALESCE(a.name, '') AS account_name,
    COALESCE(ul.input_tokens, 0) AS input_tokens,
    COALESCE(ul.output_tokens, 0) AS output_tokens
  FROM usage_logs ul
  LEFT JOIN groups g ON g.id = ul.group_id
  LEFT JOIN accounts a ON a.id = ul.account_id
  LEFT JOIN users u ON u.id = ul.user_id
  LEFT JOIN api_keys ak ON ak.id = ul.api_key_id
  WHERE ul.created_at >= $1 AND ul.created_at < $2

  UNION ALL

  SELECT
    'error'::TEXT AS kind,
    o.created_at AS created_at,
    COALESCE(NULLIF(o.request_id,''), NULLIF(o.client_request_id,''), '') AS request_id,
    COALESCE(NULLIF(o.platform, ''), NULLIF(g.platform, ''), NULLIF(a.platform, ''), '') AS platform,
    o.model AS model,
    o.duration_ms AS duration_ms,
    o.status_code AS status_code,
    o.id AS error_id,
    o.error_phase AS phase,
    o.severity AS severity,
    o.error_message AS message,
    o.user_id AS user_id,
    o.api_key_id AS api_key_id,
    o.account_id AS account_id,
    o.group_id AS group_id,
    o.stream AS stream,
    o.request_body_bytes AS request_body_bytes,
    NULL::INT AS auth_latency_ms,
    NULL::INT AS routing_latency_ms,
    NULL::INT AS upstream_latency_ms,
    NULL::INT AS response_latency_ms,
    u.username AS user_name,
    CASE WHEN ak.key IS NOT NULL THEN '***' || RIGHT(ak.key, 4) ELSE NULL END AS api_key_label,
    COALESCE(g.name, '') AS group_name,
    COALESCE(a.name, '') AS account_name,
    0 AS input_tokens,
    0 AS output_tokens
  FROM ops_error_logs o
  LEFT JOIN groups g ON g.id = o.group_id
  LEFT JOIN accounts a ON a.id = o.account_id
  LEFT JOIN users u ON u.id = o.user_id
  LEFT JOIN api_keys ak ON ak.id = o.api_key_id
  WHERE o.created_at >= $1 AND o.created_at < $2
    AND COALESCE(o.status_code, 0) >= 400
)
`

	countQuery := fmt.Sprintf(`%s SELECT COUNT(1) FROM combined %s`, cte, where)
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		if err == sql.ErrNoRows {
			total = 0
		} else {
			return nil, 0, err
		}
	}

	sort := "ORDER BY created_at DESC"
	if filter != nil {
		switch strings.TrimSpace(strings.ToLower(filter.Sort)) {
		case "", "created_at_desc":
			// default
		case "duration_desc":
			sort = "ORDER BY duration_ms DESC NULLS LAST, created_at DESC"
		default:
			return nil, 0, fmt.Errorf("invalid sort")
		}
	}

	listQuery := fmt.Sprintf(`
%s
SELECT
  kind,
  created_at,
  request_id,
  platform,
  model,
  duration_ms,
  status_code,
  error_id,
  phase,
  severity,
  message,
  user_id,
  api_key_id,
  account_id,
  group_id,
  stream,
  request_body_bytes,
  auth_latency_ms,
  routing_latency_ms,
  upstream_latency_ms,
  response_latency_ms,
  user_name,
  api_key_label,
  group_name,
  account_name,
  ARRAY_REMOVE(ARRAY[
    CASE WHEN input_tokens = 0 AND output_tokens = 0 THEN 'zero_token' ELSE NULL END,
    CASE WHEN duration_ms > %d AND duration_ms <= %d THEN 'slow_request' ELSE NULL END,
    CASE WHEN duration_ms > %d THEN 'timeout' ELSE NULL END,
    CASE WHEN status_code >= 500 THEN 'error' ELSE NULL END
  ], NULL) AS anomaly_types
FROM combined
%s
%s
LIMIT $%d OFFSET $%d
`, cte, slowMs, timeoutMs, timeoutMs, where, sort, len(args)+1, len(args)+2)

	listArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	toIntPtr := func(v sql.NullInt64) *int {
		if !v.Valid {
			return nil
		}
		i := int(v.Int64)
		return &i
	}
	toInt64Ptr := func(v sql.NullInt64) *int64 {
		if !v.Valid {
			return nil
		}
		i := v.Int64
		return &i
	}

	out := make([]*service.OpsRequestDetail, 0, pageSize)
	for rows.Next() {
		var (
			kind      string
			createdAt time.Time
			requestID sql.NullString
			platform  sql.NullString
			model     sql.NullString

			durationMs sql.NullInt64
			statusCode sql.NullInt64
			errorID    sql.NullInt64

			phase    sql.NullString
			severity sql.NullString
			message  sql.NullString

			userID    sql.NullInt64
			apiKeyID  sql.NullInt64
			accountID sql.NullInt64
			groupID   sql.NullInt64

			stream bool

			requestBodyBytes  sql.NullInt64
			authLatencyMs     sql.NullInt64
			routingLatencyMs  sql.NullInt64
			upstreamLatencyMs sql.NullInt64
			responseLatencyMs sql.NullInt64

			userName       sql.NullString
			apiKeyLabel    sql.NullString
			groupNameStr   sql.NullString
			accountNameStr sql.NullString
			anomalyTypes   pq.StringArray
		)

		if err := rows.Scan(
			&kind,
			&createdAt,
			&requestID,
			&platform,
			&model,
			&durationMs,
			&statusCode,
			&errorID,
			&phase,
			&severity,
			&message,
			&userID,
			&apiKeyID,
			&accountID,
			&groupID,
			&stream,
			&requestBodyBytes,
			&authLatencyMs,
			&routingLatencyMs,
			&upstreamLatencyMs,
			&responseLatencyMs,
			&userName,
			&apiKeyLabel,
			&groupNameStr,
			&accountNameStr,
			&anomalyTypes,
		); err != nil {
			return nil, 0, err
		}

		item := &service.OpsRequestDetail{
			Kind:      service.OpsRequestKind(kind),
			CreatedAt: createdAt,
			RequestID: strings.TrimSpace(requestID.String),
			Platform:  strings.TrimSpace(platform.String),
			Model:     strings.TrimSpace(model.String),

			DurationMs: toIntPtr(durationMs),
			StatusCode: toIntPtr(statusCode),
			ErrorID:    toInt64Ptr(errorID),
			Phase:      phase.String,
			Severity:   severity.String,
			Message:    message.String,

			UserID:    toInt64Ptr(userID),
			APIKeyID:  toInt64Ptr(apiKeyID),
			AccountID: toInt64Ptr(accountID),
			GroupID:   toInt64Ptr(groupID),

			Stream: stream,

			RequestBodyBytes: toIntPtr(requestBodyBytes),

			AuthLatencyMs:     toIntPtr(authLatencyMs),
			RoutingLatencyMs:  toIntPtr(routingLatencyMs),
			UpstreamLatencyMs: toIntPtr(upstreamLatencyMs),
			ResponseLatencyMs: toIntPtr(responseLatencyMs),
		}

		if item.Platform == "" {
			item.Platform = "unknown"
		}

		if userName.Valid && userName.String != "" {
			s := userName.String
			item.UserName = &s
		}
		if apiKeyLabel.Valid && apiKeyLabel.String != "" {
			s := apiKeyLabel.String
			item.APIKeyLabel = &s
		}
		if groupNameStr.Valid && groupNameStr.String != "" {
			s := groupNameStr.String
			item.GroupName = &s
		}
		if accountNameStr.Valid && accountNameStr.String != "" {
			s := accountNameStr.String
			item.AccountName = &s
		}
		if len(anomalyTypes) > 0 {
			item.AnomalyTypes = []string(anomalyTypes)
		}

		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return out, total, nil
}
