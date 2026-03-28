//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────
// extractCopilotPlan
// ─────────────────────────────────────────────

func TestExtractCopilotPlan_NilExtra(t *testing.T) {
	acc := &Account{Extra: nil}
	planType, seatCount := extractCopilotPlan(acc)
	require.Equal(t, "individual_pro", planType)
	require.Equal(t, 1, seatCount)
}

func TestExtractCopilotPlan_EmptyExtra(t *testing.T) {
	acc := &Account{Extra: map[string]interface{}{}}
	planType, seatCount := extractCopilotPlan(acc)
	require.Equal(t, "individual_pro", planType)
	require.Equal(t, 1, seatCount)
}

func TestExtractCopilotPlan_ValidPlanAndSeats(t *testing.T) {
	acc := &Account{Extra: map[string]interface{}{
		"copilot_plan_type":  "enterprise",
		"copilot_seat_count": float64(50),
	}}
	planType, seatCount := extractCopilotPlan(acc)
	require.Equal(t, "enterprise", planType)
	require.Equal(t, 50, seatCount)
}

func TestExtractCopilotPlan_IntSeatCount(t *testing.T) {
	acc := &Account{Extra: map[string]interface{}{
		"copilot_plan_type":  "business",
		"copilot_seat_count": int(10),
	}}
	planType, seatCount := extractCopilotPlan(acc)
	require.Equal(t, "business", planType)
	require.Equal(t, 10, seatCount)
}

func TestExtractCopilotPlan_Int64SeatCount(t *testing.T) {
	acc := &Account{Extra: map[string]interface{}{
		"copilot_plan_type":  "individual_pro_plus",
		"copilot_seat_count": int64(3),
	}}
	planType, seatCount := extractCopilotPlan(acc)
	require.Equal(t, "individual_pro_plus", planType)
	require.Equal(t, 3, seatCount)
}

func TestExtractCopilotPlan_ZeroSeatFallsBackToOne(t *testing.T) {
	acc := &Account{Extra: map[string]interface{}{
		"copilot_plan_type":  "business",
		"copilot_seat_count": float64(0),
	}}
	_, seatCount := extractCopilotPlan(acc)
	require.Equal(t, 1, seatCount)
}

func TestExtractCopilotPlan_EmptyPlanStringFallsToDefault(t *testing.T) {
	acc := &Account{Extra: map[string]interface{}{
		"copilot_plan_type": "",
	}}
	planType, _ := extractCopilotPlan(acc)
	require.Equal(t, "individual_pro", planType)
}

// ─────────────────────────────────────────────
// buildRequestHierarchy
// ─────────────────────────────────────────────

func makeRequest(requestID string, initiator string, createdAt time.Time) CopilotRequestItem {
	return CopilotRequestItem{
		RequestID: requestID,
		Model:     "gpt-4",
		Initiator: initiator,
		CreatedAt: createdAt,
	}
}

func TestBuildRequestHierarchy_Empty(t *testing.T) {
	result := buildRequestHierarchy(nil)
	require.Empty(t, result)
}

func TestBuildRequestHierarchy_OnlyUserRequests(t *testing.T) {
	now := time.Now()
	rows := []CopilotRequestItem{
		makeRequest("req1", "user", now),
		makeRequest("req2", "user", now.Add(5*time.Second)),
	}
	result := buildRequestHierarchy(rows)
	require.Len(t, result, 2)
	require.Equal(t, "req1", result[0].RequestID)
	require.Equal(t, "req2", result[1].RequestID)
	require.Nil(t, result[0].SubRequests)
	require.Nil(t, result[1].SubRequests)
}

func TestBuildRequestHierarchy_AgentAttachedWithinWindow(t *testing.T) {
	now := time.Now()
	rows := []CopilotRequestItem{
		makeRequest("user1", "user", now),
		makeRequest("agent1", "agent", now.Add(5*time.Second)),  // within 30s → attached
		makeRequest("agent2", "agent", now.Add(20*time.Second)), // within 30s → attached
	}
	result := buildRequestHierarchy(rows)
	require.Len(t, result, 1, "only the parent user request should be at top level")
	require.Equal(t, "user1", result[0].RequestID)
	require.Len(t, result[0].SubRequests, 2)
	require.Equal(t, "agent1", result[0].SubRequests[0].RequestID)
	require.Equal(t, "agent2", result[0].SubRequests[1].RequestID)
}

func TestBuildRequestHierarchy_AgentOutsideWindowPromotedToTopLevel(t *testing.T) {
	now := time.Now()
	rows := []CopilotRequestItem{
		makeRequest("user1", "user", now),
		makeRequest("agent1", "agent", now.Add(31*time.Second)), // outside 30s → orphan
	}
	result := buildRequestHierarchy(rows)
	require.Len(t, result, 2, "orphan agent should be promoted to top level")
	require.Equal(t, "user1", result[0].RequestID)
	require.Equal(t, "agent1", result[1].RequestID)
	require.Nil(t, result[0].SubRequests)
}

func TestBuildRequestHierarchy_AgentBeforeAnyUserIsOrphan(t *testing.T) {
	now := time.Now()
	rows := []CopilotRequestItem{
		makeRequest("agent_orphan", "agent", now),
		makeRequest("user1", "user", now.Add(5*time.Second)),
	}
	result := buildRequestHierarchy(rows)
	require.Len(t, result, 2, "orphan agent + user should both be top level")
	require.Equal(t, "agent_orphan", result[0].RequestID)
	require.Equal(t, "user1", result[1].RequestID)
}

func TestBuildRequestHierarchy_ExactlyAtWindowBoundary(t *testing.T) {
	now := time.Now()
	rows := []CopilotRequestItem{
		makeRequest("user1", "user", now),
		makeRequest("agent1", "agent", now.Add(30*time.Second)), // exactly 30s → attached
	}
	result := buildRequestHierarchy(rows)
	require.Len(t, result, 1)
	require.Len(t, result[0].SubRequests, 1)
}

func TestBuildRequestHierarchy_MultipleUserRequestsWithAgents(t *testing.T) {
	now := time.Now()
	rows := []CopilotRequestItem{
		makeRequest("user1", "user", now),
		makeRequest("agent_for_1", "agent", now.Add(2*time.Second)),
		makeRequest("user2", "user", now.Add(60*time.Second)),
		makeRequest("agent_for_2a", "agent", now.Add(65*time.Second)),
		makeRequest("agent_for_2b", "agent", now.Add(70*time.Second)),
	}
	result := buildRequestHierarchy(rows)
	require.Len(t, result, 2)
	require.Equal(t, "user1", result[0].RequestID)
	require.Len(t, result[0].SubRequests, 1)
	require.Equal(t, "agent_for_1", result[0].SubRequests[0].RequestID)
	require.Equal(t, "user2", result[1].RequestID)
	require.Len(t, result[1].SubRequests, 2)
}

// ─────────────────────────────────────────────
// AlertStatus
// ─────────────────────────────────────────────

func TestAlertStatus_OK(t *testing.T) {
	require.Equal(t, CopilotAlertStatusOK, AlertStatus(50.0, 80))
	require.Equal(t, CopilotAlertStatusOK, AlertStatus(0.0, 80))
}

func TestAlertStatus_Warning(t *testing.T) {
	require.Equal(t, CopilotAlertStatusWarning, AlertStatus(80.0, 80))
	require.Equal(t, CopilotAlertStatusWarning, AlertStatus(90.0, 80))
}

func TestAlertStatus_Critical(t *testing.T) {
	require.Equal(t, CopilotAlertStatusCritical, AlertStatus(95.0, 80))
	require.Equal(t, CopilotAlertStatusCritical, AlertStatus(100.0, 80))
}

func TestAlertStatus_CriticalOverridesThreshold(t *testing.T) {
	// Even if threshold is 99, usage at 95% still triggers critical
	require.Equal(t, CopilotAlertStatusCritical, AlertStatus(96.0, 99))
}

// ─────────────────────────────────────────────
// decodePostgresTextArray
// ─────────────────────────────────────────────

func TestDecodePostgresTextArray_Empty(t *testing.T) {
	var dest []string
	err := decodePostgresTextArray("{}", &dest)
	require.NoError(t, err)
	require.Nil(t, dest)
}

func TestDecodePostgresTextArray_EmptyString(t *testing.T) {
	var dest []string
	err := decodePostgresTextArray("", &dest)
	require.NoError(t, err)
	require.Nil(t, dest)
}

func TestDecodePostgresTextArray_SingleElement(t *testing.T) {
	var dest []string
	err := decodePostgresTextArray("{claude-sonnet}", &dest)
	require.NoError(t, err)
	require.Equal(t, []string{"claude-sonnet"}, dest)
}

func TestDecodePostgresTextArray_MultipleElements(t *testing.T) {
	var dest []string
	err := decodePostgresTextArray(`{"gpt-4","claude-opus","gemini-pro"}`, &dest)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-4", "claude-opus", "gemini-pro"}, dest)
}

func TestDecodePostgresTextArray_UnquotedElements(t *testing.T) {
	var dest []string
	err := decodePostgresTextArray("{a,b,c}", &dest)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "c"}, dest)
}

func TestDecodePostgresTextArray_InvalidFormat(t *testing.T) {
	var dest []string
	err := decodePostgresTextArray("not-an-array", &dest)
	require.Error(t, err)
}

// ─────────────────────────────────────────────
// CopilotPlanConfigs — cost calculation
// ─────────────────────────────────────────────

func TestCopilotPlanConfigs_IndividualPro(t *testing.T) {
	cfg := CopilotPlanConfigs["individual_pro"]
	require.Equal(t, 10.0, cfg.MonthlyCostPerSeat)
	require.Equal(t, 300, cfg.PremiumQuotaPerSeat)
}

func TestCopilotPlanConfigs_Business(t *testing.T) {
	cfg := CopilotPlanConfigs["business"]
	require.Equal(t, 19.0, cfg.MonthlyCostPerSeat)
	require.Equal(t, 300, cfg.PremiumQuotaPerSeat)
}

func TestCopilotPlanConfigs_Enterprise(t *testing.T) {
	cfg := CopilotPlanConfigs["enterprise"]
	require.Equal(t, 39.0, cfg.MonthlyCostPerSeat)
	require.Equal(t, 1000, cfg.PremiumQuotaPerSeat)
}

func TestCopilotPlanConfigs_MonthlyCostMultiSeat(t *testing.T) {
	cfg := CopilotPlanConfigs["enterprise"]
	seats := 50
	totalCost := cfg.MonthlyCostPerSeat * float64(seats)
	require.Equal(t, 1950.0, totalCost)
}
