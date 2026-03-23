package admin

import (
	"encoding/base64"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ssutil"
	"github.com/stretchr/testify/require"
)

func TestParseSubscriptionText_SupportsStandardAndLegacySS(t *testing.T) {
	standard, err := ssutil.BuildURL("aes-256-gcm", "secret", "ss.example.com", 8388, "std")
	require.NoError(t, err)
	withPlugin, err := ssutil.BuildURL("aes-256-gcm", "secret", "plugin.example.com", 8388, "")
	require.NoError(t, err)

	legacyPayload := base64.RawURLEncoding.EncodeToString([]byte("aes-128-gcm:pass@legacy.example.com:443"))
	legacy := "ss://" + legacyPayload + "#legacy"

	proxies, err := parseSubscriptionText(standard + "\n" + legacy + "\n" + withPlugin + "?plugin=obfs-local")
	require.NoError(t, err)
	require.Len(t, proxies, 2)

	require.Equal(t, "ss", proxies[0].Protocol)
	require.Equal(t, "std", proxies[0].Name)
	require.Equal(t, "aes-256-gcm", proxies[0].Username)
	require.Equal(t, "secret", proxies[0].Password)

	require.Equal(t, "ss", proxies[1].Protocol)
	require.Equal(t, "legacy", proxies[1].Name)
	require.Equal(t, "legacy.example.com", proxies[1].Host)
	require.Equal(t, 443, proxies[1].Port)
	require.Equal(t, "aes-128-gcm", proxies[1].Username)
	require.Equal(t, "pass", proxies[1].Password)
}

func TestParseClashSubscriptionYAML_SSOnly(t *testing.T) {
	body := []byte(`
proxies:
  - name: ss-ok
    type: ss
    server: ss.example.com
    port: 8388
    cipher: aes-256-gcm
    password: secret
  - name: ss-plugin
    type: ss
    server: plugin.example.com
    port: 443
    cipher: aes-128-gcm
    password: blocked
    plugin: obfs-local
  - name: vmess-skip
    type: vmess
    server: vmess.example.com
    port: 443
`)

	proxies, err := parseClashSubscriptionYAML(body)
	require.NoError(t, err)
	require.Len(t, proxies, 1)
	require.Equal(t, "ss-ok", proxies[0].Name)
	require.Equal(t, "ss", proxies[0].Protocol)
	require.Equal(t, "aes-256-gcm", proxies[0].Username)
	require.Equal(t, "secret", proxies[0].Password)
}
