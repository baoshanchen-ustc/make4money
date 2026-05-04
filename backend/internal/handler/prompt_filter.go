package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type promptFilterErrorWriter func(status int, errorType string, message string)

func (h *GatewayHandler) enforcePromptFilter(c *gin.Context, body []byte, explicitPrompt string, writeError promptFilterErrorWriter) bool {
	if h == nil {
		return false
	}
	return enforcePromptFilter(c, h.settingService, h.apiKeyService, body, explicitPrompt, writeError)
}

func (h *OpenAIGatewayHandler) enforcePromptFilter(c *gin.Context, body []byte, explicitPrompt string, writeError promptFilterErrorWriter) bool {
	if h == nil {
		return false
	}
	return enforcePromptFilter(c, h.settingService, h.apiKeyService, body, explicitPrompt, writeError)
}

func enforcePromptFilter(c *gin.Context, settingService *service.SettingService, apiKeyService *service.APIKeyService, body []byte, explicitPrompt string, writeError promptFilterErrorWriter) bool {
	if c == nil || settingService == nil || writeError == nil {
		return false
	}

	runtime := settingService.GetPromptFilterRuntime(c.Request.Context())
	if !runtime.Enabled || len(runtime.Keywords) == 0 {
		return false
	}

	var parts []string
	if prompt := strings.TrimSpace(explicitPrompt); prompt != "" {
		parts = append(parts, prompt)
	}
	if len(body) > 0 {
		if prompt := service.PromptTextFromJSON(body); strings.TrimSpace(prompt) != "" {
			parts = append(parts, prompt)
		}
	}
	if !service.PromptFilterContainsKeyword(strings.Join(parts, "\n"), runtime.Keywords) {
		return false
	}

	userID := int64(0)
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		userID = subject.UserID
	}
	count := 0
	disabled := false
	if apiKeyService != nil && userID > 0 {
		var err error
		count, disabled, err = apiKeyService.RecordPromptViolation(c.Request.Context(), userID, runtime.ViolationLimit)
		if err != nil {
			slog.Warn("failed to record prompt filter violation", "user_id", userID, "error", err)
		}
	}

	message := runtime.WarningMessage
	if disabled {
		message = runtime.BanMessage
	}
	if strings.TrimSpace(message) == "" {
		if disabled {
			message = service.DefaultPromptFilterBanMessage
		} else {
			message = service.DefaultPromptFilterWarningMessage
		}
	}

	slog.Warn("prompt filter violation blocked", "user_id", userID, "violation_count", count, "disabled", disabled)
	writeError(http.StatusForbidden, "policy_violation", message)
	return true
}
