package service

import "time"

type UserSubscription struct {
	ID      int64
	UserID  int64
	GroupID int64

	StartsAt     time.Time
	ExpiresAt    time.Time
	Status       string
	PackageCount int

	DailyWindowStart   *time.Time
	WeeklyWindowStart  *time.Time
	MonthlyWindowStart *time.Time

	DailyUsageUSD   float64
	WeeklyUsageUSD  float64
	MonthlyUsageUSD float64

	AssignedBy *int64
	AssignedAt time.Time
	Notes      string

	CreatedAt time.Time
	UpdatedAt time.Time

	User           *User
	Group          *Group
	AssignedByUser *User
}

func (s *UserSubscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive && time.Now().Before(s.ExpiresAt)
}

func (s *UserSubscription) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

func (s *UserSubscription) DaysRemaining() int {
	if s.IsExpired() {
		return 0
	}
	return int(time.Until(s.ExpiresAt).Hours() / 24)
}

func (s *UserSubscription) IsWindowActivated() bool {
	return s.DailyWindowStart != nil || s.WeeklyWindowStart != nil || s.MonthlyWindowStart != nil
}

func (s *UserSubscription) EffectivePackageCount() int {
	if s.PackageCount <= 0 {
		return 1
	}
	return s.PackageCount
}

func (s *UserSubscription) EffectiveDailyLimit(group *Group) (float64, bool) {
	if group == nil || group.DailyLimitUSD == nil || *group.DailyLimitUSD <= 0 {
		return 0, false
	}
	return *group.DailyLimitUSD * float64(s.EffectivePackageCount()), true
}

func (s *UserSubscription) EffectiveWeeklyLimit(group *Group) (float64, bool) {
	if group == nil || group.WeeklyLimitUSD == nil || *group.WeeklyLimitUSD <= 0 {
		return 0, false
	}
	return *group.WeeklyLimitUSD * float64(s.EffectivePackageCount()), true
}

func (s *UserSubscription) EffectiveMonthlyLimit(group *Group) (float64, bool) {
	if group == nil || group.MonthlyLimitUSD == nil || *group.MonthlyLimitUSD <= 0 {
		return 0, false
	}
	return *group.MonthlyLimitUSD * float64(s.EffectivePackageCount()), true
}

func (s *UserSubscription) NeedsDailyReset() bool {
	if s.DailyWindowStart == nil {
		return false
	}
	return time.Since(*s.DailyWindowStart) >= 24*time.Hour
}

func (s *UserSubscription) NeedsWeeklyReset() bool {
	if s.WeeklyWindowStart == nil {
		return false
	}
	return time.Since(*s.WeeklyWindowStart) >= 7*24*time.Hour
}

func (s *UserSubscription) NeedsMonthlyReset() bool {
	if s.MonthlyWindowStart == nil {
		return false
	}
	return time.Since(*s.MonthlyWindowStart) >= 30*24*time.Hour
}

func (s *UserSubscription) DailyResetTime() *time.Time {
	if s.DailyWindowStart == nil {
		return nil
	}
	t := s.DailyWindowStart.Add(24 * time.Hour)
	return &t
}

func (s *UserSubscription) WeeklyResetTime() *time.Time {
	if s.WeeklyWindowStart == nil {
		return nil
	}
	t := s.WeeklyWindowStart.Add(7 * 24 * time.Hour)
	return &t
}

func (s *UserSubscription) MonthlyResetTime() *time.Time {
	if s.MonthlyWindowStart == nil {
		return nil
	}
	t := s.MonthlyWindowStart.Add(30 * 24 * time.Hour)
	return &t
}

func (s *UserSubscription) CheckDailyLimit(group *Group, additionalCost float64) bool {
	limit, ok := s.EffectiveDailyLimit(group)
	if !ok {
		return true
	}
	return s.DailyUsageUSD+additionalCost <= limit
}

func (s *UserSubscription) CheckWeeklyLimit(group *Group, additionalCost float64) bool {
	limit, ok := s.EffectiveWeeklyLimit(group)
	if !ok {
		return true
	}
	return s.WeeklyUsageUSD+additionalCost <= limit
}

func (s *UserSubscription) CheckMonthlyLimit(group *Group, additionalCost float64) bool {
	limit, ok := s.EffectiveMonthlyLimit(group)
	if !ok {
		return true
	}
	return s.MonthlyUsageUSD+additionalCost <= limit
}

func (s *UserSubscription) CheckAllLimits(group *Group, additionalCost float64) (daily, weekly, monthly bool) {
	daily = s.CheckDailyLimit(group, additionalCost)
	weekly = s.CheckWeeklyLimit(group, additionalCost)
	monthly = s.CheckMonthlyLimit(group, additionalCost)
	return
}
