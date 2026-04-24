package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var cardsIssueTemplatePattern = regexp.MustCompile(`\{([^{}]+)\}`)

type CardsIssueRequest struct {
	BuyerID       string         `json:"buyer_id"`
	BuyerName     string         `json:"buyer_name"`
	OrderID       string         `json:"order_id"`
	OrderAmount   float64        `json:"order_amount"`
	OrderQuantity int            `json:"order_quantity"`
	Extra         map[string]any `json:"-"`
}

type CardsIssueResult struct {
	Created        bool    `json:"created"`
	UserID         int64   `json:"user_id"`
	LoginEmail     string  `json:"login_email,omitempty"`
	Username       string  `json:"username,omitempty"`
	Password       string  `json:"password,omitempty"`
	RechargeAmount float64 `json:"recharge_amount"`
	BalanceAfter   float64 `json:"balance_after"`
	OrderID        string  `json:"order_id"`
	Card           string  `json:"card"`
}

type cardsIssueAdminService interface {
	CreateUser(ctx context.Context, input *CreateUserInput) (*User, error)
	UpdateUserBalance(ctx context.Context, userID int64, balance float64, operation string, notes string) (*User, error)
}

type cardsIssueUserLookup interface {
	GetByEmail(ctx context.Context, email string) (*User, error)
}

type CardsIssueBindingRepository interface {
	FindUserIDByBuyerID(ctx context.Context, buyerID string) (int64, bool, error)
	BindBuyerID(ctx context.Context, userID int64, buyerID string) error
}

type CardsIssueService struct {
	adminService      cardsIssueAdminService
	userLookup        cardsIssueUserLookup
	bindingRepository CardsIssueBindingRepository
	randomPassword    func() (string, error)
}

func NewCardsIssueService(
	adminService cardsIssueAdminService,
	userLookup cardsIssueUserLookup,
	bindingRepository CardsIssueBindingRepository,
) *CardsIssueService {
	return &CardsIssueService{
		adminService:      adminService,
		userLookup:        userLookup,
		bindingRepository: bindingRepository,
		randomPassword:    generateCardsIssuePassword,
	}
}

func (s *CardsIssueService) IssueOrder(
	ctx context.Context,
	req CardsIssueRequest,
	cfg CardsIssueRuntimeConfig,
) (*CardsIssueResult, error) {
	if s == nil || s.adminService == nil || s.userLookup == nil || s.bindingRepository == nil {
		return nil, infraerrors.InternalServer("CARDS_ISSUE_UNAVAILABLE", "cards issue service is unavailable")
	}

	buyerID := strings.TrimSpace(req.BuyerID)
	orderID := strings.TrimSpace(req.OrderID)
	buyerName := strings.TrimSpace(req.BuyerName)
	if buyerID == "" {
		return nil, infraerrors.BadRequest("CARDS_ISSUE_BUYER_ID_REQUIRED", "buyer_id is required")
	}
	if orderID == "" {
		return nil, infraerrors.BadRequest("CARDS_ISSUE_ORDER_ID_REQUIRED", "order_id is required")
	}
	if math.IsNaN(req.OrderAmount) || math.IsInf(req.OrderAmount, 0) || req.OrderAmount <= 0 {
		return nil, infraerrors.BadRequest("CARDS_ISSUE_ORDER_AMOUNT_INVALID", "order_amount must be a finite positive number")
	}
	if req.OrderAmount > CardsIssueMaxOrderAmount {
		return nil, infraerrors.BadRequest("CARDS_ISSUE_ORDER_AMOUNT_TOO_LARGE", fmt.Sprintf("order_amount must not exceed %s", formatCardsIssueFloat(CardsIssueMaxOrderAmount)))
	}
	if req.OrderQuantity <= 0 {
		return nil, infraerrors.BadRequest("CARDS_ISSUE_ORDER_QUANTITY_INVALID", "order_quantity must be greater than 0")
	}
	if req.OrderQuantity > CardsIssueMaxOrderQuantity {
		return nil, infraerrors.BadRequest("CARDS_ISSUE_ORDER_QUANTITY_TOO_LARGE", fmt.Sprintf("order_quantity must not exceed %d", CardsIssueMaxOrderQuantity))
	}

	rechargeAmount := req.OrderAmount * float64(req.OrderQuantity)
	if math.IsNaN(rechargeAmount) || math.IsInf(rechargeAmount, 0) || rechargeAmount <= 0 {
		return nil, infraerrors.BadRequest("CARDS_ISSUE_ORDER_AMOUNT_INVALID", "calculated recharge amount is invalid")
	}
	if rechargeAmount > CardsIssueMaxRechargeAmount {
		return nil, infraerrors.BadRequest("CARDS_ISSUE_RECHARGE_AMOUNT_TOO_LARGE", fmt.Sprintf("recharge amount must not exceed %s", formatCardsIssueFloat(CardsIssueMaxRechargeAmount)))
	}
	var (
		user       *User
		created    bool
		password   string
		loginEmail string
	)

	if userID, ok, err := s.bindingRepository.FindUserIDByBuyerID(ctx, buyerID); err != nil {
		return nil, err
	} else if ok {
		user = &User{ID: userID}
	} else {
		generatedPassword, genErr := s.randomPassword()
		if genErr != nil {
			return nil, infraerrors.InternalServer("CARDS_ISSUE_PASSWORD_GENERATE_FAILED", "failed to generate password").WithCause(genErr)
		}
		password = generatedPassword
		loginEmail = buildCardsIssueLoginEmail(buyerID)
		createdUser, createErr := s.adminService.CreateUser(ctx, &CreateUserInput{
			Email:                    loginEmail,
			Password:                 password,
			Username:                 buildCardsIssueUsername(buyerName, buyerID),
			Notes:                    buildCardsIssueUserNotes(req),
			SignupSource:             CardsIssueSignupSource,
			SkipDefaultSubscriptions: true,
		})
		if createErr == nil {
			created = true
			user = createdUser
		} else if errors.Is(createErr, ErrEmailExists) {
			// Concurrent creation won the race or an unrelated user already owns the
			// deterministic email. Prefer an existing buyer_id binding, then fall back
			// to email lookup. Either way the new password is discarded.
			resolved, resolveErr := s.resolveUserAfterEmailConflict(ctx, buyerID, loginEmail)
			if resolveErr != nil {
				return nil, resolveErr
			}
			user = resolved
			password = ""
			loginEmail = ""
		} else {
			return nil, createErr
		}
		if err := s.bindingRepository.BindBuyerID(ctx, user.ID, buyerID); err != nil {
			return nil, err
		}
	}

	updatedUser, err := s.adminService.UpdateUserBalance(ctx, user.ID, rechargeAmount, "add", buildCardsIssueBalanceNotes(req, rechargeAmount))
	if err != nil {
		return nil, err
	}
	if !created {
		loginEmail = ""
		password = ""
	}

	vars := buildCardsIssueTemplateVars(req, updatedUser, created, password, rechargeAmount)
	card := renderCardsIssueTemplate(cfg.ResponseTemplate, vars)
	return &CardsIssueResult{
		Created:        created,
		UserID:         updatedUser.ID,
		LoginEmail:     loginEmailValue(created, updatedUser.Email),
		Username:       updatedUser.Username,
		Password:       password,
		RechargeAmount: rechargeAmount,
		BalanceAfter:   updatedUser.Balance,
		OrderID:        orderID,
		Card:           card,
	}, nil
}

func (s *CardsIssueService) resolveUserAfterEmailConflict(ctx context.Context, buyerID, loginEmail string) (*User, error) {
	if userID, ok, err := s.bindingRepository.FindUserIDByBuyerID(ctx, buyerID); err != nil {
		return nil, err
	} else if ok {
		return &User{ID: userID}, nil
	}
	existing, err := s.userLookup.GetByEmail(ctx, loginEmail)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, infraerrors.InternalServer("CARDS_ISSUE_USER_LOOKUP_FAILED", "failed to resolve existing user after email conflict")
	}
	return existing, nil
}

func loginEmailValue(created bool, email string) string {
	if !created {
		return ""
	}
	return email
}

func buildCardsIssueLoginEmail(buyerID string) string {
	normalized := strings.TrimSpace(strings.ToLower(buyerID))
	sum := sha256.Sum256([]byte(normalized))
	return "buyer_" + hex.EncodeToString(sum[:10]) + "@" + CardsIssueLoginEmailDomain
}

func buildCardsIssueUsername(buyerName, buyerID string) string {
	if trimmed := strings.TrimSpace(buyerName); trimmed != "" {
		return trimmed
	}
	return "buyer_" + strings.TrimSpace(buyerID)
}

func buildCardsIssueUserNotes(req CardsIssueRequest) string {
	parts := []string{
		"created by cards issue integration",
		"buyer_id=" + strings.TrimSpace(req.BuyerID),
		"order_id=" + strings.TrimSpace(req.OrderID),
	}
	if buyerName := strings.TrimSpace(req.BuyerName); buyerName != "" {
		parts = append(parts, "buyer_name="+buyerName)
	}
	if len(req.Extra) > 0 {
		if raw, err := json.Marshal(req.Extra); err == nil {
			parts = append(parts, "extra="+string(raw))
		}
	}
	return strings.Join(parts, " | ")
}

func buildCardsIssueBalanceNotes(req CardsIssueRequest, rechargeAmount float64) string {
	parts := []string{
		"cards issue recharge",
		"buyer_id=" + strings.TrimSpace(req.BuyerID),
		"order_id=" + strings.TrimSpace(req.OrderID),
		"amount=" + formatCardsIssueFloat(rechargeAmount),
	}
	if buyerName := strings.TrimSpace(req.BuyerName); buyerName != "" {
		parts = append(parts, "buyer_name="+buyerName)
	}
	return strings.Join(parts, " | ")
}

func buildCardsIssueTemplateVars(req CardsIssueRequest, user *User, created bool, password string, rechargeAmount float64) map[string]string {
	vars := map[string]string{
		"buyer_id":        strings.TrimSpace(req.BuyerID),
		"buyer_name":      strings.TrimSpace(req.BuyerName),
		"order_id":        strings.TrimSpace(req.OrderID),
		"order_amount":    formatCardsIssueFloat(req.OrderAmount),
		"order_quantity":  strconv.Itoa(req.OrderQuantity),
		"recharge_amount": formatCardsIssueFloat(rechargeAmount),
		"balance_after":   formatCardsIssueFloat(user.Balance),
		"user_id":         strconv.FormatInt(user.ID, 10),
		"username":        user.Username,
		"login_email":     loginEmailValue(created, user.Email),
		"password":        password,
		"created":         strconv.FormatBool(created),
	}
	if created {
		vars["user_status"] = "new"
		vars["account_notice"] = fmt.Sprintf("登录邮箱：%s\n登录密码：%s", user.Email, password)
	} else {
		vars["user_status"] = "existing"
		vars["account_notice"] = "老用户已充值，无新密码返回"
	}
	for key, value := range req.Extra {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		if _, exists := vars[trimmedKey]; exists {
			continue
		}
		vars[trimmedKey] = stringifyCardsIssueValue(value)
	}
	return vars
}

func renderCardsIssueTemplate(template string, vars map[string]string) string {
	template = normalizeCardsIssueResponseTemplate(template)
	return cardsIssueTemplatePattern.ReplaceAllStringFunc(template, func(token string) string {
		matches := cardsIssueTemplatePattern.FindStringSubmatch(token)
		if len(matches) != 2 {
			return token
		}
		key := strings.TrimSpace(matches[1])
		value, ok := vars[key]
		if !ok {
			return token
		}
		return value
	})
}

func stringifyCardsIssueValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case float64:
		return formatCardsIssueFloat(v)
	case float32:
		return formatCardsIssueFloat(float64(v))
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case bool:
		return strconv.FormatBool(v)
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(raw)
	}
}

func formatCardsIssueFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func generateCardsIssuePassword() (string, error) {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%"
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.Grow(len(bytes))
	for _, b := range bytes {
		builder.WriteByte(chars[int(b)%len(chars)])
	}
	return builder.String(), nil
}
