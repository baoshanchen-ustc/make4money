//go:build unit

package service

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

const (
	checkinEnabledKey        = "checkin_enabled"
	checkinRewardBalanceKey  = "checkin_reward_balance"
	checkinTimezoneKey       = "checkin_timezone"
	checkinHistoryVisibleKey = "checkin_history_visible"
)

type checkinSettingRepoStub struct {
	values        map[string]string
	updates       map[string]string
	requestedKeys []string
}

func (s *checkinSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *checkinSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *checkinSettingRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *checkinSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	s.requestedKeys = append([]string(nil), keys...)

	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *checkinSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	s.updates = make(map[string]string, len(settings))
	for k, v := range settings {
		s.updates[k] = v
	}
	return nil
}

func (s *checkinSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for k, v := range s.values {
		out[k] = v
	}
	return out, nil
}

func (s *checkinSettingRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestSettingService_UpdateSettings_CheckinFieldsMappedToSettingKeys(t *testing.T) {
	repo := &checkinSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	input := &SystemSettings{}
	setBoolFieldByNameCandidates(t, input, []string{"CheckinEnabled", "CheckInEnabled"}, true)
	setFloatFieldByNameCandidates(t, input, []string{"CheckinRewardBalance", "CheckInRewardBalance"}, 2.75)
	setStringFieldByNameCandidates(t, input, []string{"CheckinTimezone", "CheckInTimezone"}, "Asia/Shanghai")
	setBoolFieldByNameCandidates(t, input, []string{"CheckinHistoryVisible", "CheckInHistoryVisible"}, false)

	err := svc.UpdateSettings(context.Background(), input)
	require.NoError(t, err)

	require.Equal(t, "true", repo.updates[checkinEnabledKey])
	require.Equal(t, "Asia/Shanghai", repo.updates[checkinTimezoneKey])
	require.Equal(t, "false", repo.updates[checkinHistoryVisibleKey])

	rewardRaw, ok := repo.updates[checkinRewardBalanceKey]
	require.True(t, ok, "expected %s to be persisted", checkinRewardBalanceKey)
	reward, parseErr := strconv.ParseFloat(rewardRaw, 64)
	require.NoError(t, parseErr)
	require.InDelta(t, 2.75, reward, 1e-9)
}

func TestSettingService_GetAllSettings_ParsesCheckinFields(t *testing.T) {
	repo := &checkinSettingRepoStub{
		values: map[string]string{
			checkinEnabledKey:        "true",
			checkinRewardBalanceKey:  "1.5",
			checkinTimezoneKey:       "UTC",
			checkinHistoryVisibleKey: "false",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)

	require.True(t, getBoolFieldByNameCandidates(t, settings, []string{"CheckinEnabled", "CheckInEnabled"}))
	require.InDelta(t, 1.5, getFloatFieldByNameCandidates(t, settings, []string{"CheckinRewardBalance", "CheckInRewardBalance"}), 1e-9)
	require.Equal(t, "UTC", getStringFieldByNameCandidates(t, settings, []string{"CheckinTimezone", "CheckInTimezone"}))
	require.False(t, getBoolFieldByNameCandidates(t, settings, []string{"CheckinHistoryVisible", "CheckInHistoryVisible"}))
}

func TestSettingService_GetPublicSettings_ExposesCheckinFields(t *testing.T) {
	repo := &checkinSettingRepoStub{
		values: map[string]string{
			checkinEnabledKey:        "true",
			checkinRewardBalanceKey:  "3.25",
			checkinTimezoneKey:       "Asia/Shanghai",
			checkinHistoryVisibleKey: "true",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)

	require.Contains(t, repo.requestedKeys, checkinEnabledKey)
	require.Contains(t, repo.requestedKeys, checkinRewardBalanceKey)
	require.Contains(t, repo.requestedKeys, checkinTimezoneKey)
	require.Contains(t, repo.requestedKeys, checkinHistoryVisibleKey)

	require.True(t, getBoolFieldByNameCandidates(t, settings, []string{"CheckinEnabled", "CheckInEnabled"}))
	require.InDelta(t, 3.25, getFloatFieldByNameCandidates(t, settings, []string{"CheckinRewardBalance", "CheckInRewardBalance"}), 1e-9)
	require.Equal(t, "Asia/Shanghai", getStringFieldByNameCandidates(t, settings, []string{"CheckinTimezone", "CheckInTimezone"}))
	require.True(t, getBoolFieldByNameCandidates(t, settings, []string{"CheckinHistoryVisible", "CheckInHistoryVisible"}))
}

func setBoolFieldByNameCandidates(t *testing.T, target any, names []string, value bool) {
	t.Helper()
	field, name := mustFindFieldByNames(t, target, names)
	switch field.Kind() {
	case reflect.Bool:
		field.SetBool(value)
	case reflect.Pointer:
		if field.Type().Elem().Kind() != reflect.Bool {
			t.Fatalf("field %s is pointer but not *bool", name)
		}
		v := reflect.New(field.Type().Elem())
		v.Elem().SetBool(value)
		field.Set(v)
	default:
		t.Fatalf("field %s has unsupported kind %s for bool setter", name, field.Kind())
	}
}

func setFloatFieldByNameCandidates(t *testing.T, target any, names []string, value float64) {
	t.Helper()
	field, name := mustFindFieldByNames(t, target, names)
	switch field.Kind() {
	case reflect.Float32, reflect.Float64:
		field.SetFloat(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		field.SetInt(int64(value))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		field.SetUint(uint64(value))
	case reflect.Pointer:
		elemKind := field.Type().Elem().Kind()
		v := reflect.New(field.Type().Elem())
		switch elemKind {
		case reflect.Float32, reflect.Float64:
			v.Elem().SetFloat(value)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v.Elem().SetInt(int64(value))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			v.Elem().SetUint(uint64(value))
		default:
			t.Fatalf("field %s is pointer but unsupported numeric type %s", name, elemKind)
		}
		field.Set(v)
	default:
		t.Fatalf("field %s has unsupported kind %s for float setter", name, field.Kind())
	}
}

func setStringFieldByNameCandidates(t *testing.T, target any, names []string, value string) {
	t.Helper()
	field, name := mustFindFieldByNames(t, target, names)
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Pointer:
		if field.Type().Elem().Kind() != reflect.String {
			t.Fatalf("field %s is pointer but not *string", name)
		}
		v := reflect.New(field.Type().Elem())
		v.Elem().SetString(value)
		field.Set(v)
	default:
		t.Fatalf("field %s has unsupported kind %s for string setter", name, field.Kind())
	}
}

func getBoolFieldByNameCandidates(t *testing.T, source any, names []string) bool {
	t.Helper()
	field, name := mustFindReadableFieldByNames(t, source, names)
	switch field.Kind() {
	case reflect.Bool:
		return field.Bool()
	case reflect.Pointer:
		if field.IsNil() {
			return false
		}
		if field.Elem().Kind() == reflect.Bool {
			return field.Elem().Bool()
		}
	case reflect.String:
		return field.String() == "true"
	}
	t.Fatalf("field %s has unsupported kind %s for bool getter", name, field.Kind())
	return false
}

func getFloatFieldByNameCandidates(t *testing.T, source any, names []string) float64 {
	t.Helper()
	field, name := mustFindReadableFieldByNames(t, source, names)
	switch field.Kind() {
	case reflect.Float32, reflect.Float64:
		return field.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(field.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(field.Uint())
	case reflect.String:
		v, err := strconv.ParseFloat(field.String(), 64)
		require.NoError(t, err)
		return v
	case reflect.Pointer:
		if field.IsNil() {
			return 0
		}
		elem := field.Elem()
		switch elem.Kind() {
		case reflect.Float32, reflect.Float64:
			return elem.Float()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return float64(elem.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return float64(elem.Uint())
		}
	}
	t.Fatalf("field %s has unsupported kind %s for float getter", name, field.Kind())
	return 0
}

func getStringFieldByNameCandidates(t *testing.T, source any, names []string) string {
	t.Helper()
	field, name := mustFindReadableFieldByNames(t, source, names)
	switch field.Kind() {
	case reflect.String:
		return field.String()
	case reflect.Pointer:
		if field.IsNil() {
			return ""
		}
		if field.Elem().Kind() == reflect.String {
			return field.Elem().String()
		}
	}
	t.Fatalf("field %s has unsupported kind %s for string getter", name, field.Kind())
	return ""
}

func mustFindFieldByNames(t *testing.T, target any, names []string) (reflect.Value, string) {
	t.Helper()
	value := reflect.ValueOf(target)
	require.Equal(t, reflect.Pointer, value.Kind(), "target must be pointer")
	value = value.Elem()
	require.Equal(t, reflect.Struct, value.Kind(), "target must point to struct")

	for _, name := range names {
		field := value.FieldByName(name)
		if field.IsValid() {
			require.True(t, field.CanSet(), "field %s must be settable", name)
			return field, name
		}
	}
	t.Fatalf("expected one of fields %v on %T", names, target)
	return reflect.Value{}, ""
}

func mustFindReadableFieldByNames(t *testing.T, source any, names []string) (reflect.Value, string) {
	t.Helper()
	value := reflect.ValueOf(source)
	if value.Kind() == reflect.Pointer {
		require.False(t, value.IsNil(), "source pointer is nil")
		value = value.Elem()
	}
	require.Equal(t, reflect.Struct, value.Kind(), "source must be struct or pointer to struct")

	for _, name := range names {
		field := value.FieldByName(name)
		if field.IsValid() {
			return field, name
		}
	}
	t.Fatalf("expected one of fields %v on %T", names, source)
	return reflect.Value{}, ""
}
