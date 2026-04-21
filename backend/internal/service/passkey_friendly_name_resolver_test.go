package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPasskeyFriendlyNameResolver_ResolvePrecedence(t *testing.T) {
	now := time.Date(2026, 3, 30, 9, 30, 0, 0, time.UTC)
	metadataOnlyAAGUID := "11111111-2222-4333-8444-555555555555"
	yubiKey5NFCAAGUID := "d7781e5d-e353-46aa-afe2-3ca49f13332a"

	resolver := newPasskeyFriendlyNameResolver(NewStaticPasskeyAAGUIDMetadataCache(map[string]string{
		yubiKey5NFCAAGUID:  "YubiKey 5 NFC (Metadata)",
		metadataOnlyAAGUID: "FIDO Metadata Key",
	}))

	require.Equal(t, "Personal Key", resolver.Resolve(t.Context(), "  Personal Key  ", yubiKey5NFCAAGUID, now))
	require.Equal(t, "YubiKey 5 NFC", resolver.Resolve(t.Context(), "", yubiKey5NFCAAGUID, now))
	require.Equal(t, "FIDO Metadata Key", resolver.Resolve(t.Context(), "", metadataOnlyAAGUID, now))
	require.Equal(t, passkeyFriendlyName("", now), resolver.Resolve(t.Context(), "", "unmapped-aaguid", now))
}

func TestKnownPasskeyAAGUIDFriendlyNames(t *testing.T) {
	require.Equal(t, map[string]string{
		"d548826e-79b4-db40-a3d8-11116f7e8349": "Bitwarden",
		"de1e552d-db1d-4423-a619-566b625cdc84": "Microsoft Authenticator (iOS)",
		"90a3ccdf-635c-4729-a248-9b709135078f": "Microsoft Authenticator (Android)",
		"7fd635b3-2ef9-4542-8d9d-164f2c771efc": "Platform Credential for macOS",
		"d7781e5d-e353-46aa-afe2-3ca49f13332a": "YubiKey 5 NFC",
		"50a45b0c-80e7-f944-bf29-f552bfa2e048": "ACS FIDO Authenticator",
		"7991798a-a7f3-487f-98c0-3faf7a458a04": "HID Crescendo Key V3",
	}, knownPasskeyAAGUIDFriendlyNames)
}

func TestNewPasskeyAAGUIDMetadataCacheFromJSON_SupportsMapPayload(t *testing.T) {
	payload := []byte(`{"d548826e-79b4-db40-a3d8-11116f7e8349":"Bitwarden Metadata"}`)

	cache, err := NewPasskeyAAGUIDMetadataCacheFromJSON(payload)
	require.NoError(t, err)
	require.NotNil(t, cache)

	name, ok := cache.LookupFriendlyNameByAAGUID(t.Context(), "d548826e-79b4-db40-a3d8-11116f7e8349")
	require.True(t, ok)
	require.Equal(t, "Bitwarden Metadata", name)
}

func TestNewPasskeyAAGUIDMetadataCacheFromJSON_SupportsListPayload(t *testing.T) {
	payload := []byte(`[{"aaguid":"11111111-2222-4333-8444-555555555555","name":"Metadata Hardware Key"}]`)

	cache, err := NewPasskeyAAGUIDMetadataCacheFromJSON(payload)
	require.NoError(t, err)
	require.NotNil(t, cache)

	name, ok := cache.LookupFriendlyNameByAAGUID(t.Context(), "11111111-2222-4333-8444-555555555555")
	require.True(t, ok)
	require.Equal(t, "Metadata Hardware Key", name)
}

func TestLoadOptionalPasskeyAAGUIDMetadataCacheFromEnv_LoadsCache(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "aaguid-metadata.json")
	err := os.WriteFile(metadataPath, []byte(`{"11111111-2222-4333-8444-555555555555":"Env Metadata Key"}`), 0o600)
	require.NoError(t, err)

	t.Setenv(passkeyAAGUIDMetadataCachePathEnv, metadataPath)

	cache := loadOptionalPasskeyAAGUIDMetadataCacheFromEnv()
	require.NotNil(t, cache)

	name, ok := cache.LookupFriendlyNameByAAGUID(t.Context(), "11111111-2222-4333-8444-555555555555")
	require.True(t, ok)
	require.Equal(t, "Env Metadata Key", name)
}
