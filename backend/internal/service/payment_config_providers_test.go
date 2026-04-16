//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateProviderRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		providerKey    string
		providerName   string
		supportedTypes string
		config         map[string]string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "valid easypay with types",
			providerKey:    "easypay",
			providerName:   "MyProvider",
			supportedTypes: "alipay,wxpay",
			config:         map[string]string{"apiBase": "https://example.com"},
			wantErr:        false,
		},
		{
			name:           "valid stripe with empty types",
			providerKey:    "stripe",
			providerName:   "Stripe Provider",
			supportedTypes: "",
			config:         map[string]string{"secretKey": "sk_test_123"},
			wantErr:        false,
		},
		{
			name:           "valid alipay provider",
			providerKey:    "alipay",
			providerName:   "Alipay Direct",
			supportedTypes: "alipay",
			config: map[string]string{
				"appId":      "2021001234567890",
				"privateKey": "private-key",
				"publicKey":  "public-key",
			},
			wantErr: false,
		},
		{
			name:           "valid alipay provider with legacy alipayPublicKey",
			providerKey:    "alipay",
			providerName:   "Alipay Direct Legacy",
			supportedTypes: "alipay",
			config: map[string]string{
				"appId":           "2021001234567890",
				"privateKey":      "private-key",
				"alipayPublicKey": "public-key",
			},
			wantErr: false,
		},
		{
			name:           "valid wxpay provider",
			providerKey:    "wxpay",
			providerName:   "WeChat Pay",
			supportedTypes: "wxpay",
			config:         map[string]string{"mchId": "1234567890"},
			wantErr:        false,
		},
		{
			name:           "invalid provider key",
			providerKey:    "invalid",
			providerName:   "Name",
			supportedTypes: "alipay",
			wantErr:        true,
			errContains:    "invalid provider key",
		},
		{
			name:           "empty name",
			providerKey:    "easypay",
			providerName:   "",
			supportedTypes: "alipay",
			wantErr:        true,
			errContains:    "provider name is required",
		},
		{
			name:           "whitespace-only name",
			providerKey:    "easypay",
			providerName:   "  ",
			supportedTypes: "alipay",
			wantErr:        true,
			errContains:    "provider name is required",
		},
		{
			name:           "tab-only name",
			providerKey:    "easypay",
			providerName:   "\t",
			supportedTypes: "alipay",
			wantErr:        true,
			errContains:    "provider name is required",
		},
		{
			name:           "alipay missing public key is rejected",
			providerKey:    "alipay",
			providerName:   "Alipay Direct",
			supportedTypes: "alipay",
			config: map[string]string{
				"appId":      "2021001234567890",
				"privateKey": "private-key",
			},
			wantErr:     true,
			errContains: "alipay config missing required key: publicKey (or alipayPublicKey)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateProviderRequest(tc.providerKey, tc.providerName, tc.supportedTypes, tc.config)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIsSensitiveConfigField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		field   string
		wantSen bool
	}{
		// Sensitive fields (contain key/secret/private/password/pkey patterns)
		{"secretKey", true},
		{"apiSecret", true},
		{"pkey", true},
		{"privateKey", true},
		{"apiPassword", true},
		{"appKey", true},
		{"SECRET_TOKEN", true},
		{"PrivateData", true},
		{"PASSWORD", true},
		{"mySecretValue", true},

		// Non-sensitive fields
		{"appId", false},
		{"mchId", false},
		{"apiBase", false},
		{"endpoint", false},
		{"merchantNo", false},
		{"paymentMode", false},
		{"notifyUrl", false},
	}

	for _, tc := range tests {
		t.Run(tc.field, func(t *testing.T) {
			t.Parallel()

			got := isSensitiveConfigField(tc.field)
			assert.Equal(t, tc.wantSen, got, "isSensitiveConfigField(%q)", tc.field)
		})
	}
}

func TestDecryptAndMaskConfig(t *testing.T) {
	t.Parallel()

	svc := &PaymentConfigService{
		encryptionKey: []byte(strings.Repeat("k", 32)),
	}
	encrypted, err := svc.encryptConfig(map[string]string{
		"appId":      "2021001234567890",
		"privateKey": "merchant-private-key",
		"publicKey":  "merchant-public-key",
		"notifyUrl":  "https://crs.qazwc.com/api/v1/pay/notify/alipay",
		"password":   "",
	})
	require.NoError(t, err)

	masked, err := svc.decryptAndMaskConfig(encrypted)
	require.NoError(t, err)
	require.NotNil(t, masked)
	assert.Equal(t, "2021001234567890", masked["appId"])
	assert.Equal(t, maskedSensitiveConfigValue, masked["privateKey"])
	assert.Equal(t, maskedSensitiveConfigValue, masked["publicKey"])
	assert.Equal(t, "https://crs.qazwc.com/api/v1/pay/notify/alipay", masked["notifyUrl"])
	assert.Equal(t, "", masked["password"])
}

func TestMergeProviderConfigKeepsExistingSensitiveValueWhenClientEchoesMask(t *testing.T) {
	t.Parallel()

	merged := mergeProviderConfig(
		map[string]string{
			"appId":      "2021001234567890",
			"privateKey": "original-private-key",
			"notifyUrl":  "https://old.example.com/notify",
		},
		map[string]string{
			"privateKey": maskedSensitiveConfigValue,
			"notifyUrl":  "https://new.example.com/notify",
		},
	)

	assert.Equal(t, "2021001234567890", merged["appId"])
	assert.Equal(t, "original-private-key", merged["privateKey"])
	assert.Equal(t, "https://new.example.com/notify", merged["notifyUrl"])
}

func TestJoinTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{
			name:  "multiple types",
			input: []string{"alipay", "wxpay"},
			want:  "alipay,wxpay",
		},
		{
			name:  "single type",
			input: []string{"stripe"},
			want:  "stripe",
		},
		{
			name:  "empty slice",
			input: []string{},
			want:  "",
		},
		{
			name:  "nil slice",
			input: nil,
			want:  "",
		},
		{
			name:  "three types",
			input: []string{"alipay", "wxpay", "stripe"},
			want:  "alipay,wxpay,stripe",
		},
		{
			name:  "types with spaces are not trimmed",
			input: []string{" alipay ", " wxpay "},
			want:  " alipay , wxpay ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := joinTypes(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
