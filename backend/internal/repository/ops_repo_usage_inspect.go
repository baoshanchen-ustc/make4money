package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *opsRepository) GetLatestUsageInspectByRequestID(
	ctx context.Context,
	requestID string,
	startTime, endTime time.Time,
) (*service.OpsUsageInspectDetail, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if requestID == "" {
		return nil, sql.ErrNoRows
	}

	const q = `
SELECT
  ul.id,
  ul.created_at,
  ul.request_id,
  ul.model,
  ul.upstream_model,
  ul.inbound_endpoint,
  ul.upstream_endpoint,
  COALESCE(NULLIF(g.platform, ''), NULLIF(a.platform, ''), '') AS platform,
  ul.user_id,
  ul.api_key_id,
  ul.account_id,
  ul.group_id,
  COALESCE(a.name, '') AS account_name,
  COALESCE(g.name, '') AS group_name,
  ul.stream,
  ul.duration_ms,
  ul.first_token_ms,
  ul.auth_latency_ms,
  ul.routing_latency_ms,
  ul.upstream_latency_ms,
  ul.response_latency_ms,
  ul.input_tokens,
  ul.output_tokens,
  ul.service_tier,
  ul.reasoning_effort,
  CASE WHEN ul.ip_address IS NULL THEN NULL ELSE ul.ip_address::text END AS ip_address,
  u.username AS user_name,
  CASE WHEN ak.key IS NOT NULL THEN '***' || RIGHT(ak.key, 4) ELSE NULL END AS api_key_label
FROM usage_logs ul
LEFT JOIN groups g ON g.id = ul.group_id
LEFT JOIN accounts a ON a.id = ul.account_id
LEFT JOIN users u ON u.id = ul.user_id
LEFT JOIN api_keys ak ON ak.id = ul.api_key_id
WHERE ul.request_id = $1
  AND ul.created_at >= $2 AND ul.created_at < $3
ORDER BY ul.created_at DESC
LIMIT 1`

	var out service.OpsUsageInspectDetail
	var reqID sql.NullString
	var upstreamModel sql.NullString
	var inboundEp sql.NullString
	var upstreamEp sql.NullString
	var groupID sql.NullInt64
	var durationMs sql.NullInt64
	var firstTokenMs sql.NullInt64
	var authLatencyMs sql.NullInt64
	var routingLatencyMs sql.NullInt64
	var upstreamLatencyMs sql.NullInt64
	var responseLatencyMs sql.NullInt64
	var serviceTier sql.NullString
	var reasoningEffort sql.NullString
	var ipAddr sql.NullString
	var userName sql.NullString
	var apiKeyLabel sql.NullString

	err := r.db.QueryRowContext(
		ctx,
		q,
		requestID,
		startTime.UTC(),
		endTime.UTC(),
	).Scan(
		&out.ID,
		&out.CreatedAt,
		&reqID,
		&out.Model,
		&upstreamModel,
		&inboundEp,
		&upstreamEp,
		&out.Platform,
		&out.UserID,
		&out.APIKeyID,
		&out.AccountID,
		&groupID,
		&out.AccountName,
		&out.GroupName,
		&out.Stream,
		&durationMs,
		&firstTokenMs,
		&authLatencyMs,
		&routingLatencyMs,
		&upstreamLatencyMs,
		&responseLatencyMs,
		&out.InputTokens,
		&out.OutputTokens,
		&serviceTier,
		&reasoningEffort,
		&ipAddr,
		&userName,
		&apiKeyLabel,
	)
	if err != nil {
		return nil, err
	}

	if reqID.Valid {
		s := reqID.String
		out.RequestID = &s
	}
	if upstreamModel.Valid {
		s := upstreamModel.String
		out.UpstreamModel = &s
	}
	if inboundEp.Valid {
		s := inboundEp.String
		out.InboundEndpoint = &s
	}
	if upstreamEp.Valid {
		s := upstreamEp.String
		out.UpstreamEndpoint = &s
	}
	if groupID.Valid {
		gid := groupID.Int64
		out.GroupID = &gid
	}
	if durationMs.Valid {
		d := int(durationMs.Int64)
		out.DurationMs = &d
	}
	if firstTokenMs.Valid {
		f := int(firstTokenMs.Int64)
		out.FirstTokenMs = &f
	}
	if authLatencyMs.Valid {
		v := int(authLatencyMs.Int64)
		out.AuthLatencyMs = &v
	}
	if routingLatencyMs.Valid {
		v := int(routingLatencyMs.Int64)
		out.RoutingLatencyMs = &v
	}
	if upstreamLatencyMs.Valid {
		v := int(upstreamLatencyMs.Int64)
		out.UpstreamLatencyMs = &v
	}
	if responseLatencyMs.Valid {
		v := int(responseLatencyMs.Int64)
		out.ResponseLatencyMs = &v
	}
	if serviceTier.Valid {
		s := serviceTier.String
		out.ServiceTier = &s
	}
	if reasoningEffort.Valid {
		s := reasoningEffort.String
		out.ReasoningEffort = &s
	}
	if ipAddr.Valid {
		s := ipAddr.String
		out.IPAddress = &s
	}
	if userName.Valid && userName.String != "" {
		s := userName.String
		out.UserName = &s
	}
	if apiKeyLabel.Valid && apiKeyLabel.String != "" {
		s := apiKeyLabel.String
		out.APIKeyLabel = &s
	}

	// Fetch raw anomaly data from request_logs (may not exist if save_raw_data was off).
	if r.requestLogRepo != nil && out.RequestID != nil {
		logData, err := r.requestLogRepo.GetByRequestID(ctx, *out.RequestID)
		if err == nil && logData != nil {
			out.AnomalyTypes = logData.AnomalyTypes
			if logData.RequestBody != nil {
				out.RequestBody = json.RawMessage(logData.RequestBody)
			}
			if logData.UpstreamRequestBody != nil {
				out.UpstreamRequestBody = json.RawMessage(logData.UpstreamRequestBody)
			}
			if logData.UpstreamResponseBody != nil {
				out.UpstreamResponseBody = json.RawMessage(logData.UpstreamResponseBody)
			}
		}
	}

	return &out, nil
}
