package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserHandler_GetCheckInHistory_ContractResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewUserHandler(newStubAdminService(), nil)
	router := gin.New()
	router.GET("/api/v1/admin/users/:id/check-in-history", handler.GetCheckInHistory)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/1/check-in-history?page=1&page_size=20", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
		Data struct {
			Items []struct {
				ID           int64   `json:"id"`
				CheckInDate  string  `json:"check_in_date"`
				CheckedInAt  string  `json:"checked_in_at"`
				RewardType   string  `json:"reward_type"`
				RewardAmount float64 `json:"reward_amount"`
			} `json:"items"`
			Total         int64   `json:"total"`
			Page          int     `json:"page"`
			PageSize      int     `json:"page_size"`
			Pages         int     `json:"pages"`
			TotalReward   float64 `json:"total_reward"`
			TotalCheckIns int64   `json:"total_checkins"`
			LastCheckInAt string  `json:"last_check_in_at"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))

	require.Equal(t, 0, payload.Code)
	require.Equal(t, "success", payload.Msg)
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, int64(1), payload.Data.Items[0].ID)
	require.Equal(t, "2026-04-09", payload.Data.Items[0].CheckInDate)
	require.Equal(t, "balance", payload.Data.Items[0].RewardType)
	require.InDelta(t, 1.25, payload.Data.Items[0].RewardAmount, 1e-9)
	require.NotEmpty(t, payload.Data.Items[0].CheckedInAt)
	require.Equal(t, int64(1), payload.Data.Total)
	require.Equal(t, 1, payload.Data.Page)
	require.Equal(t, 20, payload.Data.PageSize)
	require.Equal(t, 1, payload.Data.Pages)
	require.InDelta(t, 1.25, payload.Data.TotalReward, 1e-9)
	require.Equal(t, int64(1), payload.Data.TotalCheckIns)
	require.NotEmpty(t, payload.Data.LastCheckInAt)
}

func TestUserHandler_GetCheckInHistory_InvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewUserHandler(newStubAdminService(), nil)
	router := gin.New()
	router.GET("/api/v1/admin/users/:id/check-in-history", handler.GetCheckInHistory)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/not-a-number/check-in-history", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
