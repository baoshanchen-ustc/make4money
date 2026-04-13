package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestBuildOpsSystemLogsWhere_WithClientRequestIDAndUserID(t *testing.T) {
	start := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	userID := int64(12)
	accountID := int64(34)

	filter := &service.OpsSystemLogFilter{
		StartTime:       &start,
		EndTime:         &end,
		Level:           "warn",
		Component:       "http.access",
		RequestID:       "req-1",
		ClientRequestID: "creq-1",
		UserID:          &userID,
		AccountID:       &accountID,
		Platform:        "openai",
		Model:           "gpt-5",
		Query:           "timeout",
	}

	where, args, hasConstraint := buildOpsSystemLogsWhere(filter)
	if !hasConstraint {
		t.Fatalf("expected hasConstraint=true")
	}
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 11 {
		t.Fatalf("args len = %d, want 11", len(args))
	}
	if !contains(where, "COALESCE(l.client_request_id,'') = $") {
		t.Fatalf("where should include client_request_id condition: %s", where)
	}
	if !contains(where, "l.user_id = $") {
		t.Fatalf("where should include user_id condition: %s", where)
	}
}

func TestBuildOpsSystemLogsCleanupWhere_RequireConstraint(t *testing.T) {
	where, args, hasConstraint := buildOpsSystemLogsCleanupWhere(&service.OpsSystemLogCleanupFilter{})
	if hasConstraint {
		t.Fatalf("expected hasConstraint=false")
	}
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 0 {
		t.Fatalf("args len = %d, want 0", len(args))
	}
}

func TestBuildOpsSystemLogsCleanupWhere_WithClientRequestIDAndUserID(t *testing.T) {
	userID := int64(9)
	filter := &service.OpsSystemLogCleanupFilter{
		ClientRequestID: "creq-9",
		UserID:          &userID,
	}

	where, args, hasConstraint := buildOpsSystemLogsCleanupWhere(filter)
	if !hasConstraint {
		t.Fatalf("expected hasConstraint=true")
	}
	if len(args) != 2 {
		t.Fatalf("args len = %d, want 2", len(args))
	}
	if !contains(where, "COALESCE(l.client_request_id,'') = $") {
		t.Fatalf("where should include client_request_id condition: %s", where)
	}
	if !contains(where, "l.user_id = $") {
		t.Fatalf("where should include user_id condition: %s", where)
	}
}

func TestBuildOpsSystemLogsWhere_WithResolvedRequestIDAndExtraFilters(t *testing.T) {
	apiKeyID := int64(77)
	groupID := int64(88)
	filter := &service.OpsSystemLogFilter{
		ResolvedRequestID: "req-extra-1",
		ExtraAPIKeyID:     &apiKeyID,
		ExtraGroupID:      &groupID,
	}

	where, args, hasConstraint := buildOpsSystemLogsWhere(filter)
	if !hasConstraint {
		t.Fatalf("expected hasConstraint=true")
	}
	if len(args) != 3 {
		t.Fatalf("args len = %d, want 3", len(args))
	}
	if !contains(where, "COALESCE(l.request_id, l.extra->>'request_id', l.extra->'last'->>'request_id', '') = $") {
		t.Fatalf("where should match resolved request_id expression index: %s", where)
	}
	if !contains(where, "l.extra->>'api_key_id'") {
		t.Fatalf("where should include extra api_key_id lookup: %s", where)
	}
	if !contains(where, "l.extra->>'group_id'") {
		t.Fatalf("where should include extra group_id lookup: %s", where)
	}
}

func TestBuildOpsSystemLogsWhere_QueryLooksLikeCorrelationKey(t *testing.T) {
	filter := &service.OpsSystemLogFilter{
		Query: "req-usage-1234",
	}

	where, args, hasConstraint := buildOpsSystemLogsWhere(filter)
	if !hasConstraint {
		t.Fatalf("expected hasConstraint=true")
	}
	if len(args) != 2 {
		t.Fatalf("args len = %d, want 2", len(args))
	}
	if args[0] != "req-usage-1234" {
		t.Fatalf("expected exact correlation arg first, got %#v", args[0])
	}
	if args[1] != "%req-usage-1234%" {
		t.Fatalf("expected like arg second, got %#v", args[1])
	}
	if !contains(where, "COALESCE(l.request_id,'') = $") {
		t.Fatalf("where should include exact request_id branch: %s", where)
	}
	if contains(where, "COALESCE(l.request_id,'') ILIKE $") {
		t.Fatalf("where should avoid broad request_id ILIKE for correlation key search: %s", where)
	}
	if !contains(where, "l.extra->>'request_id'") {
		t.Fatalf("where should include exact extra request_id branch: %s", where)
	}
}

func TestBuildOpsSystemLogsWhere_QueryStaysFuzzyForPlainText(t *testing.T) {
	filter := &service.OpsSystemLogFilter{
		Query: "timeout",
	}

	where, args, hasConstraint := buildOpsSystemLogsWhere(filter)
	if !hasConstraint {
		t.Fatalf("expected hasConstraint=true")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if args[0] != "%timeout%" {
		t.Fatalf("expected fuzzy like arg, got %#v", args[0])
	}
	if !contains(where, "COALESCE(l.request_id,'') ILIKE $") {
		t.Fatalf("where should keep request_id fuzzy lookup for plain text search: %s", where)
	}
}

func TestLooksLikeOpsCorrelationKey(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{raw: "req-usage-1234", want: true},
		{raw: "chatcmpl-abc123", want: true},
		{raw: "fc_call_123", want: true},
		{raw: "timeout", want: false},
		{raw: "too short", want: false},
		{raw: "plainword1", want: false},
		{raw: "contains/slash", want: false},
	}

	for _, tc := range cases {
		if got := looksLikeOpsCorrelationKey(tc.raw); got != tc.want {
			t.Fatalf("looksLikeOpsCorrelationKey(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func contains(s string, sub string) bool {
	return strings.Contains(s, sub)
}
