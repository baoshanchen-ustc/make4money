package service

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/uuid"
)

// isFirstPartyAnthropicMessagesURL 判断 targetURL 是否指向 first-party Anthropic 的
// /v1/messages 或 /v1/messages/count_tokens 端点。
//
// 仅当所有以下条件都成立时返回 true：
//   - scheme == "https"
//   - host（不含端口）== "api.anthropic.com"（精确匹配，避免 "api.anthropic.com.evil" 等 suffix 攻击）
//   - path 等于 "/v1/messages" 或 "/v1/messages/count_tokens"
//
// query 中的 ?beta=true / ?proxy=... 不影响判断；任何不解析的 URL 视为非 first-party。
//
// 该函数是 x-client-request-id 自动生成的安全边界：自定义 relay、第三方域、HTTP 协议、
// 路径不一致都不应被代理擅自填充 first-party 才有的标识。
func isFirstPartyAnthropicMessagesURL(targetURL string) bool {
	if targetURL == "" {
		return false
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return false
	}
	if u.Scheme != "https" {
		return false
	}
	host := u.Hostname() // 去掉端口；并保留原始 host 字符串校验
	if !strings.EqualFold(host, "api.anthropic.com") {
		return false
	}
	switch u.Path {
	case "/v1/messages", "/v1/messages/count_tokens":
		return true
	default:
		return false
	}
}

// shouldAutoGenerateClaudeRequestID 决定是否要为该请求自动生成 x-client-request-id。
// 必要条件全部成立才返回 true：
//   - targetURL 命中 first-party Anthropic /v1/messages 或 /v1/messages/count_tokens
//   - 请求当前缺少 x-client-request-id（兼容 canonical / wire casing）
//   - tokenType 对应的开关开启：
//     - "oauth" → cfg.AutoGenerateOAuth
//     - 其它（包括 "api_key" / "" / "bedrock"）→ cfg.AutoGenerateAPIKeyPassthrough
func shouldAutoGenerateClaudeRequestID(req *http.Request, targetURL string, cfg config.ClaudeRequestIDConfig, tokenType string) bool {
	if req == nil {
		return false
	}
	if !isFirstPartyAnthropicMessagesURL(targetURL) {
		return false
	}
	if getHeaderRaw(req.Header, "x-client-request-id") != "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(tokenType)) {
	case "oauth":
		return cfg.AutoGenerateOAuth
	default:
		return cfg.AutoGenerateAPIKeyPassthrough
	}
}

// ensureClaudeFirstPartyRequestID 在条件满足时为请求注入 UUID v4 形式的 x-client-request-id。
// 调用方应在 header 透传 / 指纹 / mimic / beta policy 处理之后调用，使判断作用于
// "最终发出"的 header 集合，避免先生成又被后续逻辑覆盖。
//
// 返回值：是否实际生成。可用于观测计数（仅记录是否生成 / 跳过原因，不记录完整 ID）。
func ensureClaudeFirstPartyRequestID(req *http.Request, targetURL string, cfg config.ClaudeRequestIDConfig, tokenType string) bool {
	if !shouldAutoGenerateClaudeRequestID(req, targetURL, cfg, tokenType) {
		return false
	}
	setHeaderRaw(req.Header, "x-client-request-id", uuid.NewString())
	return true
}
