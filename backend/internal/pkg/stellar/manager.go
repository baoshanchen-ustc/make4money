// Package stellar provides direct dial support for stellar-proxy tunnels.
//
// Instead of spawning a local HTTP proxy server, Dial opens a TLS connection
// directly to the stellar-proxy server, sends the token + target address, and
// returns the tunneled net.Conn. This conn is used as transport.DialContext so
// all HTTP traffic goes straight through the stellar-proxy server.
package stellar

import (
	"context"
	"fmt"
	"net"
	"sync"

	spcore "github.com/ruilisi/stellar-proxy/core"
	"github.com/ruilisi/stellar-proxy/socks"
)

// configCache caches TCPClientConfig per (server|sn|token) key to avoid
// repeated token parsing on every dial.
var (
	cacheMu sync.Mutex
	cache   = map[string]*spcore.TCPClientConfig{}
)

func cacheKey(server, token, sn string) string {
	return server + "|" + sn + "|" + token
}

// getConfig returns a cached *TCPClientConfig, creating one if needed.
func getConfig(server, token, sn string) (*spcore.TCPClientConfig, error) {
	if sn == "" {
		sn = "cloudflare.com"
	}
	key := cacheKey(server, token, sn)

	cacheMu.Lock()
	if cfg, ok := cache[key]; ok {
		cacheMu.Unlock()
		return cfg, nil
	}
	cacheMu.Unlock()

	cfg, err := spcore.NewTCPClientConfig(server, sn, token, nil)
	if err != nil {
		return nil, fmt.Errorf("stellar: create config: %w", err)
	}

	cacheMu.Lock()
	// Double-check after acquiring lock
	if existing, ok := cache[key]; ok {
		cacheMu.Unlock()
		return existing, nil
	}
	cache[key] = cfg
	cacheMu.Unlock()

	return cfg, nil
}

// Dial opens a direct tunnel through a stellar-proxy server to targetAddr
// (host:port). It connects to the stellar server via TLS, writes the
// authentication token + socks-encoded target address, then returns the
// connection — which is now transparently tunneled to the target.
func Dial(ctx context.Context, server, token, sn, targetAddr string) (net.Conn, error) {
	cfg, err := getConfig(server, token, sn)
	if err != nil {
		return nil, err
	}

	tgt := socks.ParseAddr(targetAddr)
	if tgt == nil {
		return nil, fmt.Errorf("stellar: failed to parse target address: %s", targetAddr)
	}

	dialer := &net.Dialer{}
	pc, _, err := spcore.TLSDialerHandshaked(dialer, "tcp", cfg.Server(), cfg.TLSConf())
	if err != nil {
		return nil, fmt.Errorf("stellar: TLS dial to %s failed: %w", cfg.Server(), err)
	}

	// Send token + socks-encoded target in one write (mirrors SendTCPViaProxy)
	header := append(cfg.TokenBytes(), tgt...)
	if _, err = pc.Write(header); err != nil {
		pc.Close()
		return nil, fmt.Errorf("stellar: send header failed: %w", err)
	}

	return pc, nil
}
