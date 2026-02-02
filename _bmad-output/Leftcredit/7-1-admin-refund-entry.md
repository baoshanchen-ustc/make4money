# Story 7.1: 管理端退款入口

Status: done

## Story

**作为** 系统管理员
**我希望** 在管理后台对已支付订单触发退款操作
**以便** 处理用户退款申请

## Acceptance Criteria

- [x] AC1: 管理端订单详情页显示「退款」按钮
- [x] AC2: 仅 `paid` 状态订单可退款
- [x] AC3: 退款前需要输入退款原因
- [x] AC4: 退款操作需要二次确认
- [x] AC5: 退款后按钮变为已退款状态

## Tasks / Subtasks

- [x] Task 1: 后端 - 创建退款 API (AC: 1, 2)
  - [x] 1.1 创建 `backend/internal/handler/admin/recharge_handler.go`
  - [x] 1.2 实现 POST `/api/v1/admin/recharge/orders/:order_no/refund`
  - [x] 1.3 添加请求/响应 DTO

- [x] Task 2: 后端 - 退款权限验证 (AC: 2)
  - [x] 2.1 检查订单状态是否为 `paid`
  - [x] 2.2 验证管理员权限

- [x] Task 3: 前端 - 订单详情页增加退款功能 (AC: 1, 3, 4, 5)
  - [x] 3.1 创建管理端订单详情页
  - [x] 3.2 添加「退款」按钮
  - [x] 3.3 创建退款确认对话框
  - [x] 3.4 退款原因输入框
  - [x] 3.5 显示退款状态

- [x] Task 4: 前端 - API 客户端 (AC: 1)
  - [x] 4.1 在管理端 API 中添加退款方法
  - [x] 4.2 定义 TypeScript 类型

- [x] Task 5: 国际化 (AC: 1-5)
  - [x] 5.1 添加退款相关中文翻译
  - [x] 5.2 添加退款相关英文翻译

## Dev Notes

### 依赖关系

**前置条件**:
- Story 4.1（查询单个订单详情）完成
- 管理端订单列表已存在（或本 Story 一并创建）

**后续依赖**:
- Story 7.2（调用微信退款API）
- Story 7.3（扣减用户余额）
- Story 7.4（退款状态更新与日志）

### 后端实现

#### 1. 退款请求 DTO

在 `backend/internal/handler/dto/recharge.go` 添加：

```go
// AdminRefundOrderRequest 管理员退款请求
type AdminRefundOrderRequest struct {
    Reason string `json:"reason" binding:"required,min=2,max=500"` // 退款原因
}

// AdminRefundOrderResponse 管理员退款响应
type AdminRefundOrderResponse struct {
    OrderNo      string    `json:"order_no"`
    Status       string    `json:"status"`
    RefundStatus string    `json:"refund_status"` // pending/success/failed
    RefundedAt   time.Time `json:"refunded_at,omitempty"`
    Message      string    `json:"message"`
}
```

#### 2. 管理端充值 Handler

创建 `backend/internal/handler/admin/recharge_handler.go`：

```go
package admin

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "your-project/internal/handler/dto"
    "your-project/internal/log"
    "your-project/internal/service"
)

// RechargeHandler 管理端充值处理器
type RechargeHandler struct {
    rechargeService *service.RechargeService
}

// NewRechargeHandler 创建管理端充值处理器
func NewRechargeHandler(rechargeService *service.RechargeService) *RechargeHandler {
    return &RechargeHandler{
        rechargeService: rechargeService,
    }
}

// RefundOrder 退款订单
// POST /api/v1/admin/recharge/orders/:order_no/refund
func (h *RechargeHandler) RefundOrder(c *gin.Context) {
    orderNo := c.Param("order_no")
    if orderNo == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "order_no is required"})
        return
    }

    var req dto.AdminRefundOrderRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
        return
    }

    // 获取当前管理员 ID
    adminID := c.GetInt64("user_id")
    ctx := c.Request.Context()

    // 调用退款服务
    result, err := h.rechargeService.RefundOrder(ctx, service.RefundOrderParams{
        OrderNo:  orderNo,
        Reason:   req.Reason,
        AdminID:  adminID,
    })

    if err != nil {
        log.Error("Refund order failed",
            "order_no", orderNo,
            "admin_id", adminID,
            "error", err)

        switch {
        case errors.Is(err, service.ErrOrderNotFound):
            c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
        case errors.Is(err, service.ErrOrderNotRefundable):
            c.JSON(http.StatusBadRequest, gin.H{"error": "order is not refundable", "message": "只有已支付状态的订单可以退款"})
        case errors.Is(err, service.ErrRefundFailed):
            c.JSON(http.StatusInternalServerError, gin.H{"error": "refund failed", "message": err.Error()})
        default:
            c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        }
        return
    }

    c.JSON(http.StatusOK, &dto.AdminRefundOrderResponse{
        OrderNo:      result.OrderNo,
        Status:       result.Status,
        RefundStatus: result.RefundStatus,
        RefundedAt:   result.RefundedAt,
        Message:      "退款处理中",
    })
}

// GetOrder 获取订单详情
// GET /api/v1/admin/recharge/orders/:order_no
func (h *RechargeHandler) GetOrder(c *gin.Context) {
    orderNo := c.Param("order_no")
    if orderNo == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "order_no is required"})
        return
    }

    ctx := c.Request.Context()
    order, err := h.rechargeService.GetOrderByOrderNo(ctx, orderNo)
    if err != nil {
        if errors.Is(err, service.ErrOrderNotFound) {
            c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }

    c.JSON(http.StatusOK, order)
}

// ListOrders 获取订单列表
// GET /api/v1/admin/recharge/orders
func (h *RechargeHandler) ListOrders(c *gin.Context) {
    // 分页和筛选参数
    page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
    pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
    status := c.Query("status")
    userID, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)

    ctx := c.Request.Context()
    result, err := h.rechargeService.ListOrdersAdmin(ctx, service.ListOrdersAdminParams{
        Page:     page,
        PageSize: pageSize,
        Status:   status,
        UserID:   userID,
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }

    c.JSON(http.StatusOK, result)
}
```

#### 3. 路由注册

在 `backend/internal/server/routes/admin.go` 添加：

```go
func registerRechargeAdminRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
    recharge := admin.Group("/recharge")
    {
        // 订单列表
        recharge.GET("/orders", h.Admin.Recharge.ListOrders)
        // 订单详情
        recharge.GET("/orders/:order_no", h.Admin.Recharge.GetOrder)
        // 退款
        recharge.POST("/orders/:order_no/refund", h.Admin.Recharge.RefundOrder)
    }
}
```

#### 4. Service 层接口定义

在 `backend/internal/service/recharge_service.go` 添加：

```go
// RefundOrderParams 退款参数
type RefundOrderParams struct {
    OrderNo string
    Reason  string
    AdminID int64
}

// RefundOrderResult 退款结果
type RefundOrderResult struct {
    OrderNo      string
    Status       string
    RefundStatus string
    RefundedAt   time.Time
}

// 错误定义
var (
    ErrOrderNotRefundable = errors.New("order is not refundable")
    ErrRefundFailed       = errors.New("refund failed")
)

// RefundOrder 退款订单（骨架，实际逻辑在 Story 7.2-7.4 实现）
func (s *RechargeService) RefundOrder(ctx context.Context, params RefundOrderParams) (*RefundOrderResult, error) {
    // 1. 查询订单
    order, err := s.orderRepo.GetByOrderNo(ctx, params.OrderNo)
    if err != nil {
        if ent.IsNotFound(err) {
            return nil, ErrOrderNotFound
        }
        return nil, fmt.Errorf("query order failed: %w", err)
    }

    // 2. 验证订单状态
    if order.Status != "paid" {
        return nil, ErrOrderNotRefundable
    }

    // 3. 调用微信退款 API（Story 7.2 实现）
    // 4. 扣减用户余额（Story 7.3 实现）
    // 5. 更新订单状态和日志（Story 7.4 实现）

    // 暂时返回待处理状态
    return &RefundOrderResult{
        OrderNo:      params.OrderNo,
        Status:       order.Status,
        RefundStatus: "pending",
    }, nil
}
```

### 前端实现

#### 1. 订单详情页

创建 `frontend/src/views/admin/recharge/OrderDetailView.vue`：

```vue
<template>
  <div class="max-w-4xl mx-auto p-6">
    <!-- 面包屑 -->
    <nav class="mb-6">
      <ol class="flex items-center space-x-2 text-sm text-gray-500">
        <li><router-link to="/admin/recharge/orders">充值订单</router-link></li>
        <li>/</li>
        <li class="text-gray-900 dark:text-white">{{ order?.order_no }}</li>
      </ol>
    </nav>

    <!-- 加载状态 -->
    <div v-if="loading" class="animate-pulse space-y-4">
      <div class="h-8 bg-gray-200 rounded w-1/3"></div>
      <div class="h-4 bg-gray-200 rounded w-1/2"></div>
    </div>

    <!-- 订单详情 -->
    <div v-else-if="order" class="bg-white dark:bg-gray-800 rounded-lg shadow">
      <!-- 头部 -->
      <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
        <div class="flex items-center justify-between">
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            订单详情
          </h1>
          <OrderStatusBadge :status="order.status" />
        </div>
      </div>

      <!-- 订单信息 -->
      <div class="p-6 space-y-6">
        <div class="grid grid-cols-2 gap-4">
          <InfoItem label="订单号" :value="order.order_no" />
          <InfoItem label="用户ID" :value="order.user_id" />
          <InfoItem label="充值金额" :value="`¥${order.amount.toFixed(2)}`" />
          <InfoItem label="支付方式" :value="order.payment_method" />
          <InfoItem label="创建时间" :value="formatDate(order.created_at)" />
          <InfoItem label="支付时间" :value="order.paid_at ? formatDate(order.paid_at) : '-'" />
          <InfoItem label="微信订单号" :value="order.transaction_id || '-'" />
        </div>

        <!-- 退款信息（如果已退款） -->
        <div v-if="order.status === 'refunded'" class="p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
          <h3 class="font-medium text-gray-900 dark:text-white mb-2">退款信息</h3>
          <div class="grid grid-cols-2 gap-4 text-sm">
            <InfoItem label="退款时间" :value="formatDate(order.refunded_at)" />
            <InfoItem label="退款原因" :value="order.refund_reason" />
          </div>
        </div>
      </div>

      <!-- 操作按钮 -->
      <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end">
        <button
          v-if="order.status === 'paid'"
          @click="showRefundDialog = true"
          class="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
        >
          申请退款
        </button>
        <span
          v-else-if="order.status === 'refunded'"
          class="px-4 py-2 bg-gray-100 text-gray-500 rounded-lg cursor-not-allowed"
        >
          已退款
        </span>
      </div>
    </div>

    <!-- 退款确认对话框 -->
    <ConfirmDialog
      v-model="showRefundDialog"
      title="确认退款"
      confirmText="确认退款"
      confirmButtonClass="bg-red-600 hover:bg-red-700"
      :loading="refunding"
      @confirm="handleRefund"
    >
      <div class="space-y-4">
        <p class="text-gray-600 dark:text-gray-400">
          确定要对订单 <strong>{{ order?.order_no }}</strong> 进行退款吗？
        </p>
        <p class="text-sm text-gray-500">
          退款金额：<strong class="text-red-600">¥{{ order?.amount.toFixed(2) }}</strong>
        </p>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            退款原因 <span class="text-red-500">*</span>
          </label>
          <textarea
            v-model="refundReason"
            rows="3"
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 dark:bg-gray-700 dark:text-white"
            placeholder="请输入退款原因..."
          ></textarea>
        </div>
      </div>
    </ConfirmDialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useToast } from '@/composables/useToast'
import { adminAPI } from '@/api'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import OrderStatusBadge from '@/components/admin/recharge/OrderStatusBadge.vue'
import InfoItem from '@/components/common/InfoItem.vue'

const route = useRoute()
const router = useRouter()
const toast = useToast()

const loading = ref(true)
const order = ref<any>(null)
const showRefundDialog = ref(false)
const refunding = ref(false)
const refundReason = ref('')

const orderNo = route.params.orderNo as string

onMounted(async () => {
  try {
    order.value = await adminAPI.recharge.getOrder(orderNo)
  } catch (error) {
    toast.error('加载订单失败')
    router.push('/admin/recharge/orders')
  } finally {
    loading.value = false
  }
})

async function handleRefund() {
  if (!refundReason.value.trim()) {
    toast.warning('请输入退款原因')
    return
  }

  refunding.value = true
  try {
    await adminAPI.recharge.refundOrder(orderNo, { reason: refundReason.value })
    toast.success('退款申请已提交')
    showRefundDialog.value = false
    // 刷新订单详情
    order.value = await adminAPI.recharge.getOrder(orderNo)
  } catch (error: any) {
    toast.error(error.response?.data?.message || '退款失败')
  } finally {
    refunding.value = false
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleString('zh-CN')
}
</script>
```

#### 2. 管理端 API 客户端

在 `frontend/src/api/admin/recharge.ts` 添加：

```typescript
import { apiClient } from '../client'

export interface RechargeOrder {
  id: number
  order_no: string
  user_id: number
  amount: number
  status: 'pending' | 'paid' | 'failed' | 'expired' | 'refunded'
  payment_method: string
  payment_channel: string
  transaction_id?: string
  created_at: string
  paid_at?: string
  refunded_at?: string
  refund_reason?: string
}

export interface RefundOrderRequest {
  reason: string
}

export interface RefundOrderResponse {
  order_no: string
  status: string
  refund_status: string
  refunded_at?: string
  message: string
}

// 获取订单详情
export async function getOrder(orderNo: string): Promise<RechargeOrder> {
  const { data } = await apiClient.get<RechargeOrder>(`/admin/recharge/orders/${orderNo}`)
  return data
}

// 获取订单列表
export async function listOrders(params?: {
  page?: number
  page_size?: number
  status?: string
  user_id?: number
}) {
  const { data } = await apiClient.get('/admin/recharge/orders', { params })
  return data
}

// 退款订单
export async function refundOrder(
  orderNo: string,
  request: RefundOrderRequest
): Promise<RefundOrderResponse> {
  const { data } = await apiClient.post<RefundOrderResponse>(
    `/admin/recharge/orders/${orderNo}/refund`,
    request
  )
  return data
}
```

#### 3. 国际化文本

**中文** (`frontend/src/locales/zh-CN.json`):
```json
{
  "admin": {
    "recharge": {
      "orderDetail": "订单详情",
      "refund": "申请退款",
      "refunded": "已退款",
      "refundConfirm": {
        "title": "确认退款",
        "message": "确定要对订单 {orderNo} 进行退款吗？",
        "amount": "退款金额",
        "reason": "退款原因",
        "reasonPlaceholder": "请输入退款原因...",
        "reasonRequired": "请输入退款原因",
        "confirm": "确认退款",
        "cancel": "取消"
      },
      "refundSuccess": "退款申请已提交",
      "refundFailed": "退款失败"
    }
  }
}
```

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `backend/internal/handler/admin/recharge_handler.go` | 管理端充值 Handler |
| `backend/internal/handler/dto/recharge.go` | 添加退款 DTO |
| `backend/internal/server/routes/admin.go` | 注册管理端路由 |
| `backend/internal/service/recharge_service.go` | 添加 RefundOrder 方法骨架 |
| `frontend/src/views/admin/recharge/OrderDetailView.vue` | 订单详情页 |
| `frontend/src/api/admin/recharge.ts` | 管理端 API 客户端 |
| `frontend/src/locales/zh-CN.json` | 中文翻译 |

### 路由配置

在前端路由中添加：

```typescript
// frontend/src/router/admin.ts
{
  path: '/admin/recharge/orders/:orderNo',
  name: 'admin-recharge-order-detail',
  component: () => import('@/views/admin/recharge/OrderDetailView.vue'),
  meta: { requiresAuth: true, requiresAdmin: true }
}
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-7.1] - 用户故事定义
- [Source: frontend/src/components/common/ConfirmDialog.vue] - 确认对话框组件
- [Source: backend/internal/handler/admin/] - 现有管理端 Handler 模式

### 安全注意事项

1. **权限验证**: 只有管理员可以执行退款操作
2. **审计日志**: 记录退款操作人和原因
3. **二次确认**: 前端必须有确认对话框，防止误操作

## Dev Agent Record

### Agent Model Used

(待开发时填写)

### Debug Log References

(待开发时填写)

### Completion Notes List

(待开发时填写)

### File List

(待开发时填写)
