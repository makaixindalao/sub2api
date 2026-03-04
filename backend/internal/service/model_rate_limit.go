package service

import (
	"context"
	"strings"
	"time"
)

const modelRateLimitsKey = "model_rate_limits"

// isRateLimitActiveForKey 检查指定 key 的限流是否生效
func (a *Account) isRateLimitActiveForKey(key string) bool {
	resetAt := a.modelRateLimitResetAt(key)
	return resetAt != nil && time.Now().Before(*resetAt)
}

// getRateLimitRemainingForKey 获取指定 key 的限流剩余时间，0 表示未限流或已过期
func (a *Account) getRateLimitRemainingForKey(key string) time.Duration {
	resetAt := a.modelRateLimitResetAt(key)
	if resetAt == nil {
		return 0
	}
	remaining := time.Until(*resetAt)
	if remaining > 0 {
		return remaining
	}
	return 0
}

func (a *Account) isModelRateLimitedWithContext(ctx context.Context, requestedModel string) bool {
	if a == nil {
		return false
	}

	// Gemini 平台：非 Code Assist 账号使用 gemini_flash/gemini_pro 分级限流
	// 必须基于映射后的上游模型名判定 tier，否则别名（如 claude-haiku → gemini-2.0-flash）
	// 会因不含 flash/lite 而被误判为 pro，导致分级限流失效
	// 作者: mkx | 日期: 2026-03-04
	if a.Platform == PlatformGemini && isGeminiPerModelQuotaAccount(a) {
		mappedModel := a.GetMappedModel(requestedModel)
		modelClass := geminiModelClassFromName(mappedModel)
		scope := "gemini_" + string(modelClass)
		return a.isRateLimitActiveForKey(scope)
	}

	modelKey := a.GetMappedModel(requestedModel)
	if a.Platform == PlatformAntigravity {
		modelKey = resolveFinalAntigravityModelKey(ctx, a, requestedModel)
	}
	modelKey = strings.TrimSpace(modelKey)
	if modelKey == "" {
		return false
	}
	return a.isRateLimitActiveForKey(modelKey)
}

// GetModelRateLimitRemainingTime 获取模型限流剩余时间
// 返回 0 表示未限流或已过期
func (a *Account) GetModelRateLimitRemainingTime(requestedModel string) time.Duration {
	return a.GetModelRateLimitRemainingTimeWithContext(context.Background(), requestedModel)
}

func (a *Account) GetModelRateLimitRemainingTimeWithContext(ctx context.Context, requestedModel string) time.Duration {
	if a == nil {
		return 0
	}

	// Gemini 平台：非 Code Assist 账号使用 gemini_flash/gemini_pro 分级限流
	// 必须基于映射后的上游模型名判定 tier，否则别名（如 claude-haiku → gemini-2.0-flash）
	// 会因不含 flash/lite 而被误判为 pro，导致分级限流失效
	// 作者: mkx | 日期: 2026-03-04
	if a.Platform == PlatformGemini && isGeminiPerModelQuotaAccount(a) {
		mappedModel := a.GetMappedModel(requestedModel)
		modelClass := geminiModelClassFromName(mappedModel)
		scope := "gemini_" + string(modelClass)
		return a.getRateLimitRemainingForKey(scope)
	}

	modelKey := a.GetMappedModel(requestedModel)
	if a.Platform == PlatformAntigravity {
		modelKey = resolveFinalAntigravityModelKey(ctx, a, requestedModel)
	}
	modelKey = strings.TrimSpace(modelKey)
	if modelKey == "" {
		return 0
	}
	return a.getRateLimitRemainingForKey(modelKey)
}

func resolveFinalAntigravityModelKey(ctx context.Context, account *Account, requestedModel string) string {
	modelKey := mapAntigravityModel(account, requestedModel)
	if modelKey == "" {
		return ""
	}
	// thinking 会影响 Antigravity 最终模型名（例如 claude-sonnet-4-5 -> claude-sonnet-4-5-thinking）
	if enabled, ok := ThinkingEnabledFromContext(ctx); ok {
		modelKey = applyThinkingModelSuffix(modelKey, enabled)
	}
	return modelKey
}

func (a *Account) modelRateLimitResetAt(scope string) *time.Time {
	if a == nil || a.Extra == nil || scope == "" {
		return nil
	}
	rawLimits, ok := a.Extra[modelRateLimitsKey].(map[string]any)
	if !ok {
		return nil
	}
	rawLimit, ok := rawLimits[scope].(map[string]any)
	if !ok {
		return nil
	}
	resetAtRaw, ok := rawLimit["rate_limit_reset_at"].(string)
	if !ok || strings.TrimSpace(resetAtRaw) == "" {
		return nil
	}
	resetAt, err := time.Parse(time.RFC3339, resetAtRaw)
	if err != nil {
		return nil
	}
	return &resetAt
}
