package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
)

const (
	passkeyAAGUIDMetadataCachePathEnv = "PASSKEY_AAGUID_METADATA_CACHE_PATH"
)

type PasskeyAAGUIDMetadataCache interface {
	LookupFriendlyNameByAAGUID(ctx context.Context, aaguid string) (string, bool)
}

type passkeyFriendlyNameResolver struct {
	knownAAGUIDNames map[string]string
	metadataCache    PasskeyAAGUIDMetadataCache
}

type passkeyAAGUIDMetadataEntry struct {
	AAGUID       string `json:"aaguid"`
	Name         string `json:"name"`
	FriendlyName string `json:"friendly_name"`
}

type StaticPasskeyAAGUIDMetadataCache struct {
	entries map[string]string
}

var knownPasskeyAAGUIDFriendlyNames = map[string]string{
	"d548826e-79b4-db40-a3d8-11116f7e8349": "Bitwarden",
	"de1e552d-db1d-4423-a619-566b625cdc84": "Microsoft Authenticator (iOS)",
	"90a3ccdf-635c-4729-a248-9b709135078f": "Microsoft Authenticator (Android)",
	"7fd635b3-2ef9-4542-8d9d-164f2c771efc": "Platform Credential for macOS",
	"d7781e5d-e353-46aa-afe2-3ca49f13332a": "YubiKey 5 NFC",
	"50a45b0c-80e7-f944-bf29-f552bfa2e048": "ACS FIDO Authenticator",
	"7991798a-a7f3-487f-98c0-3faf7a458a04": "HID Crescendo Key V3",
}

func newPasskeyFriendlyNameResolver(metadataCache PasskeyAAGUIDMetadataCache) *passkeyFriendlyNameResolver {
	return &passkeyFriendlyNameResolver{
		knownAAGUIDNames: knownPasskeyAAGUIDFriendlyNames,
		metadataCache:    metadataCache,
	}
}

func (r *passkeyFriendlyNameResolver) Resolve(ctx context.Context, providedFriendlyName, aaguid string, now time.Time) string {
	if trimmed := strings.TrimSpace(providedFriendlyName); trimmed != "" {
		return trimmed
	}

	normalizedAAGUID := normalizePasskeyAAGUID(aaguid)
	if normalizedAAGUID != "" {
		if friendlyName, ok := r.knownAAGUIDNames[normalizedAAGUID]; ok {
			return friendlyName
		}

		if r.metadataCache != nil {
			if friendlyName, ok := r.metadataCache.LookupFriendlyNameByAAGUID(ctx, normalizedAAGUID); ok {
				return friendlyName
			}
		}
	}

	return passkeyFriendlyName("", now)
}

func NewStaticPasskeyAAGUIDMetadataCache(entries map[string]string) *StaticPasskeyAAGUIDMetadataCache {
	normalizedEntries := make(map[string]string, len(entries))
	for rawAAGUID, rawFriendlyName := range entries {
		aaguid := normalizePasskeyAAGUID(rawAAGUID)
		friendlyName := strings.TrimSpace(rawFriendlyName)
		if aaguid == "" || friendlyName == "" {
			continue
		}
		normalizedEntries[aaguid] = friendlyName
	}

	return &StaticPasskeyAAGUIDMetadataCache{entries: normalizedEntries}
}

func NewPasskeyAAGUIDMetadataCacheFromJSON(payload []byte) (*StaticPasskeyAAGUIDMetadataCache, error) {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return nil, fmt.Errorf("metadata payload is empty")
	}

	mapPayload := map[string]string{}
	if err := json.Unmarshal(payload, &mapPayload); err == nil {
		return NewStaticPasskeyAAGUIDMetadataCache(mapPayload), nil
	}

	var listPayload []passkeyAAGUIDMetadataEntry
	if err := json.Unmarshal(payload, &listPayload); err == nil {
		mapped := make(map[string]string, len(listPayload))
		for _, item := range listPayload {
			aaguid := normalizePasskeyAAGUID(item.AAGUID)
			friendlyName := strings.TrimSpace(item.FriendlyName)
			if friendlyName == "" {
				friendlyName = strings.TrimSpace(item.Name)
			}
			if aaguid == "" || friendlyName == "" {
				continue
			}
			mapped[aaguid] = friendlyName
		}
		return NewStaticPasskeyAAGUIDMetadataCache(mapped), nil
	}

	return nil, fmt.Errorf("metadata payload must be map[aaguid]friendly_name or []{aaguid,name}")
}

func NewPasskeyAAGUIDMetadataCacheFromFile(path string) (*StaticPasskeyAAGUIDMetadataCache, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return nil, fmt.Errorf("metadata file path is empty")
	}

	payload, err := os.ReadFile(trimmedPath)
	if err != nil {
		return nil, fmt.Errorf("read metadata file: %w", err)
	}

	cache, err := NewPasskeyAAGUIDMetadataCacheFromJSON(payload)
	if err != nil {
		return nil, fmt.Errorf("parse metadata file: %w", err)
	}

	return cache, nil
}

func (c *StaticPasskeyAAGUIDMetadataCache) LookupFriendlyNameByAAGUID(ctx context.Context, aaguid string) (string, bool) {
	_ = ctx
	if c == nil {
		return "", false
	}
	normalizedAAGUID := normalizePasskeyAAGUID(aaguid)
	if normalizedAAGUID == "" {
		return "", false
	}
	friendlyName, ok := c.entries[normalizedAAGUID]
	if !ok {
		return "", false
	}
	friendlyName = strings.TrimSpace(friendlyName)
	if friendlyName == "" {
		return "", false
	}
	return friendlyName, true
}

func loadOptionalPasskeyAAGUIDMetadataCacheFromEnv() PasskeyAAGUIDMetadataCache {
	path := strings.TrimSpace(os.Getenv(passkeyAAGUIDMetadataCachePathEnv))
	if path == "" {
		return nil
	}

	cache, err := NewPasskeyAAGUIDMetadataCacheFromFile(path)
	if err != nil {
		logger.LegacyPrintf("service.passkey", "warning: skip passkey metadata cache load from %q: %v", path, err)
		return nil
	}

	return cache
}

func normalizePasskeyAAGUID(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}

	if parsed, err := uuid.Parse(trimmed); err == nil {
		return parsed.String()
	}

	return trimmed
}
