// Package service 的 SleepBudget 类型，统一追踪跨层级的累计 sleep 时间。
// 作者: mkx
// 变更: 新增 SleepBudget，用于限制 Gemini 重试/failover 的总 sleep 时间
// 日期: 2026-03-03
package service

import (
	"context"
	"sync"
	"time"
)

// SleepBudget 跟踪剩余可用的 sleep 预算。
// 跨 Service 层（per-account 重试退避）和 Handler 层（同账号重试、Antigravity 延时）共享，
// 确保整个请求链路的累计 sleep 时间不超过预设上限。
type SleepBudget struct {
	mu        sync.Mutex
	remaining time.Duration
}

// NewSleepBudget 创建指定总预算的 SleepBudget。
func NewSleepBudget(total time.Duration) *SleepBudget {
	return &SleepBudget{remaining: total}
}

// TrySleep 尝试消耗 d 时长的 sleep 预算。
// 如果剩余预算不足或 ctx 已取消，返回 false；否则实际 sleep 并返回 true。
func (b *SleepBudget) TrySleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}

	b.mu.Lock()
	if b.remaining < d {
		b.mu.Unlock()
		return false
	}
	b.remaining -= d
	b.mu.Unlock()

	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// Remaining 返回剩余 sleep 预算。
func (b *SleepBudget) Remaining() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.remaining
}
