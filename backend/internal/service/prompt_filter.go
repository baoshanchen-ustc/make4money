package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	DefaultPromptFilterViolationLimit = 10
	DefaultPromptFilterWarningMessage = "请求包含违规关键词，已被拦截。"
	DefaultPromptFilterBanMessage     = "账号因多次提交违规内容已被禁用。"

	promptFilterCacheTTL   = 60 * time.Second
	promptFilterErrorTTL   = 5 * time.Second
	promptFilterDBTimeout  = 5 * time.Second
	promptFilterCacheKey   = "prompt_filter"
	promptFilterMaxTextLen = 1 << 20
)

type PromptFilterRuntime struct {
	Enabled        bool
	Keywords       []string
	ViolationLimit int
	WarningMessage string
	BanMessage     string
}

type cachedPromptFilterRuntime struct {
	runtime   PromptFilterRuntime
	expiresAt int64
}

var promptFilterRuntimeCache atomic.Value // *cachedPromptFilterRuntime
var promptFilterRuntimeSF singleflight.Group

func DefaultPromptFilterRuntime() PromptFilterRuntime {
	return PromptFilterRuntime{
		Enabled:        false,
		Keywords:       []string{},
		ViolationLimit: DefaultPromptFilterViolationLimit,
		WarningMessage: DefaultPromptFilterWarningMessage,
		BanMessage:     DefaultPromptFilterBanMessage,
	}
}

func ParsePromptFilterRuntime(settings map[string]string) PromptFilterRuntime {
	runtime := DefaultPromptFilterRuntime()
	if settings == nil {
		return runtime
	}
	runtime.Enabled = settings[SettingKeyPromptFilterEnabled] == "true"
	runtime.Keywords = parsePromptFilterKeywords(settings[SettingKeyPromptFilterKeywords])
	runtime.ViolationLimit = parsePromptFilterViolationLimit(settings[SettingKeyPromptFilterViolationLimit])
	if message := strings.TrimSpace(settings[SettingKeyPromptFilterWarningMessage]); message != "" {
		runtime.WarningMessage = message
	}
	if message := strings.TrimSpace(settings[SettingKeyPromptFilterBanMessage]); message != "" {
		runtime.BanMessage = message
	}
	return runtime
}

func NormalizePromptFilterKeywords(input []string) []string {
	if len(input) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, keyword := range input {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		key := strings.ToLower(keyword)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, keyword)
	}
	return out
}

func parsePromptFilterKeywords(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err == nil {
		return NormalizePromptFilterKeywords(items)
	}
	items = strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == '，' || r == ';' || r == '；'
	})
	return NormalizePromptFilterKeywords(items)
}

func parsePromptFilterViolationLimit(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return DefaultPromptFilterViolationLimit
	}
	return v
}

func (s *SettingService) GetPromptFilterRuntime(ctx context.Context) PromptFilterRuntime {
	if s == nil || s.settingRepo == nil {
		return DefaultPromptFilterRuntime()
	}
	if cached, ok := promptFilterRuntimeCache.Load().(*cachedPromptFilterRuntime); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.runtime
		}
	}
	result, _, _ := promptFilterRuntimeSF.Do(promptFilterCacheKey, func() (any, error) {
		if cached, ok := promptFilterRuntimeCache.Load().(*cachedPromptFilterRuntime); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.runtime, nil
			}
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), promptFilterDBTimeout)
		defer cancel()
		settings, err := s.settingRepo.GetMultiple(dbCtx, []string{
			SettingKeyPromptFilterEnabled,
			SettingKeyPromptFilterKeywords,
			SettingKeyPromptFilterViolationLimit,
			SettingKeyPromptFilterWarningMessage,
			SettingKeyPromptFilterBanMessage,
		})
		if err != nil {
			runtime := DefaultPromptFilterRuntime()
			if !errors.Is(err, ErrSettingNotFound) {
				slog.Warn("failed to get prompt filter settings", "error", err)
			}
			promptFilterRuntimeCache.Store(&cachedPromptFilterRuntime{
				runtime:   runtime,
				expiresAt: time.Now().Add(promptFilterErrorTTL).UnixNano(),
			})
			return runtime, nil
		}
		runtime := ParsePromptFilterRuntime(settings)
		promptFilterRuntimeCache.Store(&cachedPromptFilterRuntime{
			runtime:   runtime,
			expiresAt: time.Now().Add(promptFilterCacheTTL).UnixNano(),
		})
		return runtime, nil
	})
	if runtime, ok := result.(PromptFilterRuntime); ok {
		return runtime
	}
	return DefaultPromptFilterRuntime()
}

func RefreshPromptFilterRuntimeCache(runtime PromptFilterRuntime) {
	promptFilterRuntimeSF.Forget(promptFilterCacheKey)
	promptFilterRuntimeCache.Store(&cachedPromptFilterRuntime{
		runtime:   runtime,
		expiresAt: time.Now().Add(promptFilterCacheTTL).UnixNano(),
	})
}

func PromptFilterContainsKeyword(text string, keywords []string) bool {
	text = strings.TrimSpace(text)
	if text == "" || len(keywords) == 0 {
		return false
	}
	if len(text) > promptFilterMaxTextLen {
		text = text[:promptFilterMaxTextLen]
	}
	lowerText := strings.ToLower(text)
	for _, keyword := range NormalizePromptFilterKeywords(keywords) {
		if strings.Contains(lowerText, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func PromptTextFromJSON(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return ""
	}
	parts := make([]string, 0, 8)
	collectPromptText(payload, "", &parts)
	return strings.Join(parts, "\n")
}

func collectPromptText(value any, key string, parts *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for childKey, child := range typed {
			collectPromptText(child, strings.ToLower(strings.TrimSpace(childKey)), parts)
		}
	case []any:
		for _, child := range typed {
			collectPromptText(child, key, parts)
		}
	case string:
		if isPromptTextKey(key) {
			if text := strings.TrimSpace(typed); text != "" {
				*parts = append(*parts, text)
			}
		}
	}
}

func isPromptTextKey(key string) bool {
	switch key {
	case "prompt", "input", "instructions", "system", "content", "text":
		return true
	default:
		return false
	}
}
