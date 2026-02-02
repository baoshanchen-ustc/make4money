# Story 1.1: 用户分销信息初始化

Status: ready-for-dev

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a 系统,
I want 在用户注册时自动创建分销信息并生成唯一邀请码,
so that 每个用户都拥有专属的推广身份。

## Acceptance Criteria

1. **AC1**: 新用户完成注册流程后，系统自动在 `user_affiliate` 表中创建对应记录
2. **AC2**: 生成 6-8 位唯一邀请码（字母数字组合，排除易混淆字符 0/O/1/I/l）
3. **AC3**: 邀请码在数据库中有唯一索引约束，冲突时自动重新生成
4. **AC4**: 初始化字段值：tier_level=1, effective_count=0, total_earnings=0, withdrawable=0, is_kol=false
5. **AC5**: 创建分销记录的操作在同一事务中完成，确保与用户创建的原子性

## Tasks / Subtasks

- [ ] Task 1: 创建 Ent Schema 和数据库迁移 (AC: #1, #3)
  - [ ] 1.1 创建 `user_affiliate` Ent schema 文件
  - [ ] 1.2 创建 SQL 迁移文件，定义表结构和索引
  - [ ] 1.3 运行 `go generate ./ent` 生成 Ent 代码
- [ ] Task 2: 实现邀请码生成算法 (AC: #2, #3)
  - [ ] 2.1 在 `internal/service/affiliate_service.go` 中实现 `GenerateReferralCode()` 函数
  - [ ] 2.2 排除易混淆字符（0, O, 1, I, l）
  - [ ] 2.3 实现冲突检测和重试逻辑（最多 5 次重试）
- [ ] Task 3: 实现 Repository 层 (AC: #1, #5)
  - [ ] 3.1 创建 `internal/repository/affiliate_repo.go`
  - [ ] 3.2 实现 `Create()` 方法，支持事务
  - [ ] 3.3 实现 `GetByUserID()` 方法
  - [ ] 3.4 实现 `ExistsByCode()` 方法（用于邀请码冲突检测）
- [ ] Task 4: 实现 Service 层 (AC: #4, #5)
  - [ ] 4.1 创建 `internal/service/affiliate_service.go`
  - [ ] 4.2 实现 `CreateUserAffiliate()` 方法
  - [ ] 4.3 定义 `AffiliateRepository` 接口
- [ ] Task 5: 集成到用户注册流程 (AC: #5)
  - [ ] 5.1 修改 `auth_service.go` 的 `Register()` 方法
  - [ ] 5.2 在用户创建成功后调用 `AffiliateService.CreateUserAffiliate()`
  - [ ] 5.3 确保在同一事务中执行

## Dev Notes

### 技术栈要求

- **ORM**: Ent (entgo.io/ent)
- **数据库**: PostgreSQL
- **迁移策略**: SQL 文件版本管理（非自动迁移）

### 数据库 Schema

```sql
-- 迁移文件: migrations/XXX_create_user_affiliate.sql

CREATE TABLE user_affiliate (
    user_id         BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    referral_code   VARCHAR(16) NOT NULL,
    tier_level      SMALLINT NOT NULL DEFAULT 1,
    effective_count INT NOT NULL DEFAULT 0,
    is_kol          BOOLEAN NOT NULL DEFAULT FALSE,
    kol_config      JSONB,
    total_earnings  DECIMAL(12,2) NOT NULL DEFAULT 0,
    withdrawable    DECIMAL(12,2) NOT NULL DEFAULT 0,
    version         INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 唯一索引：确保邀请码全局唯一
CREATE UNIQUE INDEX idx_user_affiliate_referral_code ON user_affiliate(referral_code);

-- 索引：支持按邀请码快速查找
CREATE INDEX idx_user_affiliate_tier_level ON user_affiliate(tier_level);
CREATE INDEX idx_user_affiliate_is_kol ON user_affiliate(is_kol) WHERE is_kol = TRUE;
```

### Ent Schema 定义

```go
// backend/ent/schema/user_affiliate.go
package schema

import (
    "github.com/Wei-Shaw/sub2api/ent/schema/mixins"
    "entgo.io/ent"
    "entgo.io/ent/dialect"
    "entgo.io/ent/dialect/entsql"
    "entgo.io/ent/schema"
    "entgo.io/ent/schema/edge"
    "entgo.io/ent/schema/field"
    "entgo.io/ent/schema/index"
)

type UserAffiliate struct {
    ent.Schema
}

func (UserAffiliate) Annotations() []schema.Annotation {
    return []schema.Annotation{
        entsql.Annotation{Table: "user_affiliate"},
    }
}

func (UserAffiliate) Mixin() []ent.Mixin {
    return []ent.Mixin{
        mixins.TimeMixin{},
    }
}

func (UserAffiliate) Fields() []ent.Field {
    return []ent.Field{
        field.Int64("user_id").
            Unique(),
        field.String("referral_code").
            MaxLen(16).
            NotEmpty(),
        field.Int("tier_level").
            Default(1),
        field.Int("effective_count").
            Default(0),
        field.Bool("is_kol").
            Default(false),
        field.JSON("kol_config", map[string]interface{}{}).
            Optional(),
        field.Float("total_earnings").
            SchemaType(map[string]string{dialect.Postgres: "decimal(12,2)"}).
            Default(0),
        field.Float("withdrawable").
            SchemaType(map[string]string{dialect.Postgres: "decimal(12,2)"}).
            Default(0),
        field.Int("version").
            Default(0),
    }
}

func (UserAffiliate) Edges() []ent.Edge {
    return []ent.Edge{
        edge.From("user", User.Type).
            Ref("affiliate").
            Field("user_id").
            Unique().
            Required(),
    }
}

func (UserAffiliate) Indexes() []ent.Index {
    return []ent.Index{
        index.Fields("referral_code").Unique(),
        index.Fields("tier_level"),
    }
}
```

### 邀请码生成算法

```go
// 字符集：排除易混淆字符 (0, O, 1, I, l)
const referralCodeCharset = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

func GenerateReferralCode(length int) string {
    // 使用 crypto/rand 生成安全随机数
    b := make([]byte, length)
    _, err := rand.Read(b)
    if err != nil {
        // fallback to time-based seed
        r := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
        for i := range b {
            b[i] = referralCodeCharset[r.Intn(len(referralCodeCharset))]
        }
        return string(b)
    }
    for i := range b {
        b[i] = referralCodeCharset[int(b[i])%len(referralCodeCharset)]
    }
    return string(b)
}
```

### 错误定义

```go
// internal/service/affiliate_errors.go
var (
    ErrAffiliateNotFound     = infraerrors.NotFound("AFFILIATE_NOT_FOUND", "affiliate info not found")
    ErrAffiliateAlreadyExists = infraerrors.Conflict("AFFILIATE_ALREADY_EXISTS", "affiliate info already exists for this user")
    ErrReferralCodeConflict  = infraerrors.InternalServer("REFERRAL_CODE_CONFLICT", "failed to generate unique referral code")
)
```

### 服务接口定义

```go
// internal/service/affiliate_service.go

type AffiliateRepository interface {
    Create(ctx context.Context, affiliate *UserAffiliate) error
    GetByUserID(ctx context.Context, userID int64) (*UserAffiliate, error)
    ExistsByCode(ctx context.Context, code string) (bool, error)
}

type AffiliateService struct {
    affiliateRepo AffiliateRepository
}

func NewAffiliateService(repo AffiliateRepository) *AffiliateService {
    return &AffiliateService{affiliateRepo: repo}
}

// CreateUserAffiliate 创建用户分销信息
// 应在用户注册成功后的同一事务中调用
func (s *AffiliateService) CreateUserAffiliate(ctx context.Context, userID int64) (*UserAffiliate, error) {
    // 1. 生成唯一邀请码（最多重试 5 次）
    var code string
    for i := 0; i < 5; i++ {
        code = GenerateReferralCode(8) // 使用 8 位
        exists, err := s.affiliateRepo.ExistsByCode(ctx, code)
        if err != nil {
            return nil, err
        }
        if !exists {
            break
        }
        if i == 4 {
            return nil, ErrReferralCodeConflict
        }
    }

    // 2. 创建分销记录
    affiliate := &UserAffiliate{
        UserID:         userID,
        ReferralCode:   code,
        TierLevel:      1,
        EffectiveCount: 0,
        IsKol:          false,
        TotalEarnings:  0,
        Withdrawable:   0,
        Version:        0,
    }

    if err := s.affiliateRepo.Create(ctx, affiliate); err != nil {
        return nil, err
    }

    return affiliate, nil
}
```

### 集成到注册流程

```go
// internal/service/auth_service.go
// 修改 Register 方法

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*User, error) {
    // ... 现有的用户创建逻辑 ...

    // 在事务中创建用户后，立即创建分销信息
    tx, err := s.userRepo.BeginTx(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    user, err := s.userRepo.Create(ctx, userInput)
    if err != nil {
        return nil, err
    }

    // 创建分销信息
    _, err = s.affiliateService.CreateUserAffiliate(ctx, user.ID)
    if err != nil {
        return nil, err
    }

    if err := tx.Commit(); err != nil {
        return nil, err
    }

    return user, nil
}
```

### Project Structure Notes

**文件位置遵循现有项目结构：**

```
backend/
├── ent/schema/
│   └── user_affiliate.go          # Ent Schema 定义
├── migrations/
│   └── XXX_create_user_affiliate.sql  # 数据库迁移
├── internal/
│   ├── repository/
│   │   └── affiliate_repo.go      # Repository 实现
│   └── service/
│       ├── affiliate_service.go   # Service 实现
│       └── affiliate_errors.go    # 错误定义
```

**命名约定遵循项目现有模式：**

- Schema 文件：`user_affiliate.go`（snake_case 表名）
- Repository：`affiliate_repo.go`，类型 `AffiliateRepository`
- Service：`affiliate_service.go`，类型 `AffiliateService`
- 错误变量：`Err` 前缀，如 `ErrAffiliateNotFound`

### References

- [Source: _bmad-output/affiliate/TDD-affiliate-system.md#数据库设计] - user_affiliate 表完整 Schema
- [Source: backend/ent/schema/user.go] - 现有 User Schema 模式参考
- [Source: backend/ent/schema/mixins/time.go] - TimeMixin 使用方式
- [Source: backend/internal/service/auth_service.go] - 用户注册流程
- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.1] - Story 需求定义

### Testing Requirements

1. **单元测试**
   - `affiliate_service_test.go`: 测试邀请码生成算法
   - Mock Repository 测试 Service 逻辑

2. **集成测试**
   - 测试完整的用户注册 + 分销信息创建流程
   - 测试邀请码唯一性约束
   - 测试事务回滚场景

### 注意事项

1. **邀请码生成**：使用 `crypto/rand` 而非 `math/rand` 以确保安全性
2. **事务处理**：分销信息创建必须在用户创建的同一事务中
3. **幂等性**：如果用户已有分销信息，不应重复创建
4. **性能**：邀请码查询应使用索引，避免全表扫描

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

### Completion Notes List

### File List
