//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// OpsRequestDetailsIntegrationSuite tests ListRequestDetails column-order and
// field-mapping correctness for both the success (usage_logs) and error
// (ops_error_logs) CTE branches.
//
// Each test is isolated via a per-test ent transaction that rolls back on
// cleanup, so no data leaks between tests.
type OpsRequestDetailsIntegrationSuite struct {
	suite.Suite
	ctx     context.Context
	opsRepo *opsRepository
	logRepo *usageLogRepository
}

func (s *OpsRequestDetailsIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
	// opsRepository is backed by the shared integrationDB (not a tx), because
	// the CTE query runs outside the ent transaction boundary.  We use unique
	// request IDs and time windows per test to avoid cross-test interference.
	s.opsRepo = &opsRepository{db: integrationDB}
	s.logRepo = newUsageLogRepositoryWithSQL(integrationEntClient, integrationDB)
}

func TestOpsRequestDetailsIntegrationSuite(t *testing.T) {
	suite.Run(t, new(OpsRequestDetailsIntegrationSuite))
}

// ---- helpers ----------------------------------------------------------------

// ptr returns a pointer to the given value.
func ptr[T any](v T) *T { return &v }

// uniqueEmail generates a unique email for each call site to avoid UNIQUE
// constraint collisions when tests run in the same DB instance.
func uniqueEmail(tag string) string {
	return fmt.Sprintf("ops-rd-%s-%s@example.com", tag, uuid.NewString()[:8])
}

// insertUsageLog inserts a single usage_log row directly through the
// usageLogRepository and returns the created log (with ID and CreatedAt set).
func (s *OpsRequestDetailsIntegrationSuite) insertUsageLog(log *service.UsageLog) *service.UsageLog {
	s.T().Helper()
	if log.RequestID == "" {
		log.RequestID = uuid.NewString()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}
	_, err := s.logRepo.Create(s.ctx, log)
	require.NoError(s.T(), err, "insert usage_log")
	return log
}

// insertErrorLog inserts a single ops_error_log row directly and returns the
// assigned ID.
func (s *OpsRequestDetailsIntegrationSuite) insertErrorLog(input *service.OpsInsertErrorLogInput) int64 {
	s.T().Helper()
	id, err := s.opsRepo.InsertErrorLog(s.ctx, input)
	require.NoError(s.T(), err, "insert ops_error_log")
	return id
}

// listInWindow calls ListRequestDetails with a time window that is guaranteed
// to contain the given timestamp.
func (s *OpsRequestDetailsIntegrationSuite) listInWindow(at time.Time) ([]*service.OpsRequestDetail, int64) {
	s.T().Helper()
	start := at.Add(-time.Second)
	end := at.Add(time.Hour)
	rows, total, err := s.opsRepo.ListRequestDetails(s.ctx, &service.OpsRequestDetailFilter{
		StartTime: &start,
		EndTime:   &end,
	})
	require.NoError(s.T(), err, "ListRequestDetails")
	return rows, total
}

// ---- tests ------------------------------------------------------------------

// TestSuccessBranch_UpstreamModelAndAPIKeyName verifies that when a usage_log
// row has:
//   - model != upstream_model  (model mapping was applied)
//   - an associated api_key with a non-empty name
//
// then ListRequestDetails correctly populates:
//   - OpsRequestDetail.UpstreamModel  (not nil, correct value)
//   - OpsRequestDetail.APIKeyName      (not nil, equals api_key.name)
//   - OpsRequestDetail.Spans           (correctly deserialized, not corrupted)
//
// This is a regression guard for the Scan column-order bug that previously
// caused spansJSON to receive the upstream_model string, causing Spans to be
// silently dropped.
func (s *OpsRequestDetailsIntegrationSuite) TestSuccessBranch_UpstreamModelAndAPIKeyName() {
	client := testEntClient(s.T())

	user := mustCreateUser(s.T(), client, &service.User{Email: uniqueEmail("succ-user")})
	apiKey := mustCreateApiKey(s.T(), client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-succ-" + uuid.NewString()[:8],
		Name:   "my-mapped-key",
	})
	account := mustCreateAccount(s.T(), client, &service.Account{Name: "acc-succ-" + uuid.NewString()[:8]})

	// Spans: a minimal span payload to verify correct deserialization.
	spans := []*service.OpsSpan{
		{Name: "upstream", StartUnixMs: 1000000, DurationMs: 250},
	}
	spansJSON, err := json.Marshal(spans)
	require.NoError(s.T(), err)
	spansJSONStr := string(spansJSON)

	now := time.Now().UTC()
	log := s.insertUsageLog(&service.UsageLog{
		UserID:        user.ID,
		APIKeyID:      apiKey.ID,
		AccountID:     account.ID,
		RequestID:     uuid.NewString(),
		Model:         "claude-sonnet-4-6",
		UpstreamModel: ptr("gpt-5.4"),
		InputTokens:   100,
		OutputTokens:  50,
		CreatedAt:     now,
		Spans:         &spansJSONStr,
	})

	rows, total := s.listInWindow(log.CreatedAt)
	require.GreaterOrEqual(s.T(), total, int64(1), "expected at least one row")

	// Find the row matching our request_id.
	var found *service.OpsRequestDetail
	for _, r := range rows {
		if r.RequestID == log.RequestID {
			found = r
			break
		}
	}
	require.NotNil(s.T(), found, "row not found for request_id=%s", log.RequestID)
	require.Equal(s.T(), service.OpsRequestKindSuccess, found.Kind)

	// --- UpstreamModel ---
	require.NotNil(s.T(), found.UpstreamModel, "UpstreamModel should be set when mapping was applied")
	require.Equal(s.T(), "gpt-5.4", *found.UpstreamModel, "UpstreamModel value mismatch")

	// --- APIKeyName ---
	require.NotNil(s.T(), found.APIKeyName, "APIKeyName should be set from api_keys.name")
	require.Equal(s.T(), "my-mapped-key", *found.APIKeyName, "APIKeyName value mismatch")

	// --- Spans: must not be nil and must deserialize correctly ---
	require.NotNil(s.T(), found.Spans, "Spans should be deserialized (not nil)")
	require.Len(s.T(), found.Spans, 1, "expected 1 span")
	require.Equal(s.T(), "upstream", found.Spans[0].Name, "span Name mismatch — possible column order bug")
	require.Equal(s.T(), int64(250), found.Spans[0].DurationMs, "span DurationMs mismatch")
}

// TestSuccessBranch_NoMapping_UpstreamModelNil verifies that when no model
// mapping was applied (usage_logs.upstream_model IS NULL), UpstreamModel is
// nil in the result struct.
func (s *OpsRequestDetailsIntegrationSuite) TestSuccessBranch_NoMapping_UpstreamModelNil() {
	client := testEntClient(s.T())

	user := mustCreateUser(s.T(), client, &service.User{Email: uniqueEmail("succ-nomapping")})
	apiKey := mustCreateApiKey(s.T(), client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-nm-" + uuid.NewString()[:8],
		Name:   "no-mapping-key",
	})
	account := mustCreateAccount(s.T(), client, &service.Account{Name: "acc-nm-" + uuid.NewString()[:8]})

	now := time.Now().UTC()
	log := s.insertUsageLog(&service.UsageLog{
		UserID:        user.ID,
		APIKeyID:      apiKey.ID,
		AccountID:     account.ID,
		RequestID:     uuid.NewString(),
		Model:         "claude-opus-4",
		UpstreamModel: nil, // no mapping
		InputTokens:   10,
		OutputTokens:  5,
		CreatedAt:     now,
	})

	rows, _ := s.listInWindow(log.CreatedAt)

	var found *service.OpsRequestDetail
	for _, r := range rows {
		if r.RequestID == log.RequestID {
			found = r
			break
		}
	}
	require.NotNil(s.T(), found, "row not found for request_id=%s", log.RequestID)
	require.Nil(s.T(), found.UpstreamModel, "UpstreamModel should be nil when no mapping was applied")
	require.NotNil(s.T(), found.APIKeyName, "APIKeyName should still be populated")
	require.Equal(s.T(), "no-mapping-key", *found.APIKeyName)
}

// TestErrorBranch_APIKeyName verifies that the error branch (ops_error_logs)
// correctly populates APIKeyName from api_keys.name even when all optional
// latency fields are NULL.
//
// This is a regression guard for the MEDIUM finding: the error branch
// previously hardcoded NULL::TEXT for api_key_name even though the api_keys
// table was already JOINed.
func (s *OpsRequestDetailsIntegrationSuite) TestErrorBranch_APIKeyName() {
	client := testEntClient(s.T())

	user := mustCreateUser(s.T(), client, &service.User{Email: uniqueEmail("err-user")})
	apiKey := mustCreateApiKey(s.T(), client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-err-" + uuid.NewString()[:8],
		Name:   "error-key-name",
	})

	now := time.Now().UTC()
	reqID := uuid.NewString()

	s.insertErrorLog(&service.OpsInsertErrorLogInput{
		RequestID:    reqID,
		UserID:       &user.ID,
		APIKeyID:     &apiKey.ID,
		Platform:     "openai",
		Model:        "gpt-5",
		ErrorPhase:   "upstream",
		ErrorType:    "upstream_error",
		Severity:     "error",
		StatusCode:   502,
		ErrorMessage: "bad gateway",
		CreatedAt:    now,
	})

	rows, _ := s.listInWindow(now)

	var found *service.OpsRequestDetail
	for _, r := range rows {
		if r.RequestID == reqID {
			found = r
			break
		}
	}
	require.NotNil(s.T(), found, "error row not found for request_id=%s", reqID)
	require.Equal(s.T(), service.OpsRequestKindError, found.Kind)

	// APIKeyName must be populated from ak.name — not NULL.
	require.NotNil(s.T(), found.APIKeyName,
		"APIKeyName should be non-nil for error rows (regression: was NULL::TEXT before fix)")
	require.Equal(s.T(), "error-key-name", *found.APIKeyName,
		"APIKeyName value mismatch in error branch")
}

// TestErrorBranch_UpstreamModelAlwaysNil verifies that the error branch does
// NOT populate UpstreamModel (it is always NULL in ops_error_logs), so
// UpstreamModel remains nil.
func (s *OpsRequestDetailsIntegrationSuite) TestErrorBranch_UpstreamModelAlwaysNil() {
	client := testEntClient(s.T())

	user := mustCreateUser(s.T(), client, &service.User{Email: uniqueEmail("err-noup")})
	apiKey := mustCreateApiKey(s.T(), client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-err-noup-" + uuid.NewString()[:8],
		Name:   "err-noup-key",
	})

	now := time.Now().UTC()
	reqID := uuid.NewString()

	s.insertErrorLog(&service.OpsInsertErrorLogInput{
		RequestID:    reqID,
		UserID:       &user.ID,
		APIKeyID:     &apiKey.ID,
		Platform:     "anthropic",
		Model:        "claude-3-haiku",
		ErrorPhase:   "upstream",
		ErrorType:    "upstream_error",
		Severity:     "error",
		StatusCode:   500,
		ErrorMessage: "internal server error",
		CreatedAt:    now,
	})

	rows, _ := s.listInWindow(now)

	var found *service.OpsRequestDetail
	for _, r := range rows {
		if r.RequestID == reqID {
			found = r
			break
		}
	}
	require.NotNil(s.T(), found, "error row not found for request_id=%s", reqID)
	require.Equal(s.T(), service.OpsRequestKindError, found.Kind)
	require.Nil(s.T(), found.UpstreamModel, "UpstreamModel should be nil for error branch (no upstream_model column)")
}

// TestSuccessBranch_SpansNilWhenAbsent verifies that when usage_logs.spans IS
// NULL, the result row has Spans == nil (not an empty slice or garbage).
func (s *OpsRequestDetailsIntegrationSuite) TestSuccessBranch_SpansNilWhenAbsent() {
	client := testEntClient(s.T())

	user := mustCreateUser(s.T(), client, &service.User{Email: uniqueEmail("succ-nospans")})
	apiKey := mustCreateApiKey(s.T(), client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-nospans-" + uuid.NewString()[:8],
		Name:   "nospans-key",
	})
	account := mustCreateAccount(s.T(), client, &service.Account{Name: "acc-nospans-" + uuid.NewString()[:8]})

	now := time.Now().UTC()
	log := s.insertUsageLog(&service.UsageLog{
		UserID:       user.ID,
		APIKeyID:     apiKey.ID,
		AccountID:    account.ID,
		RequestID:    uuid.NewString(),
		Model:        "claude-3-haiku",
		InputTokens:  5,
		OutputTokens: 3,
		CreatedAt:    now,
		// SpansJSON intentionally nil
	})

	rows, _ := s.listInWindow(log.CreatedAt)

	var found *service.OpsRequestDetail
	for _, r := range rows {
		if r.RequestID == log.RequestID {
			found = r
			break
		}
	}
	require.NotNil(s.T(), found, "row not found for request_id=%s", log.RequestID)
	require.Nil(s.T(), found.Spans, "Spans should be nil when usage_logs.spans is NULL")
}
