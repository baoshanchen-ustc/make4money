package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// ChannelPlatformForAccount returns the isolated channel namespace for the selected account.
func ChannelPlatformForAccount(account *Account) string {
	if account == nil {
		return ""
	}
	if account.Platform == PlatformGemini && account.Type == AccountTypeVertex {
		return PlatformVertex
	}
	return account.Platform
}

func WithChannelPlatformOverride(ctx context.Context, platform string) context.Context {
	if ctx == nil {
		return nil
	}
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.ChannelPlatformOverride, platform)
}

func ChannelPlatformOverrideFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(ctxkey.ChannelPlatformOverride).(string)
	return strings.ToLower(strings.TrimSpace(value))
}

func WithSkipChannelPricingRestrictionPrecheck(ctx context.Context, skip bool) context.Context {
	if ctx == nil {
		return nil
	}
	return context.WithValue(ctx, ctxkey.SkipChannelPricingRestrictionPrecheck, skip)
}

func SkipChannelPricingRestrictionPrecheckFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	value, _ := ctx.Value(ctxkey.SkipChannelPricingRestrictionPrecheck).(bool)
	return value
}
