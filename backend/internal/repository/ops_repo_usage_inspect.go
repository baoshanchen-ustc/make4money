package repository

import (
	"context"
	"database/sql"
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
  ul.input_tokens,
  ul.output_tokens,
  ul.service_tier,
  ul.reasoning_effort,
  CASE WHEN ul.ip_address IS NULL THEN NULL ELSE ul.ip_address::text END AS ip_address
FROM usage_logs ul
LEFT JOIN groups g ON g.id = ul.group_id
LEFT JOIN accounts a ON a.id = ul.account_id
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
	var serviceTier sql.NullString
	var reasoningEffort sql.NullString
	var ipAddr sql.NullString

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
		&out.InputTokens,
		&out.OutputTokens,
		&serviceTier,
		&reasoningEffort,
		&ipAddr,
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

	return &out, nil
}
