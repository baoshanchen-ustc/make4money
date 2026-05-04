package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// UserAccountBindingCleanupService 定期清理已过期的 user→account 长期绑定（P0-2）。
// 当 LongTermBindingTTLDays > 0 时绑定会被持久化到 user_account_bindings 表，
// 但 ResolveLongTermBinding 只会在 lazy 路径上删除当下命中的过期记录；
// 没有访问的过期记录会无限堆积，需要定期清理。
type UserAccountBindingCleanupService struct {
	repo     UserAccountBindingRepository
	interval time.Duration

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
}

// NewUserAccountBindingCleanupService creates the service with sensible defaults.
// 间隔默认 1 小时；可通过 gateway.ltb_cleanup_interval_seconds 覆盖。
func NewUserAccountBindingCleanupService(repo UserAccountBindingRepository, cfg *config.Config) *UserAccountBindingCleanupService {
	interval := time.Hour
	if cfg != nil && cfg.Gateway.LongTermBindingCleanupIntervalSeconds > 0 {
		interval = time.Duration(cfg.Gateway.LongTermBindingCleanupIntervalSeconds) * time.Second
	}
	return &UserAccountBindingCleanupService{
		repo:     repo,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动后台 ticker，每 interval 触发一次过期清理。
// 可重复调用，仅首次生效。
func (s *UserAccountBindingCleanupService) Start() {
	if s == nil || s.repo == nil {
		return
	}
	s.startOnce.Do(func() {
		logger.LegacyPrintf("service.user_account_binding_cleanup", "[UserAccountBindingCleanup] started interval=%s", s.interval)
		go s.runLoop()
	})
}

// Stop 通知后台 goroutine 退出。可重复调用，仅首次生效。
func (s *UserAccountBindingCleanupService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
		logger.LegacyPrintf("service.user_account_binding_cleanup", "[UserAccountBindingCleanup] stopped")
	})
}

func (s *UserAccountBindingCleanupService) runLoop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// 启动后先清理一轮，避免重启后积压。
	s.cleanupOnce()

	for {
		select {
		case <-ticker.C:
			s.cleanupOnce()
		case <-s.stopCh:
			return
		}
	}
}

func (s *UserAccountBindingCleanupService) cleanupOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	deleted, err := s.repo.DeleteExpired(ctx)
	if err != nil {
		logger.LegacyPrintf("service.user_account_binding_cleanup", "[UserAccountBindingCleanup] cleanup failed err=%v", err)
		return
	}
	if deleted > 0 {
		logger.LegacyPrintf("service.user_account_binding_cleanup", "[UserAccountBindingCleanup] cleaned expired bindings count=%d", deleted)
	}
}
