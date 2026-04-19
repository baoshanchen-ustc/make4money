//go:build unit

package payment

import (
	"encoding/json"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

func TestInstanceSupportsType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		supportedTypes string
		target         PaymentType
		expected       bool
	}{
		{
			name:           "exact match single type",
			supportedTypes: "alipay",
			target:         "alipay",
			expected:       true,
		},
		{
			name:           "no match single type",
			supportedTypes: "wxpay",
			target:         "alipay",
			expected:       false,
		},
		{
			name:           "match in comma-separated list",
			supportedTypes: "alipay,wxpay,stripe",
			target:         "wxpay",
			expected:       true,
		},
		{
			name:           "first in comma-separated list",
			supportedTypes: "alipay,wxpay",
			target:         "alipay",
			expected:       true,
		},
		{
			name:           "last in comma-separated list",
			supportedTypes: "alipay,wxpay,stripe",
			target:         "stripe",
			expected:       true,
		},
		{
			name:           "no match in comma-separated list",
			supportedTypes: "alipay,wxpay",
			target:         "stripe",
			expected:       false,
		},
		{
			name:           "empty target",
			supportedTypes: "alipay,wxpay",
			target:         "",
			expected:       false,
		},
		{
			name:           "types with spaces are trimmed",
			supportedTypes: " alipay , wxpay ",
			target:         "alipay",
			expected:       true,
		},
		{
			name:           "legacy direct type is normalized to alipay",
			supportedTypes: "alipay_direct",
			target:         "alipay",
			expected:       true,
		},
		{
			name:           "empty supported types means all supported",
			supportedTypes: "",
			target:         "alipay",
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := InstanceSupportsType(tt.supportedTypes, tt.target)
			if got != tt.expected {
				t.Fatalf("InstanceSupportsType(%q, %q) = %v, want %v", tt.supportedTypes, tt.target, got, tt.expected)
			}
		})
	}
}

func TestVisiblePaymentTypesForProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		providerKey    string
		supportedTypes string
		want           []PaymentType
	}{
		{
			name:        "alipay provider defaults to alipay capability",
			providerKey: TypeAlipay,
			want:        []PaymentType{TypeAlipay},
		},
		{
			name:        "wxpay provider defaults to wxpay capability",
			providerKey: TypeWxpay,
			want:        []PaymentType{TypeWxpay},
		},
		{
			name:        "easypay provider with empty types routes both capabilities",
			providerKey: TypeEasyPay,
			want:        []PaymentType{TypeAlipay, TypeWxpay},
		},
		{
			name:           "legacy direct values normalize for easypay",
			providerKey:    TypeEasyPay,
			supportedTypes: "alipay_direct,wxpay_direct",
			want:           []PaymentType{TypeAlipay, TypeWxpay},
		},
		{
			name:           "unsupported type is ignored for direct provider",
			providerKey:    TypeAlipay,
			supportedTypes: "wxpay",
			want:           nil,
		},
		{
			name:           "stripe stays isolated regardless of sub-methods",
			providerKey:    TypeStripe,
			supportedTypes: "card,link,wxpay",
			want:           []PaymentType{TypeStripe},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := VisiblePaymentTypesForProvider(tt.providerKey, tt.supportedTypes)
			if !stringSliceEqual(paymentTypesToStrings(got), paymentTypesToStrings(tt.want)) {
				t.Fatalf("VisiblePaymentTypesForProvider(%q, %q) = %v, want %v", tt.providerKey, tt.supportedTypes, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helper to build test PaymentProviderInstance values
// ---------------------------------------------------------------------------

func testInstance(id int64, providerKey, limits string) *dbent.PaymentProviderInstance {
	return &dbent.PaymentProviderInstance{
		ID:          id,
		ProviderKey: providerKey,
		Limits:      limits,
		Enabled:     true,
	}
}

// makeLimitsJSON builds a limits JSON string for a single payment type.
func makeLimitsJSON(paymentType string, cl ChannelLimits) string {
	m := map[string]ChannelLimits{paymentType: cl}
	b, _ := json.Marshal(m)
	return string(b)
}

// ---------------------------------------------------------------------------
// filterByLimits
// ---------------------------------------------------------------------------

func TestFilterByLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		candidates  []instanceCandidate
		paymentType PaymentType
		orderAmount float64
		wantIDs     []int64 // expected surviving instance IDs
	}{
		{
			name: "order below SingleMin is filtered out",
			candidates: []instanceCandidate{
				{inst: testInstance(1, "easypay", makeLimitsJSON("alipay", ChannelLimits{SingleMin: 10})), dailyUsed: 0},
			},
			paymentType: "alipay",
			orderAmount: 5,
			wantIDs:     nil,
		},
		{
			name: "order at exact SingleMin boundary passes",
			candidates: []instanceCandidate{
				{inst: testInstance(1, "easypay", makeLimitsJSON("alipay", ChannelLimits{SingleMin: 10})), dailyUsed: 0},
			},
			paymentType: "alipay",
			orderAmount: 10,
			wantIDs:     []int64{1},
		},
		{
			name: "order above SingleMax is filtered out",
			candidates: []instanceCandidate{
				{inst: testInstance(1, "easypay", makeLimitsJSON("alipay", ChannelLimits{SingleMax: 100})), dailyUsed: 0},
			},
			paymentType: "alipay",
			orderAmount: 150,
			wantIDs:     nil,
		},
		{
			name: "order at exact SingleMax boundary passes",
			candidates: []instanceCandidate{
				{inst: testInstance(1, "easypay", makeLimitsJSON("alipay", ChannelLimits{SingleMax: 100})), dailyUsed: 0},
			},
			paymentType: "alipay",
			orderAmount: 100,
			wantIDs:     []int64{1},
		},
		{
			name: "daily used + orderAmount exceeding dailyLimit is filtered out",
			candidates: []instanceCandidate{
				{inst: testInstance(1, "easypay", makeLimitsJSON("alipay", ChannelLimits{DailyLimit: 500})), dailyUsed: 480},
			},
			paymentType: "alipay",
			orderAmount: 30,
			wantIDs:     nil, // 480+30=510 > 500
		},
		{
			name: "daily used + orderAmount equal to dailyLimit passes (strict greater-than)",
			candidates: []instanceCandidate{
				{inst: testInstance(1, "easypay", makeLimitsJSON("alipay", ChannelLimits{DailyLimit: 500})), dailyUsed: 480},
			},
			paymentType: "alipay",
			orderAmount: 20,
			wantIDs:     []int64{1}, // 480+20=500, 500 > 500 is false → passes
		},
		{
			name: "daily used + orderAmount below dailyLimit passes",
			candidates: []instanceCandidate{
				{inst: testInstance(1, "easypay", makeLimitsJSON("alipay", ChannelLimits{DailyLimit: 500})), dailyUsed: 400},
			},
			paymentType: "alipay",
			orderAmount: 50,
			wantIDs:     []int64{1},
		},
		{
			name: "no limits configured passes through",
			candidates: []instanceCandidate{
				{inst: testInstance(1, "easypay", ""), dailyUsed: 99999},
			},
			paymentType: "alipay",
			orderAmount: 100,
			wantIDs:     []int64{1},
		},
		{
			name: "multiple candidates with partial filtering",
			candidates: []instanceCandidate{
				// singleMax=50, order=80 → filtered out
				{inst: testInstance(1, "easypay", makeLimitsJSON("alipay", ChannelLimits{SingleMax: 50})), dailyUsed: 0},
				// no limits → passes
				{inst: testInstance(2, "easypay", ""), dailyUsed: 0},
				// singleMin=100, order=80 → filtered out
				{inst: testInstance(3, "easypay", makeLimitsJSON("alipay", ChannelLimits{SingleMin: 100})), dailyUsed: 0},
				// daily limit ok → passes (500+80=580 < 1000)
				{inst: testInstance(4, "easypay", makeLimitsJSON("alipay", ChannelLimits{DailyLimit: 1000})), dailyUsed: 500},
			},
			paymentType: "alipay",
			orderAmount: 80,
			wantIDs:     []int64{2, 4},
		},
		{
			name: "zero SingleMin and SingleMax means no single-transaction limit",
			candidates: []instanceCandidate{
				{inst: testInstance(1, "easypay", makeLimitsJSON("alipay", ChannelLimits{SingleMin: 0, SingleMax: 0, DailyLimit: 0})), dailyUsed: 0},
			},
			paymentType: "alipay",
			orderAmount: 99999,
			wantIDs:     []int64{1},
		},
		{
			name: "all limits combined - order passes all checks",
			candidates: []instanceCandidate{
				{inst: testInstance(1, "easypay", makeLimitsJSON("alipay", ChannelLimits{SingleMin: 10, SingleMax: 200, DailyLimit: 1000})), dailyUsed: 500},
			},
			paymentType: "alipay",
			orderAmount: 50,
			wantIDs:     []int64{1},
		},
		{
			name: "all limits combined - order fails SingleMin",
			candidates: []instanceCandidate{
				{inst: testInstance(1, "easypay", makeLimitsJSON("alipay", ChannelLimits{SingleMin: 10, SingleMax: 200, DailyLimit: 1000})), dailyUsed: 500},
			},
			paymentType: "alipay",
			orderAmount: 5,
			wantIDs:     nil,
		},
		{
			name:        "empty candidates returns empty",
			candidates:  nil,
			paymentType: "alipay",
			orderAmount: 10,
			wantIDs:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterByLimits(tt.candidates, tt.paymentType, tt.orderAmount)
			gotIDs := make([]int64, len(got))
			for i, c := range got {
				gotIDs[i] = c.inst.ID
			}
			if !int64SliceEqual(gotIDs, tt.wantIDs) {
				t.Fatalf("filterByLimits() returned IDs %v, want %v", gotIDs, tt.wantIDs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// pickLeastAmount
// ---------------------------------------------------------------------------

func TestPickLeastAmount(t *testing.T) {
	t.Parallel()

	t.Run("picks candidate with lowest dailyUsed", func(t *testing.T) {
		t.Parallel()
		candidates := []instanceCandidate{
			{inst: testInstance(1, "easypay", ""), dailyUsed: 300},
			{inst: testInstance(2, "easypay", ""), dailyUsed: 100},
			{inst: testInstance(3, "easypay", ""), dailyUsed: 200},
		}
		got := pickLeastAmount(candidates)
		if got.inst.ID != 2 {
			t.Fatalf("pickLeastAmount() picked instance %d, want 2", got.inst.ID)
		}
	})

	t.Run("with equal dailyUsed picks the first one", func(t *testing.T) {
		t.Parallel()
		candidates := []instanceCandidate{
			{inst: testInstance(1, "easypay", ""), dailyUsed: 100},
			{inst: testInstance(2, "easypay", ""), dailyUsed: 100},
			{inst: testInstance(3, "easypay", ""), dailyUsed: 200},
		}
		got := pickLeastAmount(candidates)
		if got.inst.ID != 1 {
			t.Fatalf("pickLeastAmount() picked instance %d, want 1 (first with lowest)", got.inst.ID)
		}
	})

	t.Run("single candidate returns that candidate", func(t *testing.T) {
		t.Parallel()
		candidates := []instanceCandidate{
			{inst: testInstance(42, "easypay", ""), dailyUsed: 999},
		}
		got := pickLeastAmount(candidates)
		if got.inst.ID != 42 {
			t.Fatalf("pickLeastAmount() picked instance %d, want 42", got.inst.ID)
		}
	})

	t.Run("zero usage among non-zero picks zero", func(t *testing.T) {
		t.Parallel()
		candidates := []instanceCandidate{
			{inst: testInstance(1, "easypay", ""), dailyUsed: 500},
			{inst: testInstance(2, "easypay", ""), dailyUsed: 0},
			{inst: testInstance(3, "easypay", ""), dailyUsed: 300},
		}
		got := pickLeastAmount(candidates)
		if got.inst.ID != 2 {
			t.Fatalf("pickLeastAmount() picked instance %d, want 2", got.inst.ID)
		}
	})
}

// ---------------------------------------------------------------------------
// getInstanceChannelLimits
// ---------------------------------------------------------------------------

func TestGetInstanceChannelLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inst        *dbent.PaymentProviderInstance
		paymentType PaymentType
		want        ChannelLimits
	}{
		{
			name:        "empty limits string returns zero ChannelLimits",
			inst:        testInstance(1, "easypay", ""),
			paymentType: "alipay",
			want:        ChannelLimits{},
		},
		{
			name:        "invalid JSON returns zero ChannelLimits",
			inst:        testInstance(1, "easypay", "not-json{"),
			paymentType: "alipay",
			want:        ChannelLimits{},
		},
		{
			name: "valid JSON with matching payment type",
			inst: testInstance(1, "easypay",
				`{"alipay":{"singleMin":5,"singleMax":200,"dailyLimit":1000}}`),
			paymentType: "alipay",
			want:        ChannelLimits{SingleMin: 5, SingleMax: 200, DailyLimit: 1000},
		},
		{
			name: "payment type not in limits returns zero ChannelLimits",
			inst: testInstance(1, "easypay",
				`{"alipay":{"singleMin":5,"singleMax":200}}`),
			paymentType: "wxpay",
			want:        ChannelLimits{},
		},
		{
			name: "legacy direct limits key is normalized for lookup",
			inst: testInstance(1, "easypay",
				`{"wxpay_direct":{"singleMin":8,"singleMax":88}}`),
			paymentType: "wxpay",
			want:        ChannelLimits{SingleMin: 8, SingleMax: 88},
		},
		{
			name: "stripe provider uses stripe lookup key regardless of payment type",
			inst: testInstance(1, "stripe",
				`{"stripe":{"singleMin":10,"singleMax":500,"dailyLimit":5000}}`),
			paymentType: "alipay",
			want:        ChannelLimits{SingleMin: 10, SingleMax: 500, DailyLimit: 5000},
		},
		{
			name: "stripe provider ignores payment type key even if present",
			inst: testInstance(1, "stripe",
				`{"stripe":{"singleMin":10,"singleMax":500},"alipay":{"singleMin":1,"singleMax":100}}`),
			paymentType: "alipay",
			want:        ChannelLimits{SingleMin: 10, SingleMax: 500},
		},
		{
			name: "non-stripe provider uses payment type as lookup key",
			inst: testInstance(1, "easypay",
				`{"alipay":{"singleMin":5},"wxpay":{"singleMin":10}}`),
			paymentType: "wxpay",
			want:        ChannelLimits{SingleMin: 10},
		},
		{
			name: "valid JSON with partial limits (only dailyLimit)",
			inst: testInstance(1, "easypay",
				`{"alipay":{"dailyLimit":800}}`),
			paymentType: "alipay",
			want:        ChannelLimits{DailyLimit: 800},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getInstanceChannelLimits(tt.inst, tt.paymentType)
			if got != tt.want {
				t.Fatalf("getInstanceChannelLimits() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateCapabilitySelection(t *testing.T) {
	t.Parallel()

	t.Run("allows multiple instances from the same provider source", func(t *testing.T) {
		t.Parallel()

		err := validateCapabilitySelection(TypeWxpay, []*dbent.PaymentProviderInstance{
			{ID: 1, ProviderKey: string(TypeWxpay), SupportedTypes: "wxpay"},
			{ID: 2, ProviderKey: string(TypeWxpay), SupportedTypes: "wxpay"},
		})

		if err != nil {
			t.Fatalf("validateCapabilitySelection() unexpected error: %v", err)
		}
	})

	t.Run("rejects mixed provider sources for the same visible capability", func(t *testing.T) {
		t.Parallel()

		err := validateCapabilitySelection(TypeAlipay, []*dbent.PaymentProviderInstance{
			{ID: 1, ProviderKey: string(TypeEasyPay), SupportedTypes: "alipay"},
			{ID: 2, ProviderKey: string(TypeAlipay), SupportedTypes: "alipay"},
		})

		if err == nil || err.Error() != "alipay capability conflict: enabled provider types [alipay easypay]" {
			t.Fatalf("validateCapabilitySelection() = %v, want explicit alipay capability conflict", err)
		}
	})

	t.Run("never treats stripe as a visible source conflict", func(t *testing.T) {
		t.Parallel()

		err := validateCapabilitySelection(TypeStripe, []*dbent.PaymentProviderInstance{
			{ID: 1, ProviderKey: string(TypeStripe), SupportedTypes: "card,alipay,wxpay,link"},
			{ID: 2, ProviderKey: string(TypeStripe), SupportedTypes: "card"},
		})

		if err != nil {
			t.Fatalf("validateCapabilitySelection() unexpected stripe error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// startOfDay
// ---------------------------------------------------------------------------

func TestStartOfDay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{
			name: "midday returns midnight of same day",
			in:   time.Date(2025, 6, 15, 14, 30, 45, 123456789, time.UTC),
			want: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "midnight returns same time",
			in:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			want: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "last second of day returns midnight of same day",
			in:   time.Date(2025, 12, 31, 23, 59, 59, 999999999, time.UTC),
			want: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "preserves timezone location",
			in:   time.Date(2025, 3, 10, 15, 0, 0, 0, time.FixedZone("CST", 8*3600)),
			want: time.Date(2025, 3, 10, 0, 0, 0, 0, time.FixedZone("CST", 8*3600)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := startOfDay(tt.in)
			if !got.Equal(tt.want) {
				t.Fatalf("startOfDay(%v) = %v, want %v", tt.in, got, tt.want)
			}
			// Also verify location is preserved.
			if got.Location().String() != tt.want.Location().String() {
				t.Fatalf("startOfDay() location = %v, want %v", got.Location(), tt.want.Location())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// int64SliceEqual compares two int64 slices for equality.
// Both nil and empty slices are treated as equal.
func int64SliceEqual(a, b []int64) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func paymentTypesToStrings(types []PaymentType) []string {
	out := make([]string, len(types))
	for i, t := range types {
		out[i] = string(t)
	}
	return out
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
