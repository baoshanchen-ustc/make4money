package repository

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/useraccountbinding"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userAccountBindingRepository struct {
	client *ent.Client
}

// NewUserAccountBindingRepository creates a new UserAccountBindingRepository.
func NewUserAccountBindingRepository(client *ent.Client) service.UserAccountBindingRepository {
	return &userAccountBindingRepository{client: client}
}

func (r *userAccountBindingRepository) GetBinding(ctx context.Context, projectFP string, groupID int64) (*service.UserAccountBinding, error) {
	binding, err := r.client.UserAccountBinding.Query().
		Where(
			useraccountbinding.ProjectFp(projectFP),
			useraccountbinding.GroupID(groupID),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &service.UserAccountBinding{
		ID:        binding.ID,
		ProjectFP: binding.ProjectFp,
		AccountID: binding.AccountID,
		GroupID:   binding.GroupID,
		ExpiresAt: binding.ExpiresAt,
		CreatedAt: binding.CreatedAt,
		UpdatedAt: binding.UpdatedAt,
	}, nil
}

func (r *userAccountBindingRepository) UpsertBinding(ctx context.Context, projectFP string, accountID int64, groupID int64, expiresAt time.Time) error {
	return r.client.UserAccountBinding.Create().
		SetProjectFp(projectFP).
		SetAccountID(accountID).
		SetGroupID(groupID).
		SetExpiresAt(expiresAt).
		OnConflictColumns(useraccountbinding.FieldProjectFp, useraccountbinding.FieldGroupID).
		UpdateNewValues().
		Exec(ctx)
}

func (r *userAccountBindingRepository) DeleteBinding(ctx context.Context, projectFP string, groupID int64) error {
	_, err := r.client.UserAccountBinding.Delete().
		Where(
			useraccountbinding.ProjectFp(projectFP),
			useraccountbinding.GroupID(groupID),
		).
		Exec(ctx)
	return err
}

func (r *userAccountBindingRepository) DeleteByAccountID(ctx context.Context, accountID int64) (int, error) {
	return r.client.UserAccountBinding.Delete().
		Where(useraccountbinding.AccountID(accountID)).
		Exec(ctx)
}

func (r *userAccountBindingRepository) DeleteExpired(ctx context.Context) (int, error) {
	return r.client.UserAccountBinding.Delete().
		Where(useraccountbinding.ExpiresAtLT(time.Now())).
		Exec(ctx)
}

func (r *userAccountBindingRepository) SnapshotAccountUserFanout(ctx context.Context) (*service.AccountUserFanoutSnapshot, error) {
	type accountFanoutAgg struct {
		AccountID         int64 `json:"account_id"`
		ExternalUserCount int64 `json:"external_user_count"`
	}
	var aggs []accountFanoutAgg
	if err := r.client.UserAccountBinding.Query().
		Where(useraccountbinding.ExpiresAtGT(time.Now())).
		GroupBy(useraccountbinding.FieldAccountID).
		Aggregate(ent.As(ent.Count(), "external_user_count")).
		Scan(ctx, &aggs); err != nil {
		return nil, err
	}
	if len(aggs) == 0 {
		return &service.AccountUserFanoutSnapshot{}, nil
	}

	counts := make([]int64, 0, len(aggs))
	for _, a := range aggs {
		counts = append(counts, a.ExternalUserCount)
	}
	sort.Slice(counts, func(i, j int) bool { return counts[i] < counts[j] })
	idx := int(math.Ceil(float64(len(counts))*0.95)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(counts) {
		idx = len(counts) - 1
	}

	return &service.AccountUserFanoutSnapshot{
		AccountCount:     len(counts),
		ExternalUsersP95: float64(counts[idx]),
		ExternalUsersMax: counts[len(counts)-1],
	}, nil
}
