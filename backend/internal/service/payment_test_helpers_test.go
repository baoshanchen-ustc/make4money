package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func newPaymentOrderEntClient(t *testing.T) *dbent.Client {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name())
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func createPaymentOrderUserForTest(t *testing.T, client *dbent.Client, now time.Time, email string) *dbent.User {
	t.Helper()

	user, err := client.User.Create().
		SetCreatedAt(now).
		SetUpdatedAt(now).
		SetEmail(email).
		SetPasswordHash("hashed-password").
		SetSignupSource("email").
		SetRole("user").
		SetBalance(0).
		SetConcurrency(1).
		SetStatus("active").
		SetUsername("user").
		SetNotes("").
		SetTotpEnabled(false).
		SetBalanceNotifyEnabled(false).
		SetBalanceNotifyThresholdType("").
		SetBalanceNotifyExtraEmails("[]").
		SetTotalRecharged(0).
		Save(context.Background())
	require.NoError(t, err)
	return user
}

func createPaymentProviderInstanceForTest(t *testing.T, client *dbent.Client, providerKey, supportedTypes, limits string) *dbent.PaymentProviderInstance {
	t.Helper()

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(providerKey).
		SetName(providerKey + "-instance").
		SetConfig("{}").
		SetSupportedTypes(supportedTypes).
		SetEnabled(true).
		SetLimits(limits).
		Save(context.Background())
	require.NoError(t, err)
	return inst
}
