package repository

import (
	"strings"
	"testing"
	"time"

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

func TestMarshalAccountForCache_StripsGroupObjects(t *testing.T) {
	group := &service.Group{
		ID:          7,
		Name:        "TestGroup",
		Description: "A group with many fields that waste bandwidth",
		Platform:    "openai",
	}
	account := &service.Account{
		ID:   1,
		Name: "test",
		AccountGroups: []service.AccountGroup{
			{
				AccountID: 1,
				GroupID:   7,
				Priority:  1,
				CreatedAt: time.Now(),
				Group:     group,
			},
		},
		Groups: []*service.Group{group},
	}

	data, err := marshalAccountForCache(account)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(data)

	// Group name should not appear — full Group objects are stripped.
	if containsKey(s, "TestGroup") {
		t.Error("expected full Group object to be stripped from AccountGroups")
	}

	// GroupID must still be present for routing.
	if !strings.Contains(s, `"GroupID":7`) && !strings.Contains(s, `"group_id":7`) {
		t.Error("expected GroupID to be preserved in AccountGroups")
	}
}

func TestMarshalAccountForCache_DoesNotMutateOriginal(t *testing.T) {
	group := &service.Group{ID: 7, Name: "TestGroup"}
	originalCreds := map[string]any{
		"access_token":  "at",
		"id_token":      "idt",
		"refresh_token": "rt",
	}
	originalAGs := []service.AccountGroup{
		{AccountID: 1, GroupID: 7, Group: group},
	}
	originalGroups := []*service.Group{group}

	account := &service.Account{
		ID:            1,
		Credentials:   originalCreds,
		AccountGroups: originalAGs,
		Groups:        originalGroups,
	}

	_, err := marshalAccountForCache(account)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original credentials map must still contain all keys.
	for _, key := range []string{"access_token", "id_token", "refresh_token"} {
		if _, ok := originalCreds[key]; !ok {
			t.Errorf("original credentials map was mutated: missing %q", key)
		}
	}

	// Original AccountGroups must still have Group pointer.
	if account.AccountGroups[0].Group == nil {
		t.Error("original AccountGroups[0].Group was mutated to nil")
	}

	// Original Groups must still be populated.
	if account.Groups == nil || len(account.Groups) == 0 {
		t.Error("original Groups slice was mutated")
	}
}

// containsKey checks if a JSON string contains a "key": pattern.
func containsKey(jsonStr, key string) bool {
	return strings.Contains(jsonStr, `"`+key+`"`)
}
