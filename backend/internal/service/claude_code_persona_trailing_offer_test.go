package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ========== stripTrailingOffer ==========

func TestStripTrailingOffer_ChineseTrailingOffer(t *testing.T) {
	t.Parallel()
	in := "通常部署方式是 Docker Compose，也支持源码编译和脚本安装。\n如果你愿意，我下一步可以继续帮你看：项目的核心业务流程是怎么跑起来的。"
	out := stripTrailingOffer(in)
	require.NotContains(t, out, "如果你愿意")
	require.NotContains(t, out, "我下一步可以")
	require.Contains(t, out, "Docker Compose")
}

func TestStripTrailingOffer_EnglishTrailingOffer(t *testing.T) {
	t.Parallel()
	in := "The deployment uses Docker Compose; source build and install scripts are also supported.\nIf you'd like, I can walk you through the core business flow next."
	out := stripTrailingOffer(in)
	require.NotContains(t, out, "If you'd like")
	require.Contains(t, out, "Docker Compose")
}

func TestStripTrailingOffer_NoOfferReturnsUnchanged(t *testing.T) {
	t.Parallel()
	in := "Done. The function is at gateway_service.go:7240."
	out := stripTrailingOffer(in)
	require.Equal(t, in, out)
}

func TestStripTrailingOffer_EmptyAndWhitespace(t *testing.T) {
	t.Parallel()
	require.Equal(t, "", stripTrailingOffer(""))
	require.Equal(t, "   ", stripTrailingOffer("   ")) // pure whitespace untouched
}

func TestStripTrailingOffer_Idempotent(t *testing.T) {
	t.Parallel()
	in := "结论：已修复。\n如果需要，我可以继续帮你看后续的影响范围。"
	once := stripTrailingOffer(in)
	twice := stripTrailingOffer(once)
	require.Equal(t, once, twice)
	require.NotContains(t, once, "如果需要")
}

func TestStripTrailingOffer_StackedOffers(t *testing.T) {
	t.Parallel()
	in := "已完成。\n如果你愿意，我可以继续帮你看 X。\n要不要我帮你也看一下 Y？"
	out := stripTrailingOffer(in)
	require.NotContains(t, out, "如果你愿意")
	require.NotContains(t, out, "要不要我")
	require.Contains(t, out, "已完成")
}

func TestStripTrailingOffer_PreservesOfferWordsInBody(t *testing.T) {
	t.Parallel()
	// Offer-like keyword 出现在文中（非结尾段），不应被剥离
	in := "用户说「如果你愿意」时通常表示请求授权。\n这是设计原则的一部分。"
	out := stripTrailingOffer(in)
	require.Equal(t, in, out)
}

func TestStripTrailingOffer_OfferAsLastSentenceInSinglePara(t *testing.T) {
	t.Parallel()
	in := "已经修复完成。要不要我帮你看一下影响范围？"
	out := stripTrailingOffer(in)
	require.NotContains(t, out, "要不要我")
	require.Contains(t, out, "已经修复完成。")
}

// ========== scrubTrailingOfferInAnthropicBody ==========

func TestScrubAnthropicBody_TextBlock(t *testing.T) {
	t.Parallel()
	body := []byte(`{"id":"msg_x","type":"message","content":[{"type":"text","text":"已完成。\n如果你愿意，我下一步可以继续帮你看：业务流程。"}]}`)
	out := scrubTrailingOfferInAnthropicBody(body)
	s := string(out)
	require.NotContains(t, s, "如果你愿意")
	require.Contains(t, s, "已完成")
}

func TestScrubAnthropicBody_MultipleBlocks(t *testing.T) {
	t.Parallel()
	body := []byte(`{"content":[
        {"type":"text","text":"第一段。"},
        {"type":"tool_use","name":"calc","input":{}},
        {"type":"text","text":"主结论。\n如果需要，我可以继续帮你看后续。"}
    ]}`)
	out := scrubTrailingOfferInAnthropicBody(body)
	s := string(out)
	require.NotContains(t, s, "如果需要")
	require.Contains(t, s, "第一段")
	require.Contains(t, s, "tool_use")
	require.Contains(t, s, "主结论")
}

func TestScrubAnthropicBody_NoChangeWhenClean(t *testing.T) {
	t.Parallel()
	body := []byte(`{"content":[{"type":"text","text":"clean response."}]}`)
	out := scrubTrailingOfferInAnthropicBody(body)
	require.Equal(t, string(body), string(out))
}

func TestScrubAnthropicBody_EmptyBody(t *testing.T) {
	t.Parallel()
	require.Nil(t, scrubTrailingOfferInAnthropicBody(nil))
	require.Equal(t, []byte{}, scrubTrailingOfferInAnthropicBody([]byte{}))
}

// ========== PersonaSSETailState ==========

func TestPersonaSSETailState_DisabledWhenNotPersona(t *testing.T) {
	t.Parallel()
	st := NewPersonaSSETailState(false)
	require.Nil(t, st)

	// nil receiver methods 都是 no-op
	emit, sup, ch := st.HandleContentBlockDelta(map[string]any{"index": 0.0, "delta": map[string]any{"type": "text_delta", "text": "x"}})
	require.Equal(t, "", emit)
	require.False(t, sup)
	require.False(t, ch)
	require.Equal(t, "", st.FlushOnContentBlockStop(map[string]any{"index": 0.0}))
	require.Nil(t, st.FlushAll())
}

func TestPersonaSSETailState_BuffersSmallText(t *testing.T) {
	t.Parallel()
	st := NewPersonaSSETailState(true)
	event := map[string]any{
		"index": float64(0),
		"delta": map[string]any{"type": "text_delta", "text": "短文本"},
	}
	emit, suppress, changed := st.HandleContentBlockDelta(event)
	require.Equal(t, "", emit)
	require.True(t, suppress, "短文本应当被压制并 buffer")
	require.True(t, changed)
}

func TestPersonaSSETailState_EmitsPrefixWhenOverWindow(t *testing.T) {
	t.Parallel()
	st := NewPersonaSSETailState(true)
	// 拼一段超过 window (240 字节) 的文本
	long := strings.Repeat("A", 300)
	event := map[string]any{
		"index": float64(0),
		"delta": map[string]any{"type": "text_delta", "text": long},
	}
	emit, suppress, changed := st.HandleContentBlockDelta(event)
	require.False(t, suppress)
	require.True(t, changed)
	require.NotEmpty(t, emit)
	require.True(t, len(emit) >= 60, "应当发出超出 window 的部分")
}

func TestPersonaSSETailState_FlushScrubsTrailingOffer(t *testing.T) {
	t.Parallel()
	st := NewPersonaSSETailState(true)
	// 全部 buffer（短文本）
	st.HandleContentBlockDelta(map[string]any{
		"index": float64(0),
		"delta": map[string]any{"type": "text_delta", "text": "结论：已完成。\n如果你愿意，我下一步可以继续帮你看业务流程。"},
	})
	out := st.FlushOnContentBlockStop(map[string]any{"index": float64(0)})
	require.NotEmpty(t, out)
	require.Contains(t, out, "event: content_block_delta")
	require.Contains(t, out, "结论：已完成")
	require.NotContains(t, out, "如果你愿意")
}

func TestPersonaSSETailState_FlushReturnsEmptyWhenScrubLeavesNothing(t *testing.T) {
	t.Parallel()
	st := NewPersonaSSETailState(true)
	st.HandleContentBlockDelta(map[string]any{
		"index": float64(0),
		"delta": map[string]any{"type": "text_delta", "text": "如果你愿意，我可以继续。"},
	})
	out := st.FlushOnContentBlockStop(map[string]any{"index": float64(0)})
	// 整段都是 offer，scrub 后为空，不应 emit
	require.Equal(t, "", out)
}

func TestPersonaSSETailState_FlushAllAcrossBlocks(t *testing.T) {
	t.Parallel()
	st := NewPersonaSSETailState(true)
	st.HandleContentBlockDelta(map[string]any{
		"index": float64(0),
		"delta": map[string]any{"type": "text_delta", "text": "结论 A。"},
	})
	st.HandleContentBlockDelta(map[string]any{
		"index": float64(1),
		"delta": map[string]any{"type": "text_delta", "text": "结论 B。\n要不要我帮你继续看？"},
	})
	all := st.FlushAll()
	require.Len(t, all, 2)
	combined := strings.Join(all, "\n")
	require.Contains(t, combined, "结论 A")
	require.Contains(t, combined, "结论 B")
	require.NotContains(t, combined, "要不要我")
	// 二次调用应为空（buffers 已清）
	require.Nil(t, st.FlushAll())
}

func TestPersonaSSETailState_NonTextDeltaPassesThrough(t *testing.T) {
	t.Parallel()
	st := NewPersonaSSETailState(true)
	// tool_use delta 不应被 buffer
	emit, suppress, changed := st.HandleContentBlockDelta(map[string]any{
		"index": float64(0),
		"delta": map[string]any{"type": "input_json_delta", "partial_json": `{"a":1}`},
	})
	require.Equal(t, "", emit)
	require.False(t, suppress)
	require.False(t, changed)
}

// ========== adjustCutToRuneBoundary ==========

func TestAdjustCutToRuneBoundary_AsciiUnchanged(t *testing.T) {
	t.Parallel()
	s := "abcdef"
	require.Equal(t, 3, adjustCutToRuneBoundary(s, 3))
}

func TestAdjustCutToRuneBoundary_CJKWalksBack(t *testing.T) {
	t.Parallel()
	// 中 = E4 B8 AD（3 bytes）
	s := "中文字符" // 3*3 = 12 bytes
	// cut=2 在第一个 rune 的中间，应回退到 0
	require.Equal(t, 0, adjustCutToRuneBoundary(s, 2))
	// cut=3 是合法边界，保持
	require.Equal(t, 3, adjustCutToRuneBoundary(s, 3))
	// cut=5 在第二个 rune 的中间（3+2=5），回退到 3
	require.Equal(t, 3, adjustCutToRuneBoundary(s, 5))
}

func TestAdjustCutToRuneBoundary_OutOfRange(t *testing.T) {
	t.Parallel()
	require.Equal(t, 0, adjustCutToRuneBoundary("abc", 0))
	require.Equal(t, 3, adjustCutToRuneBoundary("abc", 3))
	require.Equal(t, -1, adjustCutToRuneBoundary("abc", -1))
}
