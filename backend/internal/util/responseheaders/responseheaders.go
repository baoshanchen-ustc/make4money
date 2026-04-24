package responseheaders

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// defaultAllowed 定义允许透传的响应头白名单
// 注意：以下头部由 Go HTTP 包自动处理，不应手动设置：
//   - content-length: 由 ResponseWriter 根据实际写入数据自动设置
//   - transfer-encoding: 由 HTTP 库根据需要自动添加/移除
//   - connection: 由 HTTP 库管理连接复用
var defaultAllowed = map[string]struct{}{
	"content-type":                   {},
	"content-encoding":               {},
	"content-language":               {},
	"cache-control":                  {},
	"etag":                           {},
	"last-modified":                  {},
	"expires":                        {},
	"vary":                           {},
	"date":                           {},
	"x-request-id":                   {},
	"x-ratelimit-limit-requests":     {},
	"x-ratelimit-limit-tokens":       {},
	"x-ratelimit-remaining-requests": {},
	"x-ratelimit-remaining-tokens":   {},
	"x-ratelimit-reset-requests":     {},
	"x-ratelimit-reset-tokens":       {},
	"retry-after":                    {},
	"location":                       {},
	"www-authenticate":               {},
}

// hopByHopHeaders 是跳过的 hop-by-hop 头部，这些头部由 HTTP 库自动处理
var hopByHopHeaders = map[string]struct{}{
	"content-length":    {},
	"transfer-encoding": {},
	"connection":        {},
}

type CompiledHeaderFilter struct {
	allowed     map[string]struct{}
	forceRemove map[string]struct{}
}

var defaultCompiledHeaderFilter = CompileHeaderFilter(config.ResponseHeaderConfig{})

func CompileHeaderFilter(cfg config.ResponseHeaderConfig) *CompiledHeaderFilter {
	allowed := make(map[string]struct{}, len(defaultAllowed)+len(cfg.AdditionalAllowed))
	for key := range defaultAllowed {
		allowed[key] = struct{}{}
	}
	// 关闭时只使用默认白名单，additional/force_remove 不生效
	if cfg.Enabled {
		for _, key := range cfg.AdditionalAllowed {
			normalized := strings.ToLower(strings.TrimSpace(key))
			if normalized == "" {
				continue
			}
			allowed[normalized] = struct{}{}
		}
	}

	forceRemove := map[string]struct{}{}
	if cfg.Enabled {
		forceRemove = make(map[string]struct{}, len(cfg.ForceRemove))
		for _, key := range cfg.ForceRemove {
			normalized := strings.ToLower(strings.TrimSpace(key))
			if normalized == "" {
				continue
			}
			forceRemove[normalized] = struct{}{}
		}
	}

	return &CompiledHeaderFilter{
		allowed:     allowed,
		forceRemove: forceRemove,
	}
}

func FilterHeaders(src http.Header, filter *CompiledHeaderFilter) http.Header {
	if filter == nil {
		filter = defaultCompiledHeaderFilter
	}

	filtered := make(http.Header, len(src))
	for key, values := range src {
		lower := strings.ToLower(key)
		if _, blocked := filter.forceRemove[lower]; blocked {
			continue
		}
		if _, ok := filter.allowed[lower]; !ok {
			continue
		}
		// 跳过 hop-by-hop 头部，这些由 HTTP 库自动处理
		if _, isHopByHop := hopByHopHeaders[lower]; isHopByHop {
			continue
		}
		for _, value := range values {
			filtered.Add(key, value)
		}
	}
	return filtered
}

func WriteFilteredHeaders(dst http.Header, src http.Header, filter *CompiledHeaderFilter) {
	filtered := FilterHeaders(src, filter)
	for key, values := range filtered {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// 头部前缀/全名集合：在 NormalizeForAnthropicPersona 中识别 OpenAI 风格 ratelimit / 厂商泄露头
var (
	openAIRateLimitPrefix = "x-ratelimit-"
	openAIRequestIDKey    = "x-request-id"
	anthropicRequestIDKey = "request-id"

	// 任意被识别为厂商身份痕迹的响应头都会被剥离。
	vendorIdentityHeaderPrefixes = []string{
		"openai-",
		"x-openai-",
		"x-goog-",
		"x-google-",
		"x-amz-",
		"x-amzn-",
		"x-azure-",
		"x-bedrock-",
		"x-gemini-",
		"x-vertex-",
	}
	vendorIdentityHeaderExact = map[string]struct{}{
		"server":            {},
		"via":               {},
		"x-served-by":       {},
		"x-cache":           {},
		"openai-version":    {},
		"openai-organization": {},
		"openai-processing-ms": {},
	}

	// OpenAI x-ratelimit-* → Anthropic anthropic-ratelimit-* 名称映射。
	// 注意值的语义不一定 1:1（e.g. OpenAI tokens vs Anthropic tokens），但 Claude Code CLI 仅关心是否
	// 存在合理键值；保留原值优于完全缺失。
	openAIToAnthropicRateLimit = map[string]string{
		"x-ratelimit-limit-requests":     "anthropic-ratelimit-requests-limit",
		"x-ratelimit-limit-tokens":       "anthropic-ratelimit-tokens-limit",
		"x-ratelimit-remaining-requests": "anthropic-ratelimit-requests-remaining",
		"x-ratelimit-remaining-tokens":   "anthropic-ratelimit-tokens-remaining",
		"x-ratelimit-reset-requests":     "anthropic-ratelimit-requests-reset",
		"x-ratelimit-reset-tokens":       "anthropic-ratelimit-tokens-reset",
	}
)

// NormalizeForAnthropicPersona 把已写入 dst 的响应头规范化为 Anthropic 风格：
//   - 剥离 server / via / openai-* / x-goog-* / x-amz-* 等厂商泄露头
//   - x-ratelimit-* → anthropic-ratelimit-* （键名重写，值原样保留）
//   - x-request-id → request-id（Anthropic 风格小写无前缀）
//
// 仅在 persona 模式下调用。其他流量不受影响。
func NormalizeForAnthropicPersona(dst http.Header) {
	if dst == nil {
		return
	}

	// 收集需要 rename 或删除的键，避免在迭代中改 map
	type rename struct{ from, to string }
	var renames []rename
	var deletes []string

	for key := range dst {
		lower := strings.ToLower(key)

		if anthropicName, ok := openAIToAnthropicRateLimit[lower]; ok {
			renames = append(renames, rename{from: key, to: anthropicName})
			continue
		}
		if lower == openAIRequestIDKey {
			renames = append(renames, rename{from: key, to: anthropicRequestIDKey})
			continue
		}

		// 已经是 anthropic-ratelimit-* / request-id 时保持
		if strings.HasPrefix(lower, "anthropic-") || lower == anthropicRequestIDKey {
			continue
		}

		if _, hit := vendorIdentityHeaderExact[lower]; hit {
			deletes = append(deletes, key)
			continue
		}
		for _, prefix := range vendorIdentityHeaderPrefixes {
			if strings.HasPrefix(lower, prefix) {
				deletes = append(deletes, key)
				break
			}
		}

		// 漏网的 x-ratelimit-* 子健（比如 x-ratelimit-reset 不在白名单也兜底处理）
		if strings.HasPrefix(lower, openAIRateLimitPrefix) {
			deletes = append(deletes, key)
		}
	}

	for _, r := range renames {
		values := dst.Values(r.from)
		dst.Del(r.from)
		for _, v := range values {
			dst.Add(r.to, v)
		}
	}
	for _, k := range deletes {
		dst.Del(k)
	}
}
