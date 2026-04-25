//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedeemRecharge_AccruesAffiliateRebateForPositiveBalanceCode(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)

	inviteeID := int64(2)
	inviterID := int64(1)
	userRepo := &mockUserRepo{
		getByIDUser: &User{ID: inviteeID, Balance: 0},
	}
	userRepo.updateBalanceFn = func(_ context.Context, id int64, amount float64) error {
		require.Equal(t, inviteeID, id)
		require.Equal(t, 100.0, amount)
		userRepo.getByIDUser.Balance += amount
		return nil
	}

	redeemRepo := &paymentOrderLifecycleRedeemRepo{
		codesByCode: map[string]*RedeemCode{
			"recharge-code-1": {
				ID:     10,
				Code:   "recharge-code-1",
				Type:   RedeemTypeBalance,
				Value:  100,
				Status: StatusUnused,
			},
		},
	}
	affiliateRepo := &affiliateRepoStub{
		summaries: map[int64]*AffiliateSummary{
			inviteeID: {UserID: inviteeID, InviterID: &inviterID},
			inviterID: {UserID: inviterID, AffCode: "INVITER"},
		},
	}
	settingSvc := NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:    "true",
		SettingKeyAffiliateRebateRate: "20",
	}}, nil)
	affiliateSvc := NewAffiliateService(affiliateRepo, settingSvc, nil, nil)
	redeemSvc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, affiliateSvc)

	redeemed, err := redeemSvc.RedeemRecharge(ctx, inviteeID, "recharge-code-1")
	require.NoError(t, err)
	require.Equal(t, StatusUsed, redeemed.Status)
	require.Equal(t, 100.0, userRepo.getByIDUser.Balance)
	require.Len(t, affiliateRepo.accrueCalls, 1)
	require.Equal(t, inviterID, affiliateRepo.accrueCalls[0].inviterID)
	require.Equal(t, inviteeID, affiliateRepo.accrueCalls[0].inviteeUserID)
	require.InDelta(t, 20.0, affiliateRepo.accrueCalls[0].amount, 1e-9)
}

func TestRedeem_DoesNotAccrueAffiliateRebateForManualRedeem(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)

	inviteeID := int64(2)
	inviterID := int64(1)
	userRepo := &mockUserRepo{
		getByIDUser: &User{ID: inviteeID, Balance: 0},
	}
	userRepo.updateBalanceFn = func(_ context.Context, id int64, amount float64) error {
		require.Equal(t, inviteeID, id)
		require.Equal(t, 100.0, amount)
		userRepo.getByIDUser.Balance += amount
		return nil
	}

	redeemRepo := &paymentOrderLifecycleRedeemRepo{
		codesByCode: map[string]*RedeemCode{
			"manual-code-1": {
				ID:     11,
				Code:   "manual-code-1",
				Type:   RedeemTypeBalance,
				Value:  100,
				Status: StatusUnused,
			},
		},
	}
	affiliateRepo := &affiliateRepoStub{
		summaries: map[int64]*AffiliateSummary{
			inviteeID: {UserID: inviteeID, InviterID: &inviterID},
			inviterID: {UserID: inviterID, AffCode: "INVITER"},
		},
	}
	settingSvc := NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:    "true",
		SettingKeyAffiliateRebateRate: "20",
	}}, nil)
	affiliateSvc := NewAffiliateService(affiliateRepo, settingSvc, nil, nil)
	redeemSvc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, affiliateSvc)

	redeemed, err := redeemSvc.Redeem(ctx, inviteeID, "manual-code-1")
	require.NoError(t, err)
	require.Equal(t, StatusUsed, redeemed.Status)
	require.Equal(t, 100.0, userRepo.getByIDUser.Balance)
	require.Empty(t, affiliateRepo.accrueCalls)
}
