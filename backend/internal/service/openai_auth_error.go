package service

import "strings"

func classifyOpenAIPermanentAuthError(statusCode int, responseBody []byte) string {
	if statusCode != 401 {
		return ""
	}

	switch extractUpstreamErrorCode(responseBody) {
	case "token_invalidated", "token_revoked", "account_deactivated":
		return extractUpstreamErrorCode(responseBody)
	default:
		return ""
	}
}

func buildOpenAIPermanentAuthErrorMessage(errorCode, upstreamMsg string) string {
	upstreamMsg = strings.TrimSpace(upstreamMsg)

	switch errorCode {
	case "account_deactivated":
		if upstreamMsg != "" {
			return "Account deactivated (401): " + upstreamMsg
		}
		return "Account deactivated (401): account has been deactivated"
	case "token_invalidated", "token_revoked":
		if upstreamMsg != "" {
			return "Token revoked (401): " + upstreamMsg
		}
		return "Token revoked (401): account authentication permanently revoked"
	default:
		if upstreamMsg != "" {
			return "Authentication failed (401): " + upstreamMsg
		}
		return "Authentication failed (401): invalid or expired credentials"
	}
}
