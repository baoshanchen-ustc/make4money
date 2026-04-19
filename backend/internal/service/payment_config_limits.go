package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// GetAvailableMethodLimits collects user-facing payment capabilities from enabled provider
// instances and returns limits for each, plus the global widest range.
func (s *PaymentConfigService) GetAvailableMethodLimits(ctx context.Context) (*MethodLimitsResponse, error) {
	instances, err := s.entClient.PaymentProviderInstance.Query().
		Where(paymentproviderinstance.EnabledEQ(true)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query provider instances: %w", err)
	}
	cfg, err := s.GetPaymentConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get payment config: %w", err)
	}
	typeInstances := pcGroupByPaymentType(instances)
	resp := &MethodLimitsResponse{
		Methods: make(map[string]MethodLimits, len(typeInstances)),
	}
	for pt, insts := range typeInstances {
		ml := pcAggregateMethodLimits(pt, insts)
		ml.FeeRate = cfg.RechargeFeeRate
		resp.Methods[ml.PaymentType] = ml
	}
	if usageMap, usageErr := s.loadMethodUsageMap(ctx, instances); usageErr == nil {
		resp.Methods = applyMethodAvailability(resp.Methods, typeInstances, usageMap)
	}
	resp.Methods = filterMethodLimitsByEnabledTypes(resp.Methods, cfg)
	resp.GlobalMin, resp.GlobalMax = pcComputeGlobalRange(resp.Methods)
	return resp, nil
}

// GetMethodLimits returns per-payment-type limits from enabled provider instances.
func (s *PaymentConfigService) GetMethodLimits(ctx context.Context, types []string) ([]MethodLimits, error) {
	resp, err := s.GetAvailableMethodLimits(ctx)
	if err != nil {
		return nil, err
	}
	return selectRequestedMethodLimits(resp.Methods, types), nil
}

// pcGroupByPaymentType groups instances by user-facing payment capability.
func pcGroupByPaymentType(instances []*dbent.PaymentProviderInstance) map[string][]*dbent.PaymentProviderInstance {
	typeInstances := make(map[string][]*dbent.PaymentProviderInstance)
	seen := make(map[string]map[int64]bool)
	add := func(key string, inst *dbent.PaymentProviderInstance) {
		if seen[key] == nil {
			seen[key] = make(map[int64]bool)
		}
		if !seen[key][int64(inst.ID)] {
			seen[key][int64(inst.ID)] = true
			typeInstances[key] = append(typeInstances[key], inst)
		}
	}
	for _, inst := range instances {
		for _, t := range payment.VisiblePaymentTypesForProvider(inst.ProviderKey, inst.SupportedTypes) {
			add(string(t), inst)
		}
	}
	return typeInstances
}

// pcInstanceTypeLimits extracts per-type limits from a provider instance.
// Returns (limits, true) if configured; (zero, false) if unlimited.
// Supports legacy direct keys on read while exposing only visible capabilities.
func pcInstanceTypeLimits(inst *dbent.PaymentProviderInstance, pt string) (payment.ChannelLimits, bool) {
	if inst.Limits == "" {
		return payment.ChannelLimits{}, false
	}
	var limits payment.InstanceLimits
	if err := json.Unmarshal([]byte(inst.Limits), &limits); err != nil {
		return payment.ChannelLimits{}, false
	}
	for _, lookupKey := range pcLimitLookupKeys(inst.ProviderKey, pt) {
		if cl, ok := limits[lookupKey]; ok {
			return cl, true
		}
	}
	return payment.ChannelLimits{}, false
}

// unionFloat merges a single limit value into the aggregate using UNION semantics.
//   - For "min" fields (wantMin=true): keeps the lowest non-zero value
//   - For "max"/"cap" fields (wantMin=false): keeps the highest non-zero value
//   - If any value is 0 (unlimited), the result is unlimited.
//
// Returns (aggregated value, still limited).
func unionFloat(agg float64, limited bool, val float64, wantMin bool) (float64, bool) {
	if val == 0 {
		return agg, false
	}
	if !limited {
		return agg, false
	}
	if agg == 0 {
		return val, true
	}
	if wantMin && val < agg {
		return val, true
	}
	if !wantMin && val > agg {
		return val, true
	}
	return agg, true
}

// pcAggregateMethodLimits computes the UNION (least restrictive) of limits
// across all provider instances for a given payment type.
//
// Since the load balancer can route an order to any available instance,
// the user should see the widest possible range:
//   - SingleMin: lowest floor across instances; 0 if any is unlimited
//   - SingleMax: highest ceiling across instances; 0 if any is unlimited
//   - DailyLimit: highest cap across instances; 0 if any is unlimited
func pcAggregateMethodLimits(pt string, instances []*dbent.PaymentProviderInstance) MethodLimits {
	ml := MethodLimits{PaymentType: pt}
	minLimited, maxLimited, dailyLimited := true, true, true

	for _, inst := range instances {
		cl, hasLimits := pcInstanceTypeLimits(inst, pt)
		if !hasLimits {
			return MethodLimits{PaymentType: pt} // any unlimited instance → all zeros
		}
		ml.SingleMin, minLimited = unionFloat(ml.SingleMin, minLimited, cl.SingleMin, true)
		ml.SingleMax, maxLimited = unionFloat(ml.SingleMax, maxLimited, cl.SingleMax, false)
		ml.DailyLimit, dailyLimited = unionFloat(ml.DailyLimit, dailyLimited, cl.DailyLimit, false)
	}

	if !minLimited {
		ml.SingleMin = 0
	}
	if !maxLimited {
		ml.SingleMax = 0
	}
	if !dailyLimited {
		ml.DailyLimit = 0
	}
	return ml
}

// pcComputeGlobalRange computes the widest [min, max] across all methods.
// Uses the same union logic: lowest min, highest max, 0 if any is unlimited.
func pcComputeGlobalRange(methods map[string]MethodLimits) (globalMin, globalMax float64) {
	minLimited, maxLimited := true, true
	for _, ml := range methods {
		globalMin, minLimited = unionFloat(globalMin, minLimited, ml.SingleMin, true)
		globalMax, maxLimited = unionFloat(globalMax, maxLimited, ml.SingleMax, false)
	}
	if !minLimited {
		globalMin = 0
	}
	if !maxLimited {
		globalMax = 0
	}
	return globalMin, globalMax
}

func filterMethodLimitsByEnabledTypes(methods map[string]MethodLimits, cfg *PaymentConfig) map[string]MethodLimits {
	if len(methods) == 0 {
		return methods
	}
	if cfg == nil || len(cfg.EnabledTypes) == 0 {
		return methods
	}
	filtered := make(map[string]MethodLimits, len(methods))
	for key, ml := range methods {
		if psIsEnabledPaymentType(key, cfg.EnabledTypes) {
			filtered[key] = ml
		}
	}
	return filtered
}

func selectRequestedMethodLimits(methods map[string]MethodLimits, types []string) []MethodLimits {
	result := make([]MethodLimits, 0, len(types))
	for _, pt := range types {
		key := string(payment.NormalizeVisiblePaymentType(pt))
		if ml, ok := methods[key]; ok {
			result = append(result, ml)
			continue
		}
		result = append(result, MethodLimits{PaymentType: key})
	}
	return result
}

type methodUsageSnapshot struct {
	dailyUsed      float64
	dailyRemaining float64
	available      bool
}

func (s *PaymentConfigService) loadMethodUsageMap(ctx context.Context, instances []*dbent.PaymentProviderInstance) (map[int64]float64, error) {
	if len(instances) == 0 {
		return map[int64]float64{}, nil
	}
	ids := make([]string, 0, len(instances))
	for _, inst := range instances {
		ids = append(ids, strconv.FormatInt(int64(inst.ID), 10))
	}

	orders, err := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.ProviderInstanceIDIn(ids...),
			paymentorder.StatusIn(
				OrderStatusPending,
				OrderStatusPaid,
				OrderStatusCompleted,
				OrderStatusRecharging,
			),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query method usage: %w", err)
	}
	todayStart := psStartOfDayUTC(time.Now())
	usageMap := make(map[int64]float64, len(orders))
	for _, order := range orders {
		if !pcOrderCountsForProviderDailyUsage(order, todayStart) {
			continue
		}
		if order.ProviderInstanceID == nil || strings.TrimSpace(*order.ProviderInstanceID) == "" {
			continue
		}
		id, parseErr := strconv.ParseInt(*order.ProviderInstanceID, 10, 64)
		if parseErr != nil || id <= 0 {
			continue
		}
		usageMap[id] += order.PayAmount
	}
	return usageMap, nil
}

func applyMethodAvailability(methods map[string]MethodLimits, grouped map[string][]*dbent.PaymentProviderInstance, usageMap map[int64]float64) map[string]MethodLimits {
	for pt, ml := range methods {
		snapshot := computeMethodUsageSnapshot(pt, grouped[pt], usageMap)
		ml.DailyUsed = snapshot.dailyUsed
		ml.DailyRemaining = snapshot.dailyRemaining
		ml.Available = snapshot.available
		methods[pt] = ml
	}
	return methods
}

func computeMethodUsageSnapshot(pt string, instances []*dbent.PaymentProviderInstance, usageMap map[int64]float64) methodUsageSnapshot {
	if len(instances) == 0 {
		return methodUsageSnapshot{}
	}
	bestRemaining := -1.0
	bestUsed := 0.0
	for _, inst := range instances {
		cl, hasLimits := pcInstanceTypeLimits(inst, pt)
		if !hasLimits || cl.DailyLimit <= 0 {
			return methodUsageSnapshot{available: true}
		}
		used := usageMap[int64(inst.ID)]
		remaining := cl.DailyLimit - used
		if remaining > bestRemaining {
			bestRemaining = remaining
			bestUsed = used
		}
	}
	if bestRemaining < 0 {
		bestRemaining = 0
	}
	return methodUsageSnapshot{
		dailyUsed:      bestUsed,
		dailyRemaining: bestRemaining,
		available:      bestRemaining > 0,
	}
}

func pcOrderCountsForProviderDailyUsage(order *dbent.PaymentOrder, todayStart time.Time) bool {
	if order == nil {
		return false
	}
	if order.Status == OrderStatusPending {
		return true
	}
	if order.Status == OrderStatusPaid || order.Status == OrderStatusCompleted || order.Status == OrderStatusRecharging {
		return order.PaidAt != nil && !order.PaidAt.Before(todayStart)
	}
	return false
}

func pcInstanceSupportsVisibleType(inst *dbent.PaymentProviderInstance, pt string) bool {
	target := payment.NormalizeVisiblePaymentType(pt)
	for _, capability := range payment.VisiblePaymentTypesForProvider(inst.ProviderKey, inst.SupportedTypes) {
		if capability == target {
			return true
		}
	}
	return false
}

func pcLimitLookupKeys(providerKey, pt string) []string {
	if providerKey == payment.TypeStripe {
		return []string{payment.TypeStripe}
	}

	switch payment.NormalizeVisiblePaymentType(pt) {
	case payment.TypeAlipay:
		return []string{payment.TypeAlipay, payment.TypeAlipayDirect}
	case payment.TypeWxpay:
		return []string{payment.TypeWxpay, payment.TypeWxpayDirect}
	default:
		return []string{string(payment.NormalizeVisiblePaymentType(pt))}
	}
}
