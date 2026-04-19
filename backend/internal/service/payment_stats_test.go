package service

import (
	"sort"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestBuildMethodDistributionNormalizesVisiblePaymentTypes(t *testing.T) {
	t.Parallel()

	methods := buildMethodDistribution([]*dbent.PaymentOrder{
		{PaymentType: "alipay", PayAmount: 5},
		{PaymentType: "alipay_direct", PayAmount: 10},
		{PaymentType: "wxpay_direct", PayAmount: 20},
		{PaymentType: "card", PayAmount: 3},
		{PaymentType: "link", PayAmount: 7.5},
		{PaymentType: "stripe", PayAmount: 9.5},
	})

	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Type < methods[j].Type
	})

	require.Equal(t, []PaymentMethodStat{
		{Type: "alipay", Amount: 15, Count: 2},
		{Type: "stripe", Amount: 20, Count: 3},
		{Type: "wxpay", Amount: 20, Count: 1},
	}, methods)
}
