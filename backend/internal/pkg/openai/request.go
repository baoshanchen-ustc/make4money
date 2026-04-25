package openai

import "strings"

// CodexCLIUserAgentPrefixes matches Codex CLI User-Agent patterns
// Examples: "codex_vscode/1.0.0", "codex_cli_rs/0.1.2"
var CodexCLIUserAgentPrefixes = []string{
	"codex_vscode/",
	"codex_cli_rs/",
}

// CodexOfficialClientUserAgentPrefixes matches Codex 官方客户端家族 User-Agent 前缀。
// 该列表仅用于 OpenAI OAuth `codex_cli_only` 访问限制判定。
var CodexOfficialClientUserAgentPrefixes = []string{
	"codex_cli_rs/",
	"codex_vscode/",
	"codex-tui/",
	"codex_app/",
	"codex_chatgpt_desktop/",
	"codex_atlas/",
	"codex_exec/",
	"codex_sdk_ts/",
	"codex ",
}

// CodexOfficialClientOriginatorPrefixes matches Codex 官方客户端家族 originator 前缀。
// 说明：OpenAI 官方 Codex 客户端并不只使用固定的 codex_app 标识。
// 例如 codex_cli_rs、codex_vscode、codex_chatgpt_desktop、codex_atlas、codex_exec、codex_sdk_ts 等。
var CodexOfficialClientOriginatorPrefixes = []string{
	"codex_cli_rs",
	"codex_vscode",
	"codex-tui",
	"codex_app",
	"codex_chatgpt_desktop",
	"codex_atlas",
	"codex_exec",
	"codex_sdk_ts",
	"codex desktop",
}

// IsCodexCLIRequest checks if the User-Agent indicates a Codex CLI request
func IsCodexCLIRequest(userAgent string) bool {
	ua := normalizeCodexClientHeader(userAgent)
	if ua == "" {
		return false
	}
	return matchCodexClientHeaderPrefixes(ua, CodexCLIUserAgentPrefixes)
}

// IsCodexOfficialClientRequest checks if the User-Agent indicates a Codex 官方客户端请求。
// 与 IsCodexCLIRequest 解耦，避免影响历史兼容逻辑。
func IsCodexOfficialClientRequest(userAgent string) bool {
	ua := normalizeCodexClientHeader(userAgent)
	if ua == "" {
		return false
	}
	return matchCodexClientHeaderPrefixes(ua, CodexOfficialClientUserAgentPrefixes)
}

// IsCodexOfficialClientOriginator checks if originator indicates a Codex 官方客户端请求。
func IsCodexOfficialClientOriginator(originator string) bool {
	v := normalizeCodexClientHeader(originator)
	if v == "" {
		return false
	}
	return matchCodexClientOriginatorPrefixes(v, CodexOfficialClientOriginatorPrefixes)
}

// IsCodexOfficialClientByHeaders checks whether the request headers indicate an
// official Codex client family request.
func IsCodexOfficialClientByHeaders(userAgent, originator string) bool {
	return IsCodexOfficialClientRequest(userAgent) || IsCodexOfficialClientOriginator(originator)
}

// ExtractCodexClientVersion extracts the semantic-ish version token from a
// Codex official client User-Agent, for example "0.125.0" from
// "codex-tui/0.125.0" or "Codex Desktop/1.2.3".
func ExtractCodexClientVersion(userAgent string) string {
	ua := strings.TrimSpace(userAgent)
	if ua == "" {
		return ""
	}

	lowerUA := strings.ToLower(ua)
	searchFrom := 0
	for searchFrom < len(lowerUA) {
		offset := strings.Index(lowerUA[searchFrom:], "codex")
		if offset < 0 {
			return ""
		}

		start := searchFrom + offset
		searchFrom = start + len("codex")
		if start > 0 && isASCIIAlphaNumeric(lowerUA[start-1]) {
			continue
		}

		candidateLower := lowerUA[start:]
		if !hasOfficialCodexUserAgentPrefix(candidateLower) {
			continue
		}

		candidate := ua[start:]
		slash := strings.Index(candidate, "/")
		if slash < 0 || slash+1 >= len(candidate) {
			continue
		}

		version := trimCodexClientVersionToken(candidate[slash+1:])
		if version == "" {
			continue
		}
		if version[0] < '0' || version[0] > '9' {
			continue
		}
		return version
	}

	return ""
}

func normalizeCodexClientHeader(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func matchCodexClientHeaderPrefixes(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		normalizedPrefix := normalizeCodexClientHeader(prefix)
		if normalizedPrefix == "" {
			continue
		}
		// 优先前缀匹配；若 UA/Originator 被网关拼接为复合字符串时，退化为包含匹配。
		if strings.HasPrefix(value, normalizedPrefix) || strings.Contains(value, normalizedPrefix) {
			return true
		}
	}
	return false
}

func matchCodexClientOriginatorPrefixes(value string, prefixes []string) bool {
	if matchCodexClientTokenPrefixes(value, prefixes) {
		return true
	}

	for _, token := range splitCodexClientCompositeHeader(value) {
		if matchCodexClientTokenPrefixes(token, prefixes) {
			return true
		}
	}
	return false
}

func matchCodexClientTokenPrefixes(value string, prefixes []string) bool {
	normalizedValue := normalizeCodexClientHeader(value)
	if normalizedValue == "" {
		return false
	}
	for _, prefix := range prefixes {
		normalizedPrefix := normalizeCodexClientHeader(prefix)
		if normalizedPrefix == "" {
			continue
		}
		if strings.HasPrefix(normalizedValue, normalizedPrefix) {
			return true
		}
	}
	return false
}

func splitCodexClientCompositeHeader(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ',', ';', '|':
			return true
		default:
			return false
		}
	})
}

func hasOfficialCodexUserAgentPrefix(value string) bool {
	for _, prefix := range CodexOfficialClientUserAgentPrefixes {
		normalizedPrefix := normalizeCodexClientHeader(prefix)
		if normalizedPrefix == "" {
			continue
		}
		if strings.HasPrefix(value, normalizedPrefix) {
			return true
		}
	}
	return false
}

func isASCIIAlphaNumeric(b byte) bool {
	return (b >= '0' && b <= '9') ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z')
}

func trimCodexClientVersionToken(value string) string {
	end := 0
	for end < len(value) {
		ch := value[end]
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch == '.', ch == '-', ch == '_':
		default:
			return value[:end]
		}
		end++
	}
	return value[:end]
}
