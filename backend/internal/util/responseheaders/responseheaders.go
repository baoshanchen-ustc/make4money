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

// gatewayTracePrefixes 是已知 LLM gateway / proxy 在响应头中追加的痕迹前缀。
// Claude Code 把这些前缀的存在视为 gateway detection 信号，可能调整流量归属或拒绝服务。
//
// 全部以小写形式比对，并使用 strings.HasPrefix 而非 exact match，
// 因为 gateway header 普遍带可变后缀（如 x-litellm-model-id / x-litellm-key-name）。
//
// 该列表是**安全边界**，比 additional_allowed 优先级更高：
// 即使管理员在 additional_allowed 中声明放行 "x-litellm-model-id"，仍会被这里拦截，
// 除非显式开启 ResponseHeaderConfig.AllowGatewayTraceHeaders（危险诊断 override，默认关闭）。
//
// 不要用 exact-match 的 ForceRemove 替代本前缀机制；前者无法覆盖未来新增的 gateway 派生头。
var gatewayTracePrefixes = []string{
	"x-litellm-",
	"helicone-",
	"x-portkey-",
	"cf-aig-",
	"x-kong-",
	"x-bt-",
}

type CompiledHeaderFilter struct {
	allowed                  map[string]struct{}
	forceRemove              map[string]struct{}
	gatewayTracePrefixes     []string
	allowGatewayTraceHeaders bool
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

	// 始终装载 gateway trace prefix denylist，无论 cfg.Enabled 是否打开；
	// "关闭可配置头过滤"不应退化为"完全放行 gateway 痕迹"。
	tracePrefixes := make([]string, len(gatewayTracePrefixes))
	copy(tracePrefixes, gatewayTracePrefixes)

	return &CompiledHeaderFilter{
		allowed:                  allowed,
		forceRemove:              forceRemove,
		gatewayTracePrefixes:     tracePrefixes,
		allowGatewayTraceHeaders: cfg.AllowGatewayTraceHeaders,
	}
}

// matchesGatewayTracePrefix 以小写 prefix 比对 lower 是否命中已知 gateway 痕迹。
func (f *CompiledHeaderFilter) matchesGatewayTracePrefix(lower string) bool {
	if f == nil {
		return false
	}
	for _, p := range f.gatewayTracePrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
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
		// gateway trace prefix denylist：优先级高于 additional_allowed。
		// 仅在显式 dangerous override 开启时才允许透传；默认安全。
		if !filter.allowGatewayTraceHeaders && filter.matchesGatewayTracePrefix(lower) {
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
