package service

import (
	"context"
	"time"
)

// 余额变动类型常量
const (
	BalanceChangeTypeRecharge = "recharge" // 充值
	BalanceChangeTypeConsume  = "consume"  // 消费
	BalanceChangeTypeRefund   = "refund"   // 退款
	BalanceChangeTypeAdjust   = "adjust"   // 调整
)

// 操作人类型常量
const (
	OperatorTypeSystem = "system" // 系统
	OperatorTypeAdmin  = "admin"  // 管理员
	OperatorTypeUser   = "user"   // 用户
)

// BalanceLog 余额变动日志模型
type BalanceLog struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	ChangeType     string    `json:"change_type"`
	Amount         float64   `json:"amount"`
	BalanceBefore  float64   `json:"balance_before"`
	BalanceAfter   float64   `json:"balance_after"`
	RelatedOrderNo *string   `json:"related_order_no,omitempty"`
	Description    string    `json:"description"`
	OperatorID     int64     `json:"operator_id"`
	OperatorType   string    `json:"operator_type"`
	CreatedAt      time.Time `json:"created_at"`
}

// BalanceLogRepository 余额日志仓储接口
// 注意：该表只允许插入，不允许修改和删除（应用层控制）
type BalanceLogRepository interface {
	// Create 创建余额变动日志
	Create(ctx context.Context, log *BalanceLog) error
	// GetByOrderNo 根据订单号查询余额日志
	GetByOrderNo(ctx context.Context, orderNo string) ([]*BalanceLog, error)
}
