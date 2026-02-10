package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// BalanceLotExpiryScheduler 余额批次过期处理定时任务
type BalanceLotExpiryScheduler struct {
	balanceLotService *BalanceLotService
	redisClient       *redis.Client
	stopCh            chan struct{}
	stopOnce          sync.Once
	wg                sync.WaitGroup
}

// NewBalanceLotExpiryScheduler 创建余额批次过期调度器
func NewBalanceLotExpiryScheduler(
	balanceLotService *BalanceLotService,
	redisClient *redis.Client,
) *BalanceLotExpiryScheduler {
	return &BalanceLotExpiryScheduler{
		balanceLotService: balanceLotService,
		redisClient:       redisClient,
		stopCh:            make(chan struct{}),
	}
}

// Start 启动调度器
func (s *BalanceLotExpiryScheduler) Start() {
	s.wg.Add(1)
	go s.run()
	log.Printf("[BalanceLotExpiry] Started")
}

// Stop 停止调度器
func (s *BalanceLotExpiryScheduler) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
	log.Printf("[BalanceLotExpiry] Stopped")
}

func (s *BalanceLotExpiryScheduler) run() {
	defer s.wg.Done()

	// 延迟 3 分钟首次执行
	select {
	case <-time.After(3 * time.Minute):
	case <-s.stopCh:
		return
	}

	// 首次执行时检查并迁移历史余额
	s.maybeMigrateLegacyBalances()

	// 首次执行过期处理
	s.processExpiredLots()

	// 每小时执行一次
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.processExpiredLots()
		}
	}
}

func (s *BalanceLotExpiryScheduler) processExpiredLots() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	count, err := s.balanceLotService.ExpireLots(ctx)
	if err != nil {
		log.Printf("[BalanceLotExpiry] Error processing expired lots: %v", err)
		return
	}

	if count > 0 {
		log.Printf("[BalanceLotExpiry] Processed %d expired lots", count)
	}
}

// maybeMigrateLegacyBalances 迁移历史余额
// 为所有有余额但没有余额批次记录的用户创建迁移批次
func (s *BalanceLotExpiryScheduler) maybeMigrateLegacyBalances() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Redis SetNX 防止重复迁移
	migrationKey := "balance_lots_migration_done"
	migrationRetryTTL := 6 * time.Hour
	ok, err := s.redisClient.SetNX(ctx, migrationKey, "1", 0).Result()
	if err != nil {
		log.Printf("[BalanceLotExpiry] Failed to check migration lock: %v", err)
		return
	}
	if !ok {
		// 已经迁移过
		return
	}

	log.Printf("[BalanceLotExpiry] Starting legacy balance migration...")

	migrationFailed := false

	// 获取过期天数配置
	days := s.balanceLotService.GetExpiryDays(ctx)
	totalMigrated := 0

	// 使用原生 SQL 查询有余额但无批次的用户
	query := `
		SELECT u.id, u.balance
		FROM users u
		WHERE u.balance > 0
		AND u.deleted_at IS NULL
		AND NOT EXISTS (
			SELECT 1 FROM balance_lots bl WHERE bl.user_id = u.id
		)
	`

	rows, err := s.balanceLotService.entClient.QueryContext(ctx, query)
	if err != nil {
		log.Printf("[BalanceLotExpiry] Failed to query users for migration: %v", err)
		s.setMigrationRetryMarker(ctx, migrationKey, migrationRetryTTL)
		return
	}
	defer rows.Close()

	expiresAt := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	sourceRef := "initial_balance_migration"

	for rows.Next() {
		var userID int64
		var balance float64
		if err := rows.Scan(&userID, &balance); err != nil {
			log.Printf("[BalanceLotExpiry] Failed to scan user row: %v", err)
			migrationFailed = true
			continue
		}

		lot := &BalanceLot{
			UserID:          userID,
			SourceType:      BalanceLotSourceMigration,
			SourceRef:       &sourceRef,
			OriginalAmount:  balance,
			RemainingAmount: balance,
			Status:          BalanceLotStatusActive,
			ExpiresAt:       expiresAt,
			Description:     "历史余额迁移",
		}

		if err := s.balanceLotService.lotRepo.Create(ctx, lot); err != nil {
			log.Printf("[BalanceLotExpiry] Failed to create migration lot for user %d: %v", userID, err)
			migrationFailed = true
			continue
		}

		totalMigrated++
	}

	if err := rows.Err(); err != nil {
		log.Printf("[BalanceLotExpiry] Row iteration error: %v", err)
		migrationFailed = true
	}

	if migrationFailed {
		log.Printf("[BalanceLotExpiry] Legacy balance migration completed with partial failures: %d users migrated, retry in up to %s", totalMigrated, migrationRetryTTL)
		s.setMigrationRetryMarker(ctx, migrationKey, migrationRetryTTL)
		return
	}

	// 迁移成功后保留完成标记，并设置较长 TTL 以降低误触发概率
	if err := s.redisClient.Expire(ctx, migrationKey, 365*24*time.Hour).Err(); err != nil {
		log.Printf("[BalanceLotExpiry] Failed to set migration marker TTL: %v", err)
	}

	log.Printf("[BalanceLotExpiry] Legacy balance migration completed: %d users migrated", totalMigrated)
}

func (s *BalanceLotExpiryScheduler) setMigrationRetryMarker(ctx context.Context, migrationKey string, retryTTL time.Duration) {
	if err := s.redisClient.Set(ctx, migrationKey, "1", retryTTL).Err(); err != nil {
		log.Printf("[BalanceLotExpiry] Failed to set retry marker TTL: %v", err)
		// 回退：清空标记，确保下次可重试
		s.redisClient.Del(ctx, migrationKey)
	}
}
