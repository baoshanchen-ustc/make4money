package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const DefaultCardsIssueResponseTemplate = "【发货成功】\n订单号：{order_id}\n买家ID：{buyer_id}\n买家昵称：{buyer_name}\n账号状态：{user_status}\n充值金额：{recharge_amount}\n当前余额：{balance_after}\n{account_notice}"

type CardsIssueAdminConfig struct {
	Enabled          bool   `json:"enabled"`
	ResponseTemplate string `json:"response_template"`
	KeyExists        bool   `json:"key_exists"`
	MaskedKey        string `json:"masked_key"`
}

type UpdateCardsIssueConfigInput struct {
	Enabled          bool
	ResponseTemplate string
}

type CardsIssueRuntimeConfig struct {
	Enabled          bool
	BearerKey        string
	ResponseTemplate string
}

func normalizeCardsIssueResponseTemplate(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return DefaultCardsIssueResponseTemplate
	}
	return trimmed
}

func maskSensitiveKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 16 {
		return key
	}
	return key[:12] + "..." + key[len(key)-4:]
}

func (s *SettingService) GetCardsIssueAdminConfig(ctx context.Context) (*CardsIssueAdminConfig, error) {
	cfg, err := s.GetCardsIssueRuntimeConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &CardsIssueAdminConfig{
		Enabled:          cfg.Enabled,
		ResponseTemplate: cfg.ResponseTemplate,
		KeyExists:        strings.TrimSpace(cfg.BearerKey) != "",
		MaskedKey:        maskSensitiveKey(cfg.BearerKey),
	}, nil
}

func (s *SettingService) GetCardsIssueRuntimeConfig(ctx context.Context) (*CardsIssueRuntimeConfig, error) {
	if s == nil || s.settingRepo == nil {
		return &CardsIssueRuntimeConfig{ResponseTemplate: DefaultCardsIssueResponseTemplate}, nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyCardsIssueEnabled,
		SettingKeyCardsIssueBearerKey,
		SettingKeyCardsIssueResponseTemplate,
	})
	if err != nil {
		return nil, err
	}
	return &CardsIssueRuntimeConfig{
		Enabled:          strings.TrimSpace(values[SettingKeyCardsIssueEnabled]) == "true",
		BearerKey:        strings.TrimSpace(values[SettingKeyCardsIssueBearerKey]),
		ResponseTemplate: normalizeCardsIssueResponseTemplate(values[SettingKeyCardsIssueResponseTemplate]),
	}, nil
}

func (s *SettingService) UpdateCardsIssueConfig(ctx context.Context, input UpdateCardsIssueConfigInput) error {
	if s == nil || s.settingRepo == nil {
		return ErrServiceUnavailable
	}
	updates := map[string]string{
		SettingKeyCardsIssueEnabled:          fmt.Sprintf("%t", input.Enabled),
		SettingKeyCardsIssueResponseTemplate: normalizeCardsIssueResponseTemplate(input.ResponseTemplate),
	}
	return s.settingRepo.SetMultiple(ctx, updates)
}

func (s *SettingService) GenerateCardsIssueBearerKey(ctx context.Context) (string, error) {
	if s == nil || s.settingRepo == nil {
		return "", ErrServiceUnavailable
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	key := CardsIssueBearerKeyPrefix + hex.EncodeToString(bytes)
	if err := s.settingRepo.Set(ctx, SettingKeyCardsIssueBearerKey, key); err != nil {
		return "", fmt.Errorf("save cards issue bearer key: %w", err)
	}
	return key, nil
}

func (s *SettingService) DeleteCardsIssueBearerKey(ctx context.Context) error {
	if s == nil || s.settingRepo == nil {
		return ErrServiceUnavailable
	}
	return s.settingRepo.Delete(ctx, SettingKeyCardsIssueBearerKey)
}

func (s *SettingService) GetCardsIssueBearerKey(ctx context.Context) (string, error) {
	cfg, err := s.GetCardsIssueRuntimeConfig(ctx)
	if err != nil {
		return "", err
	}
	return cfg.BearerKey, nil
}

func (s *SettingService) GetCardsIssueBearerKeyStatus(ctx context.Context) (maskedKey string, exists bool, err error) {
	key, err := s.GetCardsIssueBearerKey(ctx)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false, nil
	}
	return maskSensitiveKey(key), true, nil
}
