package repository

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProxyProbeServiceSuite struct {
	suite.Suite
	ctx      context.Context
	proxySrv *httptest.Server
	prober   *proxyProbeService
	oldURLs  []struct {
		url    string
		parser string
	}
}

func (s *ProxyProbeServiceSuite) SetupTest() {
	s.ctx = context.Background()
	s.prober = &proxyProbeService{
		allowPrivateHosts: true,
	}
	s.oldURLs = append([]struct {
		url    string
		parser string
	}(nil), probeURLs...)
	probeURLs = []struct {
		url    string
		parser string
	}{
		{"http://api.country.is/?fields=city,subdivision", "country-is"},
		{"http://ifconfig.co/json", "ifconfig"},
	}
}

func (s *ProxyProbeServiceSuite) TearDownTest() {
	probeURLs = s.oldURLs
	if s.proxySrv != nil {
		s.proxySrv.Close()
		s.proxySrv = nil
	}
}

func (s *ProxyProbeServiceSuite) setupProxyServer(handler http.HandlerFunc) {
	s.proxySrv = newLocalTestServer(s.T(), handler)
}

func (s *ProxyProbeServiceSuite) TestProbeProxy_InvalidProxyURL() {
	_, _, err := s.prober.ProbeProxy(s.ctx, "://bad")
	require.Error(s.T(), err)
	require.ErrorContains(s.T(), err, "failed to create proxy client")
}

func (s *ProxyProbeServiceSuite) TestProbeProxy_UnsupportedProxyScheme() {
	_, _, err := s.prober.ProbeProxy(s.ctx, "ftp://127.0.0.1:1")
	require.Error(s.T(), err)
	require.ErrorContains(s.T(), err, "failed to create proxy client")
}

func (s *ProxyProbeServiceSuite) TestProbeProxy_Success_CountryIs() {
	s.setupProxyServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.RequestURI, "api.country.is") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ip":"1.2.3.4","country":"CC","city":"c","subdivision":"r"}`)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	info, latencyMs, err := s.prober.ProbeProxy(s.ctx, s.proxySrv.URL)
	require.NoError(s.T(), err, "ProbeProxy")
	require.GreaterOrEqual(s.T(), latencyMs, int64(0), "unexpected latency")
	require.Equal(s.T(), "1.2.3.4", info.IP)
	require.Equal(s.T(), "c", info.City)
	require.Equal(s.T(), "r", info.Region)
	require.Equal(s.T(), "CC", info.Country)
	require.Equal(s.T(), "CC", info.CountryCode)
}

func (s *ProxyProbeServiceSuite) TestProbeProxy_Success_IfConfigFallback() {
	s.setupProxyServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.RequestURI, "api.country.is") {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if strings.Contains(r.RequestURI, "ifconfig.co/json") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ip":"5.6.7.8","city":"fallback-city","country":"Fallback","country_iso":"FB"}`)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	info, latencyMs, err := s.prober.ProbeProxy(s.ctx, s.proxySrv.URL)
	require.NoError(s.T(), err, "ProbeProxy should fallback to ifconfig")
	require.GreaterOrEqual(s.T(), latencyMs, int64(0), "unexpected latency")
	require.Equal(s.T(), "5.6.7.8", info.IP)
	require.Equal(s.T(), "fallback-city", info.City)
	require.Equal(s.T(), "Fallback", info.Country)
	require.Equal(s.T(), "FB", info.CountryCode)
}

func (s *ProxyProbeServiceSuite) TestProbeProxy_AllFailed() {
	s.setupProxyServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	_, _, err := s.prober.ProbeProxy(s.ctx, s.proxySrv.URL)
	require.Error(s.T(), err)
	require.ErrorContains(s.T(), err, "all probe URLs failed")
}

func (s *ProxyProbeServiceSuite) TestProbeProxy_InvalidJSON() {
	s.setupProxyServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.RequestURI, "api.country.is") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, "not-json")
			return
		}
		if strings.Contains(r.RequestURI, "ifconfig.co/json") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, "not-json")
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	_, _, err := s.prober.ProbeProxy(s.ctx, s.proxySrv.URL)
	require.Error(s.T(), err)
	require.ErrorContains(s.T(), err, "all probe URLs failed")
}

func (s *ProxyProbeServiceSuite) TestProbeProxy_ProxyServerClosed() {
	s.setupProxyServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	s.proxySrv.Close()

	_, _, err := s.prober.ProbeProxy(s.ctx, s.proxySrv.URL)
	require.Error(s.T(), err, "expected error when proxy server is closed")
}

func (s *ProxyProbeServiceSuite) TestParseCountryIs_Success() {
	body := []byte(`{"ip":"1.2.3.4","country":"CN","city":"Beijing","subdivision":"Beijing"}`)
	info, latencyMs, err := s.prober.parseCountryIs(body, 100)
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(100), latencyMs)
	require.Equal(s.T(), "1.2.3.4", info.IP)
	require.Equal(s.T(), "Beijing", info.City)
	require.Equal(s.T(), "Beijing", info.Region)
	require.Equal(s.T(), "CN", info.Country)
	require.Equal(s.T(), "CN", info.CountryCode)
}

func (s *ProxyProbeServiceSuite) TestParseCountryIs_Failure() {
	body := []byte(`{"success":false,"message":"rate limited"}`)
	_, _, err := s.prober.parseCountryIs(body, 100)
	require.Error(s.T(), err)
	require.ErrorContains(s.T(), err, "rate limited")
}

func (s *ProxyProbeServiceSuite) TestParseIfConfig_Success() {
	body := []byte(`{"ip":"9.8.7.6","city":"Paris","country":"France","country_iso":"FR"}`)
	info, latencyMs, err := s.prober.parseIfConfig(body, 50)
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(50), latencyMs)
	require.Equal(s.T(), "9.8.7.6", info.IP)
	require.Equal(s.T(), "Paris", info.City)
	require.Equal(s.T(), "France", info.Country)
	require.Equal(s.T(), "FR", info.CountryCode)
}

func (s *ProxyProbeServiceSuite) TestParseIfConfig_NoIP() {
	body := []byte(`{"ip": ""}`)
	_, _, err := s.prober.parseIfConfig(body, 50)
	require.Error(s.T(), err)
	require.ErrorContains(s.T(), err, "no IP found")
}

func TestProxyProbeServiceSuite(t *testing.T) {
	suite.Run(t, new(ProxyProbeServiceSuite))
}
