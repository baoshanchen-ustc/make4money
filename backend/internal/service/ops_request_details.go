package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type OpsRequestKind string

const (
	OpsRequestKindSuccess OpsRequestKind = "success"
	OpsRequestKindError   OpsRequestKind = "error"
)

// OpsRequestDetail is a request-level view across success (usage_logs) and error (ops_error_logs).
// It powers "request drilldown" UIs without exposing full request bodies for successful requests.
type OpsRequestDetail struct {
	Kind      OpsRequestKind `json:"kind"`
	CreatedAt time.Time      `json:"created_at"`
	RequestID string         `json:"request_id"`

	Platform string `json:"platform,omitempty"`
	Model    string `json:"model,omitempty"`

	DurationMs *int `json:"duration_ms,omitempty"`
	StatusCode *int `json:"status_code,omitempty"`

	// When Kind == "error", ErrorID links to /admin/ops/errors/:id.
	ErrorID *int64 `json:"error_id,omitempty"`

	Phase    string `json:"phase,omitempty"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message,omitempty"`

	UserID    *int64 `json:"user_id,omitempty"`
	APIKeyID  *int64 `json:"api_key_id,omitempty"`
	AccountID *int64 `json:"account_id,omitempty"`
	GroupID   *int64 `json:"group_id,omitempty"`

	Stream bool `json:"stream"`

	RequestBodyBytes *int `json:"request_body_bytes,omitempty"`

	// Stage latencies — only populated for successful requests.
	AuthLatencyMs     *int `json:"auth_latency_ms,omitempty"`
	RoutingLatencyMs  *int `json:"routing_latency_ms,omitempty"`
	UpstreamLatencyMs *int `json:"upstream_latency_ms,omitempty"`
	ResponseLatencyMs *int `json:"response_latency_ms,omitempty"`
}

type OpsRequestDetailFilter struct {
	StartTime *time.Time
	EndTime   *time.Time

	// kind: success|error|all
	Kind string

	Platform string
	GroupID  *int64

	UserID    *int64
	APIKeyID  *int64
	AccountID *int64

	Model     string
	RequestID string
	Query     string

	MinDurationMs *int
	MaxDurationMs *int

	// Sort: created_at_desc (default) or duration_desc.
	Sort string

	Page     int
	PageSize int
}

func (f *OpsRequestDetailFilter) Normalize() (page, pageSize int, startTime, endTime time.Time) {
	page = 1
	pageSize = 50
	endTime = time.Now()
	startTime = endTime.Add(-1 * time.Hour)

	if f == nil {
		return page, pageSize, startTime, endTime
	}

	if f.Page > 0 {
		page = f.Page
	}
	if f.PageSize > 0 {
		pageSize = f.PageSize
	}
	if pageSize > 100 {
		pageSize = 100
	}

	if f.EndTime != nil {
		endTime = *f.EndTime
	}
	if f.StartTime != nil {
		startTime = *f.StartTime
	} else if f.EndTime != nil {
		startTime = endTime.Add(-1 * time.Hour)
	}

	if startTime.After(endTime) {
		startTime, endTime = endTime, startTime
	}

	return page, pageSize, startTime, endTime
}

type OpsRequestDetailList struct {
	Items    []*OpsRequestDetail `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// OpsUsageInspectDetail is one usage_logs row for inspecting successful request metadata
// (client model vs upstream model, endpoints, tokens). Full bodies are not stored here.
type OpsUsageInspectDetail struct {
	ID               int64     `json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	RequestID        *string   `json:"request_id,omitempty"`
	Model            string    `json:"model"`
	UpstreamModel    *string   `json:"upstream_model,omitempty"`
	InboundEndpoint  *string   `json:"inbound_endpoint,omitempty"`
	UpstreamEndpoint *string   `json:"upstream_endpoint,omitempty"`
	Platform         string    `json:"platform"`
	UserID           int64     `json:"user_id"`
	APIKeyID         int64     `json:"api_key_id"`
	AccountID        int64     `json:"account_id"`
	GroupID          *int64    `json:"group_id,omitempty"`
	AccountName      string    `json:"account_name"`
	GroupName        string    `json:"group_name"`
	Stream           bool      `json:"stream"`
	DurationMs       *int      `json:"duration_ms,omitempty"`
	FirstTokenMs     *int      `json:"first_token_ms,omitempty"`

	// 阶段耗时（毫秒），用于精确定位各环节瓶颈
	AuthLatencyMs     *int `json:"auth_latency_ms,omitempty"`     // 认证鉴权阶段耗时
	RoutingLatencyMs  *int `json:"routing_latency_ms,omitempty"`  // 路由选择阶段耗时
	UpstreamLatencyMs *int `json:"upstream_latency_ms,omitempty"` // 上游请求阶段耗时（发出请求→收到首字节）
	ResponseLatencyMs *int `json:"response_latency_ms,omitempty"` // 响应处理阶段耗时（流式传输或读取响应体）

	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	ServiceTier     *string `json:"service_tier,omitempty"`
	ReasoningEffort *string `json:"reasoning_effort,omitempty"`
	IPAddress       *string `json:"ip_address,omitempty"`
}

func (s *OpsService) ListRequestDetails(ctx context.Context, filter *OpsRequestDetailFilter) (*OpsRequestDetailList, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if s.opsRepo == nil {
		return &OpsRequestDetailList{
			Items:    []*OpsRequestDetail{},
			Total:    0,
			Page:     1,
			PageSize: 50,
		}, nil
	}

	page, pageSize, startTime, endTime := filter.Normalize()
	filterCopy := &OpsRequestDetailFilter{}
	if filter != nil {
		*filterCopy = *filter
	}
	filterCopy.Page = page
	filterCopy.PageSize = pageSize
	filterCopy.StartTime = &startTime
	filterCopy.EndTime = &endTime

	items, total, err := s.opsRepo.ListRequestDetails(ctx, filterCopy)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []*OpsRequestDetail{}
	}

	return &OpsRequestDetailList{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *OpsService) GetLatestUsageInspectByRequestID(
	ctx context.Context,
	requestID string,
	startTime, endTime time.Time,
) (*OpsUsageInspectDetail, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if s.opsRepo == nil {
		return nil, infraerrors.ServiceUnavailable("OPS_REPO_UNAVAILABLE", "Ops repository not available")
	}
	rid := strings.TrimSpace(requestID)
	if rid == "" {
		return nil, infraerrors.BadRequest("OPS_USAGE_INSPECT_INVALID", "request_id is required")
	}
	if !startTime.IsZero() && !endTime.IsZero() && startTime.After(endTime) {
		return nil, infraerrors.BadRequest("OPS_USAGE_INSPECT_INVALID", "invalid time range")
	}
	detail, err := s.opsRepo.GetLatestUsageInspectByRequestID(ctx, rid, startTime, endTime)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infraerrors.NotFound(
				"OPS_USAGE_INSPECT_NOT_FOUND",
				"no usage log matched this request_id in the time window",
			)
		}
		return nil, infraerrors.InternalServer("OPS_USAGE_INSPECT_FAILED", "failed to load usage inspect").WithCause(err)
	}
	return detail, nil
}
