package service

import (
	"context"
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// claudeCodePersonaCtxKey 用于在 gin/context 中携带「本次请求是否启用人设」标志。
// 用于跨服务调用边界（如 OpenAI Forward 不接收 ParsedRequest）传递该信号，
// 以便响应阶段做 model 名强制重写。
type claudeCodePersonaCtxKey struct{}

const claudeCodePersonaGinKey = "claude_code_persona"

// SetClaudeCodePersonaInContext 在 gin.Context 中标记本次请求需要人设语义。
// 在 handler 解析完 apiKey.Group.ClaudeCodePersona 之后调用一次。
func SetClaudeCodePersonaInContext(c *gin.Context, enabled bool) {
	if c == nil || !enabled {
		return
	}
	c.Set(claudeCodePersonaGinKey, true)
}

// IsClaudeCodePersonaForced 读取 gin/context.Context 中的人设标记。
// 任意一种载体存在即返回 true。
func IsClaudeCodePersonaForced(ctx context.Context, c *gin.Context) bool {
	if c != nil {
		if v, ok := c.Get(claudeCodePersonaGinKey); ok {
			if b, ok := v.(bool); ok && b {
				return true
			}
		}
	}
	if ctx != nil {
		if v, ok := ctx.Value(claudeCodePersonaCtxKey{}).(bool); ok && v {
			return true
		}
	}
	return false
}

// WithClaudeCodePersona 派生一个携带人设标记的 context（用于纯 ctx 调用链）。
func WithClaudeCodePersona(ctx context.Context, enabled bool) context.Context {
	if ctx == nil || !enabled {
		return ctx
	}
	return context.WithValue(ctx, claudeCodePersonaCtxKey{}, true)
}

// claudeCodePersonaPromptText 是注入到上游请求中的 Claude Code 人设系统提示词。
// 内容随二进制编译时嵌入，运行期不可热更新。
//
//go:embed prompts/claude_code_persona.txt
var claudeCodePersonaPromptText string

// claudeCodePersonaSentinel 是幂等检查锚点：若上游请求体中已包含该子串，
// 则视为已注入，跳过本次注入避免叠加。**该字符串必须出现在 prompt 文件首句**。
const claudeCodePersonaSentinel = "You are Claude Code, Anthropic's official CLI for Claude."

// ClaudeCodePersonaPrompt 返回当前持久化的人设提示词内容。
// 暴露给 handler/测试以便构造或断言。
func ClaudeCodePersonaPrompt() string {
	return claudeCodePersonaPromptText
}

// hasClaudeCodePersona 检查 body 中任意位置是否已含人设标识。
// 用于跨平台幂等判断：不同平台调用各自的 inject 函数前都先做此检查。
func hasClaudeCodePersona(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	return strings.Contains(string(body), claudeCodePersonaSentinel)
}

// InjectClaudeCodePersonaAnthropic 在 Anthropic /v1/messages 请求体中注入人设提示词。
// 注入到顶层 system 字段：
//   - system 缺失或为字符串：组合后写回字符串；
//   - system 为数组（content blocks）：在数组首位插入 {"type":"text","text":<persona>}。
//
// 幂等：若已含 sentinel 则原样返回。
func InjectClaudeCodePersonaAnthropic(body []byte) []byte {
	if len(body) == 0 || hasClaudeCodePersona(body) {
		return body
	}
	persona := claudeCodePersonaPromptText

	systemNode := gjson.GetBytes(body, "system")
	switch {
	case !systemNode.Exists():
		// 直接写字符串
		out, err := sjson.SetBytes(body, "system", persona)
		if err != nil {
			return body
		}
		return out

	case systemNode.IsArray():
		block := map[string]any{"type": "text", "text": persona}
		// 前置插入：sjson 用 -1 是追加，没有"前置"原生语法，先解码再覆写
		var arr []any
		if err := json.Unmarshal([]byte(systemNode.Raw), &arr); err != nil {
			return body
		}
		arr = append([]any{block}, arr...)
		out, err := sjson.SetBytes(body, "system", arr)
		if err != nil {
			return body
		}
		return out

	default:
		// 字符串或其他原始类型：拼接为新字符串
		existing := systemNode.String()
		merged := persona
		if strings.TrimSpace(existing) != "" {
			merged = persona + "\n\n" + existing
		}
		out, err := sjson.SetBytes(body, "system", merged)
		if err != nil {
			return body
		}
		return out
	}
}

// InjectClaudeCodePersonaChatMessages 在 OpenAI /v1/chat/completions 请求体的
// messages 数组中注入人设。如首条 role==system，则将人设 prepend 到其 content；
// 否则在数组首位插入 {role:"system", content:<persona>}。幂等。
func InjectClaudeCodePersonaChatMessages(body []byte) []byte {
	if len(body) == 0 || hasClaudeCodePersona(body) {
		return body
	}
	persona := claudeCodePersonaPromptText

	messagesNode := gjson.GetBytes(body, "messages")
	if !messagesNode.Exists() || !messagesNode.IsArray() {
		// 没有 messages 结构，直接写一个新的
		out, err := sjson.SetBytes(body, "messages", []any{
			map[string]any{"role": "system", "content": persona},
		})
		if err != nil {
			return body
		}
		return out
	}

	var arr []any
	if err := json.Unmarshal([]byte(messagesNode.Raw), &arr); err != nil {
		return body
	}

	if len(arr) > 0 {
		if first, ok := arr[0].(map[string]any); ok {
			if role, _ := first["role"].(string); role == "system" || role == "developer" {
				switch existing := first["content"].(type) {
				case string:
					first["content"] = persona + "\n\n" + existing
				case []any:
					prefixBlock := map[string]any{"type": "text", "text": persona}
					first["content"] = append([]any{prefixBlock}, existing...)
				default:
					first["content"] = persona
				}
				arr[0] = first
				out, err := sjson.SetBytes(body, "messages", arr)
				if err != nil {
					return body
				}
				return out
			}
		}
	}

	arr = append([]any{
		map[string]any{"role": "system", "content": persona},
	}, arr...)
	out, err := sjson.SetBytes(body, "messages", arr)
	if err != nil {
		return body
	}
	return out
}

// InjectClaudeCodePersonaInstructions 在 OpenAI /v1/responses 请求体的
// instructions 字段（字符串）头部前置人设。幂等。
func InjectClaudeCodePersonaInstructions(body []byte) []byte {
	if len(body) == 0 || hasClaudeCodePersona(body) {
		return body
	}
	persona := claudeCodePersonaPromptText

	node := gjson.GetBytes(body, "instructions")
	merged := persona
	if node.Exists() && strings.TrimSpace(node.String()) != "" {
		merged = persona + "\n\n" + node.String()
	}
	out, err := sjson.SetBytes(body, "instructions", merged)
	if err != nil {
		return body
	}
	return out
}

// InjectClaudeCodePersonaGemini 在 Gemini generateContent 请求体的
// systemInstruction.parts[0].text 头部前置人设。幂等。
func InjectClaudeCodePersonaGemini(body []byte) []byte {
	if len(body) == 0 || hasClaudeCodePersona(body) {
		return body
	}
	persona := claudeCodePersonaPromptText

	textNode := gjson.GetBytes(body, "systemInstruction.parts.0.text")
	if textNode.Exists() && strings.TrimSpace(textNode.String()) != "" {
		merged := persona + "\n\n" + textNode.String()
		out, err := sjson.SetBytes(body, "systemInstruction.parts.0.text", merged)
		if err != nil {
			return body
		}
		return out
	}
	// systemInstruction 缺失或 parts[0] 缺失：整体写入
	out, err := sjson.SetBytes(body, "systemInstruction", map[string]any{
		"parts": []any{
			map[string]any{"text": persona},
		},
	})
	if err != nil {
		return body
	}
	return out
}
