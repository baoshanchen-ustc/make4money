//go:build unit

package service

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
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
		wantErr        bool
		errContains    string
	}{
		{
			name:           "valid easypay with types",
			providerKey:    "easypay",
			providerName:   "MyProvider",
			supportedTypes: "alipay,wxpay",
			wantErr:        false,
		},
		{
			name:           "valid stripe with empty types",
			providerKey:    "stripe",
			providerName:   "Stripe Provider",
			supportedTypes: "",
			wantErr:        false,
		},
		{
			name:           "valid alipay provider",
			providerKey:    "alipay",
			providerName:   "Alipay Direct",
			supportedTypes: "alipay",
			wantErr:        false,
		},
		{
			name:           "valid wxpay provider",
			providerKey:    "wxpay",
			providerName:   "WeChat Pay",
			supportedTypes: "wxpay",
			wantErr:        false,
		},
		{
			name:           "legacy direct type normalizes for validation",
			providerKey:    "easypay",
			providerName:   "Legacy EasyPay",
			supportedTypes: "alipay_direct,wxpay_direct",
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
			name:           "alipay provider rejects wxpay capability",
			providerKey:    "alipay",
			providerName:   "Ali",
			supportedTypes: "wxpay",
			wantErr:        true,
			errContains:    "provider alipay only supports [alipay]",
		},
		{
			name:           "easypay rejects stripe capability",
			providerKey:    "easypay",
			providerName:   "Easy",
			supportedTypes: "stripe",
			wantErr:        true,
			errContains:    "provider easypay only supports [alipay wxpay]",
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateProviderRequest(tc.providerKey, tc.providerName, tc.supportedTypes)
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
			name:  "legacy direct types normalize and dedupe",
			input: []string{"alipay_direct", "alipay", "wxpay_direct"},
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
			name:  "types with spaces are trimmed",
			input: []string{" alipay ", " wxpay "},
			want:  "alipay,wxpay",
		},
		{
			name:  "stripe sub-methods are preserved",
			input: []string{" card ", "link", "alipay_direct"},
			want:  "card,link,alipay",
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

func TestValidateCapabilityRouteConflicts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		existing       []*dbent.PaymentProviderInstance
		selfID         int64
		providerKey    string
		supportedTypes string
		enabled        bool
		wantErr        bool
		errContains    string
	}{
		{
			name: "same provider type across multiple instances is allowed",
			existing: []*dbent.PaymentProviderInstance{
				{ID: 1, ProviderKey: "wxpay", SupportedTypes: "wxpay", Enabled: true},
				{ID: 2, ProviderKey: "wxpay", SupportedTypes: "wxpay_direct", Enabled: true},
			},
			providerKey:    "wxpay",
			supportedTypes: "wxpay",
			enabled:        true,
			wantErr:        false,
		},
		{
			name: "wxpay capability conflict is explicit",
			existing: []*dbent.PaymentProviderInstance{
				{ID: 1, ProviderKey: "wxpay", SupportedTypes: "wxpay", Enabled: true},
			},
			providerKey:    "easypay",
			supportedTypes: "wxpay_direct",
			enabled:        true,
			wantErr:        true,
			errContains:    "wxpay capability conflict: enabled provider types [easypay wxpay]",
		},
		{
			name: "disabled instance does not conflict",
			existing: []*dbent.PaymentProviderInstance{
				{ID: 1, ProviderKey: "wxpay", SupportedTypes: "wxpay", Enabled: true},
			},
			providerKey:    "easypay",
			supportedTypes: "wxpay",
			enabled:        false,
			wantErr:        false,
		},
		{
			name: "self instance is ignored during update",
			existing: []*dbent.PaymentProviderInstance{
				{ID: 9, ProviderKey: "easypay", SupportedTypes: "alipay", Enabled: true},
			},
			selfID:         9,
			providerKey:    "easypay",
			supportedTypes: "alipay_direct",
			enabled:        true,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateCapabilityRouteConflicts(tt.existing, tt.selfID, tt.providerKey, tt.supportedTypes, tt.enabled)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
