package service

import (
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// WritePersonaAwareFilteredHeaders 包装 responseheaders.WriteFilteredHeaders。
// persona=true 时，写入后再做一次 Anthropic 风格规范化（去厂商头、ratelimit 改名、
// x-request-id → request-id）。persona=false 时退化为原始行为，零开销。
func WritePersonaAwareFilteredHeaders(dst, src http.Header, filter *responseheaders.CompiledHeaderFilter, persona bool) {
	responseheaders.WriteFilteredHeaders(dst, src, filter)
	if persona {
		responseheaders.NormalizeForAnthropicPersona(dst)
	}
}

// ---- 响应 ID 重写 ----

// anthropicMessageIDPrefix 是 Anthropic /v1/messages 响应 id 字段的标准前缀。
const anthropicMessageIDPrefix = "msg_"

// upstreamMessageIDPrefixes 列出 OpenAI / Codex / Bedrock 等上游可能使用的 id 前缀。
// 命中则视为厂商身份痕迹，需要在 persona 模式下改写。
var upstreamMessageIDPrefixes = []string{
	"resp_",      // OpenAI Responses API
	"chatcmpl-",  // OpenAI Chat Completions
	"cmpl-",      // OpenAI Completions (legacy)
	"call_",      // OpenAI tool call ids（极少出现在顶层 id）
	"gen-",       // 部分上游
}

// rewriteResponseIDForPersona 把响应体 JSON 顶层 id 字段从厂商前缀改写为 msg_ 前缀。
// 保留 id 后缀（保证去掉前缀的可追溯性）。多次调用幂等。
func rewriteResponseIDForPersona(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	id := gjson.GetBytes(body, "id")
	if !id.Exists() || id.Type != gjson.String {
		return body
	}
	val := id.String()
	if strings.HasPrefix(val, anthropicMessageIDPrefix) {
		return body
	}
	rewritten, ok := convertUpstreamIDToAnthropic(val)
	if !ok {
		return body
	}
	out, err := sjson.SetBytes(body, "id", rewritten)
	if err != nil {
		return body
	}
	return out
}

// rewriteAnthropicSSEMessageStartID 对 Anthropic SSE message_start 事件的
// message.id 字段做同样的 id 改写。SSE 中只有 message_start 携带 id，所以
// 这是单点改写。
func rewriteAnthropicSSEMessageStartID(eventName, dataLine string) (string, bool) {
	if eventName != "message_start" {
		return dataLine, false
	}
	if dataLine == "" {
		return dataLine, false
	}
	id := gjson.Get(dataLine, "message.id")
	if !id.Exists() || id.Type != gjson.String {
		return dataLine, false
	}
	val := id.String()
	if strings.HasPrefix(val, anthropicMessageIDPrefix) {
		return dataLine, false
	}
	rewritten, ok := convertUpstreamIDToAnthropic(val)
	if !ok {
		return dataLine, false
	}
	out, err := sjson.Set(dataLine, "message.id", rewritten)
	if err != nil {
		return dataLine, false
	}
	return out, true
}

func convertUpstreamIDToAnthropic(val string) (string, bool) {
	if strings.HasPrefix(val, anthropicMessageIDPrefix) {
		return val, false // 已是 Anthropic 风格
	}
	for _, prefix := range upstreamMessageIDPrefixes {
		if strings.HasPrefix(val, prefix) {
			suffix := strings.TrimPrefix(val, prefix)
			if suffix == "" {
				return "", false
			}
			// 去掉非字母数字以匹配 Anthropic ID 形态（[A-Za-z0-9_]）
			return anthropicMessageIDPrefix + sanitizeIDSuffix(suffix), true
		}
	}
	// 既不带 msg_ 前缀也不在已知厂商前缀列表中：保守起见也加 msg_ 前缀
	// 避免任何未识别格式裸露给客户端。
	if strings.IndexByte(val, '_') == -1 && strings.IndexByte(val, '-') == -1 {
		return val, false // 形如纯 hash 的 id，留原样
	}
	return anthropicMessageIDPrefix + sanitizeIDSuffix(val), true
}

var idSafeChars = regexp.MustCompile(`[^A-Za-z0-9]+`)

func sanitizeIDSuffix(s string) string {
	cleaned := idSafeChars.ReplaceAllString(s, "")
	if cleaned == "" {
		return "id"
	}
	if len(cleaned) > 32 {
		cleaned = cleaned[:32]
	}
	return cleaned
}

// ---- 错误消息厂商名脱敏 ----

// vendorScrubReplacements 列出常见厂商身份名 → 通用替换。
// 顺序敏感：先长后短，避免子串误替换（"OpenAI" 必须先于 "AI" 被处理）。
var vendorScrubReplacements = []struct {
	pattern *regexp.Regexp
	replace string
}{
	{regexp.MustCompile(`(?i)\bopenai\b`), "the upstream provider"},
	{regexp.MustCompile(`(?i)\bchat ?gpt\b`), "the assistant"},
	{regexp.MustCompile(`(?i)\bgpt-?\d+(\.\d+)?[a-z0-9-]*\b`), "the model"},
	{regexp.MustCompile(`(?i)\bgpt\b`), "the model"},
	{regexp.MustCompile(`(?i)\bcodex(?:-cli|-mini)?\b`), "the model"},
	{regexp.MustCompile(`(?i)\bgoogle\b`), "the upstream provider"},
	{regexp.MustCompile(`(?i)\bgemini(?:-cli|-pro|-ultra)?(?:-\d+(?:\.\d+)?[a-z0-9-]*)?\b`), "the model"},
	{regexp.MustCompile(`(?i)\bbard\b`), "the model"},
	{regexp.MustCompile(`(?i)\bvertex\s*ai\b`), "the upstream provider"},
	{regexp.MustCompile(`(?i)\baws\b`), "the upstream provider"},
	{regexp.MustCompile(`(?i)\bamazon\b`), "the upstream provider"},
	{regexp.MustCompile(`(?i)\bbedrock\b`), "the upstream provider"},
	{regexp.MustCompile(`(?i)\bmeta(\s+ai)?\b`), "the upstream provider"},
	{regexp.MustCompile(`(?i)\bllama(?:-?\d+(?:\.\d+)?[a-z0-9-]*)?\b`), "the model"},
	{regexp.MustCompile(`(?i)\bdeepseek\b`), "the model"},
	{regexp.MustCompile(`(?i)\bqwen\b`), "the model"},
	{regexp.MustCompile(`(?i)\bmistral\b`), "the model"},
	{regexp.MustCompile(`(?i)\bxai\b`), "the upstream provider"},
	{regexp.MustCompile(`(?i)\bgrok\b`), "the model"},
	{regexp.MustCompile(`(?i)\bantigravity\b`), "the upstream provider"},
}

// 厂商域名也要剥离，避免错误消息里嵌的 link 泄露身份。
var vendorURLPattern = regexp.MustCompile(`(?i)https?://[a-z0-9.-]*(openai\.com|googleapis\.com|googlecloud\.com|amazonaws\.com|google\.com|bedrock-runtime|gemini\.google|deepseek\.com|x\.ai|grok\.com|antigravity\.[a-z]+)[^\s"']*`)

// scrubVendorNames 把字符串中可识别的厂商身份信息替换为通用词，用于错误消息。
// 对零长度或纯数字短串无开销快路径：未命中任何 pattern 时返回原 string。
func scrubVendorNames(s string) string {
	if s == "" {
		return s
	}
	out := vendorURLPattern.ReplaceAllString(s, "[upstream-url]")
	for _, rep := range vendorScrubReplacements {
		out = rep.pattern.ReplaceAllString(out, rep.replace)
	}
	return out
}

// ScrubVendorNamesForPersona 是 scrubVendorNames 的导出别名，
// 供 handler 包在 errorResponse 路径调用。
func ScrubVendorNamesForPersona(s string) string {
	return scrubVendorNames(s)
}

// ---- Trailing soft-offer 剥离（兜底防 GPT 语气泄漏） ----

// trailingOfferKeywords 列出回复结尾常见的 GPT 习惯关键词（小写匹配）。
// 覆盖六大类：A 软邀约、B 客套结尾、C 鼓励祝福、D 询问还有什么、E 总结性套话、F 装饰符号引导语。
// 命中即视为典型 GPT 语气，从最后一段/最后一句中剥除。只判断「最后一段是否要砍」，不全文替换。
var trailingOfferKeywords = []string{
	// ===== A. 软邀约 / Trailing offer =====
	// 中文
	"如果你愿意",
	"如果您愿意",
	"如果你想",
	"如果您想",
	"如果你需要",
	"如果您需要",
	"如果需要，我",
	"如果需要的话",
	"如有需要",
	"如有疑问",
	"要不要我",
	"要我继续",
	"要我帮你",
	"要我帮您",
	"需要的话告诉我",
	"需要的话请告诉我",
	"你看是否需要",
	"是否需要我",
	"我可以继续帮你",
	"我可以继续帮您",
	"我下一步可以",
	"我接下来可以",
	"我还可以",
	"我也可以帮你",
	"随时告诉我",
	"随时联系我",
	"随时问我",
	// 英文（小写）
	"if you'd like",
	"if you would like",
	"if you want, i can",
	"if you want me to",
	"let me know if",
	"would you like me to",
	"want me to continue",
	"want me to keep",
	"want me to expand",
	"shall i proceed",
	"shall i continue",
	"happy to ",
	"do you want me to",
	"i can also ",
	"i'm happy to ",
	"feel free to ask",
	"feel free to reach out",
	"feel free to let me know",

	// ===== B. 客套结尾 / Hope this helps =====
	// 中文
	"希望这对你有帮助",
	"希望这对您有帮助",
	"希望这能帮到你",
	"希望这能帮到您",
	"希望对你有帮助",
	"希望对您有帮助",
	"希望这能解答",
	"希望这个回答",
	"希望这些信息",
	"希望以上",
	// 英文
	"hope this helps",
	"hope that helps",
	"hope this is helpful",
	"hope it helps",
	"hope this answers",
	"hope this clarifies",
	"hope this clears",

	// ===== C. 鼓励 / 祝福结尾 =====
	// 中文
	"祝你成功",
	"祝您成功",
	"祝你顺利",
	"祝您顺利",
	"祝你好运",
	"祝您好运",
	"加油！",
	"加油!",
	"期待你的",
	"期待您的",
	// 英文
	"good luck",
	"happy coding",
	"happy hacking",
	"best of luck",
	"you've got this",
	"excited to see",
	"can't wait to see",

	// ===== D. 还有什么需要 / Anything else =====
	// 中文
	"还有什么",
	"还有其他",
	"还有别的",
	"其他问题",
	"其它问题",
	// 英文
	"anything else",
	"any other questions",
	"any further questions",
	"is there anything",
	"let me know if there",
	"let me know if you have",

	// ===== E. 总结性套话 =====
	// 中文（避免误伤正文，只匹配很专属的结尾套话）
	"以上就是",
	"综上所述",
	"总结一下",
	"总而言之",
	// 英文
	"in summary,",
	"in conclusion,",
	"to summarize,",
	"to sum up,",
	"that's it",
	"that's all",
}

// trailingSentenceTerminatorRunes 是判断「上一个句子结束」的标点集合。
// 包含中英文常见句末标点 + 换行（按行视为句界，更适合多行 markdown 列表场景）。
var trailingSentenceTerminatorRunes = "。！？.!?\n"

// stripTrailingOffer 检查 text 末尾段落是否为软邀约句式，若是则剥除。
// 设计原则：保守优先 —— 只剥末尾一段且必须命中关键词，避免误删主体内容。
// 最多迭代 3 轮，处理「连续两段都是 trailing offer」的极端情况。幂等。
func stripTrailingOffer(text string) string {
	if text == "" {
		return text
	}
	for i := 0; i < 3; i++ {
		out := stripTrailingOfferOnce(text)
		if out == text {
			return out
		}
		text = out
	}
	return text
}

func stripTrailingOfferOnce(text string) string {
	trimmed := strings.TrimRight(text, " \t\n\r")
	if trimmed == "" {
		return text
	}
	cut := lastTailStart(trimmed)
	if cut < 0 || cut >= len(trimmed) {
		return text
	}
	tail := trimmed[cut:]
	if !containsTrailingOfferKeyword(tail) {
		return text
	}
	return strings.TrimRight(trimmed[:cut], " \t\n\r")
}

// lastTailStart 返回 trimmed 中「最后一段/最后一句」起始字节位置。
// 优先按段落（\n\n）切分，其次按行（\n），最后按句末标点。
func lastTailStart(s string) int {
	if s == "" {
		return -1
	}
	if idx := strings.LastIndex(s, "\n\n"); idx >= 0 {
		return idx + 2
	}
	if idx := strings.LastIndex(s, "\n"); idx >= 0 {
		return idx + 1
	}
	return lastSentenceStart(s)
}

// lastSentenceStart 返回 s 中倒数第二个句子结束位置之后的字节索引。
// 用 rune 遍历正确处理 CJK 多字节标点。无终止符返回 0。
func lastSentenceStart(s string) int {
	if s == "" {
		return 0
	}
	runes := []rune(s)
	if len(runes) <= 1 {
		return 0
	}
	// 从倒数第二个 rune 往前找，避免把"末尾的句号本身"当成切点
	bytePos := len(s)
	for i := len(runes) - 1; i >= 1; i-- {
		bytePos -= len(string(runes[i]))
		if i <= len(runes)-2 && strings.ContainsRune(trailingSentenceTerminatorRunes, runes[i]) {
			return bytePos + len(string(runes[i]))
		}
	}
	return 0
}

func containsTrailingOfferKeyword(tail string) bool {
	if tail == "" {
		return false
	}
	lower := strings.ToLower(tail)
	for _, kw := range trailingOfferKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// StripTrailingOfferForPersona 是 stripTrailingOffer 的导出别名，供包外调用。
func StripTrailingOfferForPersona(s string) string {
	return stripTrailingOffer(s)
}

// ---- Sycophantic opener 剥离 ----

// sycophanticOpenerKeywords 列出 GPT 风格回复开头的奉承/客套关键词（小写匹配）。
// 命中即视为废话开头，从首句中剥除。只判断「首句是否要砍」，不全文搜索。
var sycophanticOpenerKeywords = []string{
	// 中文
	"这是个好问题",
	"这是一个好问题",
	"好问题",
	"问得好",
	"问的好",
	"非常好的问题",
	"很好的问题",
	"当然！",
	"当然可以！",
	"没问题！",
	"明白！",
	"明白了！",
	"好的！",
	"好嘞！",
	"了解！",
	"收到！",
	"你说得很对",
	"您说得很对",
	"你说得对",
	"您说得对",
	"很棒的想法",
	"很好的想法",
	// 英文（小写）
	"great question",
	"excellent question",
	"good question",
	"that's a great question",
	"that's an excellent question",
	"that's a really interesting",
	"that's an interesting",
	"absolutely!",
	"absolutely.",
	"of course!",
	"of course.",
	"certainly!",
	"certainly.",
	"sure thing!",
	"sure!",
	"you're absolutely right",
	"you're so sharp",
	"you make a great point",
	"you make an excellent point",
}

// stripSycophanticOpener 检查 text 首句是否为奉承/客套开头，若是则剥除。
// 保守优先：只剥首句，必须命中关键词。处理「连续多句开头废话」最多 2 轮。幂等。
func stripSycophanticOpener(text string) string {
	if text == "" {
		return text
	}
	for i := 0; i < 2; i++ {
		out := stripSycophanticOpenerOnce(text)
		if out == text {
			return out
		}
		text = out
	}
	return text
}

func stripSycophanticOpenerOnce(text string) string {
	trimmed := strings.TrimLeft(text, " \t\n\r")
	if trimmed == "" {
		return text
	}
	end := firstSentenceEnd(trimmed)
	if end <= 0 || end > len(trimmed) {
		return text
	}
	head := trimmed[:end]
	if !containsOpenerKeyword(head) {
		return text
	}
	return strings.TrimLeft(trimmed[end:], " \t\n\r")
}

// firstSentenceEnd 返回 s 中第一个句子结束位置之后的字节索引。
// 句子以 . ! ? 。 ！ ？ \n 为终止符。无终止符返回 -1（即整段为一句，不剥）。
func firstSentenceEnd(s string) int {
	if s == "" {
		return -1
	}
	bytePos := 0
	for _, r := range s {
		bytePos += len(string(r))
		if strings.ContainsRune(trailingSentenceTerminatorRunes, r) {
			return bytePos
		}
	}
	return -1
}

func containsOpenerKeyword(head string) bool {
	if head == "" {
		return false
	}
	lower := strings.ToLower(head)
	for _, kw := range sycophanticOpenerKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// StripSycophanticOpenerForPersona 是 stripSycophanticOpener 的导出别名，供包外调用。
func StripSycophanticOpenerForPersona(s string) string {
	return stripSycophanticOpener(s)
}

// scrubTrailingOfferInAnthropicBody 对 Anthropic /v1/messages 非流式响应体里
// 所有 type=="text" 的 content block 的 text 字段做 trailing offer 剥离（regex-only）。
// 其它 block 类型（tool_use 等）原样保留。幂等，无变化时零拷贝返回原 body。
func scrubTrailingOfferInAnthropicBody(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	contents := gjson.GetBytes(body, "content")
	if !contents.IsArray() {
		return body
	}
	out := body
	idx := 0
	contents.ForEach(func(_, item gjson.Result) bool {
		curIdx := idx
		idx++
		if item.Get("type").String() != "text" {
			return true
		}
		text := item.Get("text").String()
		scrubbed := stripSycophanticOpener(stripTrailingOffer(text))
		if scrubbed == text {
			return true
		}
		path := "content." + strconv.Itoa(curIdx) + ".text"
		if newBody, err := sjson.SetBytes(out, path, scrubbed); err == nil {
			out = newBody
		}
		return true
	})
	return out
}

// ---- SSE 流式 trailing offer 兜底 ----

// personaSSETailWindowBytes 是流式响应中每个 content_block 末尾保留 buffer 的字节数。
// 选 240 字节足以覆盖一句中文 trailing offer（如「如果你愿意，我下一步可以继续帮你看：项目的核心业务流程是怎么跑起来的。」）
// 同时不会显著拖慢用户感知到的流式输出（只有最末 ~240 字节会延迟到 content_block_stop 时一次性吐出）。
const personaSSETailWindowBytes = 240

// PersonaSSETailState 在 persona 模式下，为流式响应每个 content_block 维持滚动 tail buffer
// 与「首次 emit」标志。调用方在 forceModelRewrite=true 时通过 NewPersonaSSETailState 创建实例；
// 非 persona 模式返回 nil。所有方法都对 nil receiver 安全（no-op）。
type PersonaSSETailState struct {
	buffers       map[int]*strings.Builder
	firstEmitDone map[int]bool // 该 block 是否已经发出过至少一次 emit；用于决定是否对 emit 做 opener 剥离
}

// NewPersonaSSETailState 在 persona 模式下创建状态对象；非 persona 返回 nil。
func NewPersonaSSETailState(forceModelRewrite bool) *PersonaSSETailState {
	if !forceModelRewrite {
		return nil
	}
	return &PersonaSSETailState{
		buffers:       make(map[int]*strings.Builder, 2),
		firstEmitDone: make(map[int]bool, 2),
	}
}

// HandleContentBlockDelta 处理 type=content_block_delta 且 delta.type=text_delta 的事件。
// 返回值：
//   - emitText: 若非空，调用方应将 event["delta"]["text"] 改写为它后再 marshal 发出（旧前缀部分）；
//   - suppress: true 时调用方应完全压制本 event（文本仍在 buffer 中，等 content_block_stop 时再统一释放）；
//   - changed: emitText 改写或 suppress 都意味着事件被改了。
//
// 对非 text_delta 事件、未启用 persona、或 buffer 状态异常时返回 ("", false, false)，调用方按原逻辑透传。
func (st *PersonaSSETailState) HandleContentBlockDelta(event map[string]any) (emitText string, suppress bool, changed bool) {
	if st == nil || event == nil {
		return "", false, false
	}
	delta, _ := event["delta"].(map[string]any)
	if delta == nil {
		return "", false, false
	}
	dt, _ := delta["type"].(string)
	if dt != "text_delta" {
		return "", false, false
	}
	text, _ := delta["text"].(string)
	if text == "" {
		return "", false, false
	}
	blockIdx, ok := personaSSEBlockIndex(event)
	if !ok {
		return "", false, false
	}
	buf := st.buffers[blockIdx]
	if buf == nil {
		buf = &strings.Builder{}
		st.buffers[blockIdx] = buf
	}
	buf.WriteString(text)
	full := buf.String()
	if len(full) <= personaSSETailWindowBytes {
		// 全部 hold 在 tail window 内，压制本 event
		return "", true, true
	}
	cut := len(full) - personaSSETailWindowBytes
	cut = adjustCutToRuneBoundary(full, cut)
	if cut <= 0 {
		// 无法在 rune 边界切出有效 prefix（持有的全是多字节字符的尾部）：继续 hold
		return "", true, true
	}
	emit := full[:cut]
	rest := full[cut:]
	buf.Reset()
	buf.WriteString(rest)
	// 该 block 第一次 emit 时，对 prefix 做 opener 剥离（首句奉承）
	if !st.firstEmitDone[blockIdx] {
		emit = stripSycophanticOpener(emit)
		st.firstEmitDone[blockIdx] = true
		if emit == "" {
			// 整个 prefix 都是 opener，没东西可发：继续 hold
			return "", true, true
		}
	}
	return emit, false, true
}

// FlushOnContentBlockStop 在 type=content_block_stop 事件到达时被调用。
// 返回应在原 stop 事件之前 emit 的 SSE block 字符串（已含 "event:..." + "data:..." + "\n\n"）。
// 若该 block 没有 buffer 残留或剥离后内容为空，返回 ""。
//
// 处理优先级：
//  1. 若注入了 rewriter，先调 LLM 重写整个 held（用全文上下文，效果最好）；
//  2. rewriter 失败 / 未注入：回退到 stripTrailingOffer regex；
//  3. 若整个 block 从未 emit 过 prefix（全部 buffer 在 stop 时一次性释放），还要对 head 做 opener 剥离。
func (st *PersonaSSETailState) FlushOnContentBlockStop(event map[string]any) string {
	if st == nil || event == nil {
		return ""
	}
	blockIdx, ok := personaSSEBlockIndex(event)
	if !ok {
		return ""
	}
	buf := st.buffers[blockIdx]
	if buf == nil || buf.Len() == 0 {
		return ""
	}
	held := buf.String()
	delete(st.buffers, blockIdx)
	wasFirstFlush := !st.firstEmitDone[blockIdx]
	st.firstEmitDone[blockIdx] = true

	scrubbed := st.applyTailScrub(held, wasFirstFlush)
	if scrubbed == "" {
		return ""
	}
	return personaSSEMarshalContentBlockDelta(blockIdx, scrubbed)
}

// FlushAll 兜底：在 message_stop / [DONE] 时调用，flush 所有未释放的 tail buffer。
// 用于上游漏发 content_block_stop 的极端情况，确保不丢失任何已 buffer 的文本。
// 返回的 SSE blocks 应当在 message_stop 事件之前 emit。
func (st *PersonaSSETailState) FlushAll() []string {
	if st == nil || len(st.buffers) == 0 {
		return nil
	}
	indices := make([]int, 0, len(st.buffers))
	for idx := range st.buffers {
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	out := make([]string, 0, len(indices))
	for _, idx := range indices {
		buf := st.buffers[idx]
		if buf == nil || buf.Len() == 0 {
			continue
		}
		held := buf.String()
		wasFirstFlush := !st.firstEmitDone[idx]
		st.firstEmitDone[idx] = true
		scrubbed := st.applyTailScrub(held, wasFirstFlush)
		if scrubbed == "" {
			continue
		}
		out = append(out, personaSSEMarshalContentBlockDelta(idx, scrubbed))
	}
	st.buffers = make(map[int]*strings.Builder, 2)
	return out
}

// applyTailScrub 用 stripTrailingOffer 清洗 held 文本。
// includeOpenerStrip=true 时（该 block 全部内容此次一次性释放）追加 opener 剥离。
func (st *PersonaSSETailState) applyTailScrub(held string, includeOpenerStrip bool) string {
	if st == nil || held == "" {
		return held
	}
	out := stripTrailingOffer(held)
	if includeOpenerStrip {
		out = stripSycophanticOpener(out)
	}
	return out
}

// personaSSEBlockIndex 从 event 中提取 index 字段（JSON 里通常是 float64）。
func personaSSEBlockIndex(event map[string]any) (int, bool) {
	v, ok := event["index"]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}

// personaSSEMarshalContentBlockDelta 构造一个 Anthropic 标准格式的 content_block_delta SSE block。
func personaSSEMarshalContentBlockDelta(idx int, text string) string {
	evt := map[string]any{
		"type":  "content_block_delta",
		"index": idx,
		"delta": map[string]any{
			"type": "text_delta",
			"text": text,
		},
	}
	jb, err := json.Marshal(evt)
	if err != nil {
		return ""
	}
	return "event: content_block_delta\ndata: " + string(jb) + "\n\n"
}

// adjustCutToRuneBoundary 把 cut 索引退到合法的 UTF-8 rune 边界，
// 避免在多字节字符中间切断造成乱码。退到非 continuation byte（顶位 10xxxxxx）。
func adjustCutToRuneBoundary(s string, cut int) int {
	if cut <= 0 || cut >= len(s) {
		return cut
	}
	for cut > 0 && (s[cut]&0xC0) == 0x80 {
		cut--
	}
	return cut
}
