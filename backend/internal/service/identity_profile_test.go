package service

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testIdentityProfileSecret = "00000000000000000000000000000000-test-secret"

func TestIdentityProfileService_StableAcrossCalls(t *testing.T) {
	svc := NewIdentityProfileService(testIdentityProfileSecret, 14)
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	a := svc.Profile(42, PlatformAnthropic, now)
	b := svc.Profile(42, PlatformAnthropic, now.Add(6*time.Hour))

	require.Equal(t, a.MachineID, b.MachineID)
	require.Equal(t, a.OS, b.OS)
	require.Equal(t, a.Arch, b.Arch)
	require.Equal(t, a.Locale, b.Locale)
	require.Equal(t, a.Timezone, b.Timezone)
	require.Equal(t, a.RotationSalt, b.RotationSalt)
	require.NotEmpty(t, a.MachineID)
	require.Len(t, a.MachineID, 32, "machine_id should be 32 hex chars")
}

func TestIdentityProfileService_DifferentUsersGetDifferentProfiles(t *testing.T) {
	svc := NewIdentityProfileService(testIdentityProfileSecret, 14)
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	uniqueIDs := make(map[string]struct{})
	for userID := int64(1); userID <= 200; userID++ {
		p := svc.Profile(userID, PlatformAnthropic, now)
		uniqueIDs[p.MachineID] = struct{}{}
	}

	require.GreaterOrEqual(t, len(uniqueIDs), 195, "machine_id collisions should be extremely rare across 200 users")
}

func TestIdentityProfileService_DifferentPlatformsGetDifferentMachineIDs(t *testing.T) {
	svc := NewIdentityProfileService(testIdentityProfileSecret, 14)
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	anthropic := svc.Profile(42, PlatformAnthropic, now)
	openai := svc.Profile(42, PlatformOpenAI, now)
	gemini := svc.Profile(42, PlatformGemini, now)

	require.NotEqual(t, anthropic.MachineID, openai.MachineID)
	require.NotEqual(t, anthropic.MachineID, gemini.MachineID)
	require.NotEqual(t, openai.MachineID, gemini.MachineID)
}

func TestIdentityProfileService_RotatesAfterTTL(t *testing.T) {
	svc := NewIdentityProfileService(testIdentityProfileSecret, 14)
	t0 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(20 * 24 * time.Hour) // > 14 day window

	a := svc.Profile(42, PlatformAnthropic, t0)
	b := svc.Profile(42, PlatformAnthropic, t1)

	require.NotEqual(t, a.RotationSalt, b.RotationSalt, "rotation salt should change after TTL")
	require.NotEqual(t, a.MachineID, b.MachineID, "machine_id should rotate when salt changes")
}

func TestIdentityProfileService_CandidateValuesAreFromExpectedPools(t *testing.T) {
	svc := NewIdentityProfileService(testIdentityProfileSecret, 14)
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	for userID := int64(1); userID <= 50; userID++ {
		p := svc.Profile(userID, PlatformAnthropic, now)
		require.Contains(t, identityProfileOSPool, p.OS)
		require.Contains(t, identityProfileArchPool, p.Arch)
		require.Contains(t, identityProfileLocalePool, p.Locale)
		require.Contains(t, identityProfileTimezonePool, p.Timezone)
		require.NotEmpty(t, p.RotationSalt)
		require.True(t, strings.HasPrefix(p.UserAgentVersion, "2."), "cli version should be a 2.x release, got %q", p.UserAgentVersion)
	}
}

func TestIdentityProfileService_SecretChangeRotatesProfile(t *testing.T) {
	svcA := NewIdentityProfileService(testIdentityProfileSecret, 14)
	svcB := NewIdentityProfileService(testIdentityProfileSecret+"-rotated", 14)
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	a := svcA.Profile(42, PlatformAnthropic, now)
	b := svcB.Profile(42, PlatformAnthropic, now)
	require.NotEqual(t, a.MachineID, b.MachineID, "different secrets must produce different fingerprints")
}

func TestIdentityProfileService_EmptyInputsAreSafe(t *testing.T) {
	svc := NewIdentityProfileService("", 0)
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	a := svc.Profile(0, "", now)
	require.NotEmpty(t, a.MachineID)
	require.Equal(t, "unknown", a.Platform)

	b := svc.Profile(0, "", now)
	require.Equal(t, a.MachineID, b.MachineID, "deterministic even with zero/empty inputs")
}
