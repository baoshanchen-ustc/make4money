package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestExtractProjectFPRaw_StablePath_DeviceID 当 metadata.user_id 包含 device_id 时
// 应当走稳定路径，返回 isStable=true 且不同 groupID 产生不同 fp。
func TestExtractProjectFPRaw_StablePath_DeviceID(t *testing.T) {
	s := &GatewayService{}
	// 使用 ParseMetadataUserID 接受的 legacy 格式：device_id 必须是 64 位 hex，
	// session_id 必须是标准 UUID。
	uid := "user_" +
		"1111111111111111111111111111111111111111111111111111111111111111" +
		"_account__session_" +
		"550e8400-e29b-41d4-a716-446655440000"

	fp1, stable1 := s.ExtractProjectFPRaw(uid, 100, "1.2.3.4", 7)
	fp2, stable2 := s.ExtractProjectFPRaw(uid, 100, "1.2.3.4", 7)
	require.True(t, stable1, "device_id 应触发稳定路径")
	require.True(t, stable2)
	require.NotEmpty(t, fp1)
	require.Equal(t, fp1, fp2, "同 device_id + 同 groupID 必须产生同 fp")
	require.Len(t, fp1, 32, "fp 长度固定 32 字符")

	// 不同 groupID 应产生不同 fp（防止跨组泄漏）
	fp3, _ := s.ExtractProjectFPRaw(uid, 100, "1.2.3.4", 8)
	require.NotEqual(t, fp1, fp3, "不同 groupID 必须产生不同 fp")

	// 同 device_id 即使 IP / api_key 变化也保持稳定（device 优先）
	fp4, stable4 := s.ExtractProjectFPRaw(uid, 200, "9.9.9.9", 7)
	require.True(t, stable4)
	require.Equal(t, fp1, fp4, "稳定路径仅依赖 device_id 和 groupID")
}

// TestExtractProjectFPRaw_FallbackPath_IPNet24 当没有 device_id 时退化到
// (api_key_id, IPv4 /24, group_id)。同子网应产生同 fp，跨子网产生不同 fp。
func TestExtractProjectFPRaw_FallbackPath_IPNet24(t *testing.T) {
	s := &GatewayService{}
	fp1, stable1 := s.ExtractProjectFPRaw("", 42, "10.0.0.1", 1)
	fp2, stable2 := s.ExtractProjectFPRaw("", 42, "10.0.0.99", 1)
	require.False(t, stable1, "无 device_id 必须返回 isStable=false")
	require.False(t, stable2)
	require.NotEmpty(t, fp1)
	require.Equal(t, fp1, fp2, "同 /24 子网必须产生同 fp")

	fp3, _ := s.ExtractProjectFPRaw("", 42, "10.0.1.1", 1)
	require.NotEqual(t, fp1, fp3, "跨 /24 子网必须产生不同 fp")

	// 不同 api_key 必须产生不同 fp（防止 API key 共享时跨用户合并）
	fp4, _ := s.ExtractProjectFPRaw("", 43, "10.0.0.1", 1)
	require.NotEqual(t, fp1, fp4, "不同 api_key 必须产生不同 fp")
}

// TestExtractProjectFPRaw_NoInputs 没有 device_id 也没有 api_key 时返回空 fp。
func TestExtractProjectFPRaw_NoInputs(t *testing.T) {
	s := &GatewayService{}
	fp, stable := s.ExtractProjectFPRaw("", 0, "1.2.3.4", 1)
	require.Empty(t, fp)
	require.False(t, stable)
}

// TestExtractProjectFP_ParsedEquivalence 通过 ParsedRequest 路径 (Anthropic) 与
// 通过 raw inputs 路径 (Gemini/OpenAI) 必须产生完全一致的 fp，否则同一用户在
// Anthropic 和 OpenAI 上会被识别为不同 binding，违背设计意图。
func TestExtractProjectFP_ParsedEquivalence(t *testing.T) {
	s := &GatewayService{}
	groupID := int64(5)
	parsed := &ParsedRequest{
		MetadataUserID: "",
		GroupID:        &groupID,
		SessionContext: &SessionContext{
			ClientIP: "172.16.5.123",
			APIKeyID: 999,
		},
	}
	fpFromParsed, stableFromParsed := s.ExtractProjectFP(parsed)
	fpFromRaw, stableFromRaw := s.ExtractProjectFPRaw("", 999, "172.16.5.123", 5)

	require.Equal(t, fpFromParsed, fpFromRaw, "ExtractProjectFP 与 ExtractProjectFPRaw 必须语义等价")
	require.Equal(t, stableFromParsed, stableFromRaw)
}

// TestSnapshotLongTermBindingMetrics_HitRate 验证命中率字段计算正确。
// 注意：因为是包级 atomic counter，不能并行修改；只读快照。
func TestSnapshotLongTermBindingMetrics_HitRate(t *testing.T) {
	// 先记录基线，避免被其它测试污染影响断言。
	before := SnapshotLongTermBindingMetrics()
	if before.ResolveHitTotal+before.ResolveMissTotal == 0 {
		require.Equal(t, float64(0), before.HitRate, "无样本时 HitRate 应为 0")
	}

	// 手动制造 2 hit + 3 miss
	longTermBindingResolveHitTotal.Add(2)
	longTermBindingResolveMissTotal.Add(3)
	defer func() {
		longTermBindingResolveHitTotal.Add(-2)
		longTermBindingResolveMissTotal.Add(-3)
	}()

	snap := SnapshotLongTermBindingMetrics()
	deltaHit := snap.ResolveHitTotal - before.ResolveHitTotal
	deltaMiss := snap.ResolveMissTotal - before.ResolveMissTotal
	require.Equal(t, int64(2), deltaHit)
	require.Equal(t, int64(3), deltaMiss)
	// 总命中率 = (before.hit + 2) / (before.total + 5)
	totalHit := snap.ResolveHitTotal
	totalMiss := snap.ResolveMissTotal
	expected := float64(totalHit) / float64(totalHit+totalMiss)
	require.InDelta(t, expected, snap.HitRate, 1e-9)
}
