package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestMarshalAccountForCache_NilAccount(t *testing.T) {
	data, err := marshalAccountForCache(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil, got %s", data)
	}
}

func TestMarshalAccountForCache_EmptyCredentials(t *testing.T) {
	account := &service.Account{ID: 1, Name: "test"}
	data, err := marshalAccountForCache(account)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil data")
	}
}

func TestMarshalAccountForCache_StripsHeavyTokens(t *testing.T) {
	account := &service.Account{
		ID:   1,
		Name: "test",
		Credentials: map[string]any{
			"access_token":  "keep-this",
			"api_key":       "keep-this-too",
			"id_token":      "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.very-long-jwt-token",
			"refresh_token": "rt_very-long-refresh-token",
			"other_field":   "also-keep",
		},
	}

	data, err := marshalAccountForCache(account)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify stripped fields are absent from serialised output.
	s := string(data)
	for _, denied := range schedulerCacheCredentialDenyList {
		if containsKey(s, denied) {
			t.Errorf("expected %q to be stripped from cache payload", denied)
		}
	}

	// Verify preserved fields are present.
	for _, kept := range []string{"access_token", "api_key", "other_field"} {
		if !containsKey(s, kept) {
			t.Errorf("expected %q to be preserved in cache payload", kept)
		}
	}
}

func TestMarshalAccountForCache_DoesNotMutateOriginal(t *testing.T) {
	original := map[string]any{
		"access_token":  "at",
		"id_token":      "idt",
		"refresh_token": "rt",
	}
	account := &service.Account{
		ID:          1,
		Credentials: original,
	}

	_, err := marshalAccountForCache(account)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original map must still contain all keys.
	for _, key := range []string{"access_token", "id_token", "refresh_token"} {
		if _, ok := original[key]; !ok {
			t.Errorf("original credentials map was mutated: missing %q", key)
		}
	}
}

// containsKey is a simple helper that checks if a JSON string contains a key.
func containsKey(json, key string) bool {
	return len(json) > 0 && len(key) > 0 &&
		// Match "key": pattern in JSON.
		contains(json, `"`+key+`"`)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
