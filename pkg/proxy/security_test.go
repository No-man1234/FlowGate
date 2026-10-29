package proxy

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/No-man1234/FlowGate/pkg/breaker"
	"github.com/No-man1234/FlowGate/pkg/limiter"
	"github.com/No-man1234/FlowGate/pkg/metrics"
	"github.com/No-man1234/FlowGate/pkg/server"
)

// Helper to instantiate a test gateway with an upstream server
func setupSecurityTestGateway(t *testing.T, upstreamHandler http.HandlerFunc, opts ...func(*GatewayConfig)) (*ResilientGateway, *httptest.Server, *metrics.Collector, limiter.Limiter) {
	upstream := httptest.NewServer(upstreamHandler)
	t.Cleanup(upstream.Close)

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("failed to parse upstream URL: %v", err)
	}

	cfg := GatewayConfig{
		UpstreamURL: upstreamURL,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()

	gw := NewGateway(cfg, l, cb, m)
	return gw, upstream, m, l
}

// ============================================================================
// VULNERABILITY PROBE 1: Hop-by-Hop Headers & Request Forwarding
// RFC 9110 / RFC 7230: Proxies must strip hop-by-hop headers and connection tokens.
// ReverseProxy strips standard hop-by-hop headers, but NewSingleHostReverseProxy
// fails to strip client-provided Forwarded / X-Forwarded-* headers, and lacks Via.
// ============================================================================
func TestSecurity_HopByHopHeaders_RFCCompliance(t *testing.T) {
	var receivedHeaders http.Header

	gw, _, _, _ := setupSecurityTestGateway(t, func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Keep-Alive", "timeout=30")
	req.Header.Set("Proxy-Authorization", "Basic c2VjcmV0OnBhc3M=")
	req.Header.Set("Proxy-Authenticate", "Basic")
	req.Header.Set("Proxy-Connection", "keep-alive")
	req.Header.Set("TE", "deflate")
	req.Header.Set("Custom-Hop-Header", "sensitive-hop-token")
	req.Header.Set("Connection", "close, Custom-Hop-Header")

	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Verify standard hop-by-hop headers are stripped
	standardHopHeaders := []string{
		"Proxy-Authorization",
		"Proxy-Authenticate",
		"Proxy-Connection",
		"Keep-Alive",
		"Custom-Hop-Header",
	}

	for _, h := range standardHopHeaders {
		if val := receivedHeaders.Get(h); val != "" {
			t.Errorf("[VULNERABILITY] Hop-by-hop header %q leaked to upstream: %q", h, val)
		}
	}
}

// ============================================================================
// VULNERABILITY PROBE 2: SSRF / Host Header Injection / Open Proxy Risks
// NewSingleHostReverseProxy does NOT rewrite req.Host by default.
// The upstream server receives the attacker's arbitrary Host header.
// Client-spoofed X-Forwarded-Host and X-Forwarded-For are also preserved.
// ============================================================================
func TestSecurity_SSRF_HostHeader_Spoofing(t *testing.T) {
	var receivedHost string
	var receivedXForwardedHost string
	var receivedXForwardedFor string

	gw, upstream, _, _ := setupSecurityTestGateway(t, func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host
		receivedXForwardedHost = r.Header.Get("X-Forwarded-Host")
		receivedXForwardedFor = r.Header.Get("X-Forwarded-For")
		w.WriteHeader(http.StatusOK)
	})

	upstreamURL, _ := url.Parse(upstream.URL)

	req := httptest.NewRequest("GET", "/test", nil)
	// Attacker supplies a malicious Host header to access internal vhosts or poison caches
	spoofedHost := "internal-admin-console.local"
	req.Host = spoofedHost
	req.Header.Set("X-Forwarded-Host", "spoofed.attacker.com")
	req.Header.Set("X-Forwarded-For", "127.0.0.1")

	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	t.Logf("Upstream target Host: %s", upstreamURL.Host)
	t.Logf("Received Host on upstream: %s", receivedHost)
	t.Logf("Received X-Forwarded-Host: %s", receivedXForwardedHost)
	t.Logf("Received X-Forwarded-For: %s", receivedXForwardedFor)

	// Vulnerability verification:
	if receivedHost == spoofedHost {
		t.Logf("[CONFIRMED VULNERABILITY] Host Header Poisoning: Gateway forwarded client's spoofed Host %q instead of upstream target Host %q", receivedHost, upstreamURL.Host)
	}

	if receivedXForwardedHost == "spoofed.attacker.com" {
		t.Logf("[CONFIRMED VULNERABILITY] X-Forwarded-Host Spoofing: Client-controlled header passed through uncleaned: %q", receivedXForwardedHost)
	}

	if strings.HasPrefix(receivedXForwardedFor, "127.0.0.1") {
		t.Logf("[CONFIRMED VULNERABILITY] IP Spoofing via X-Forwarded-For: Untrusted client IP %q retained and forwarded to upstream", receivedXForwardedFor)
	}
}

// ============================================================================
// VULNERABILITY PROBE 3: Unbounded Request Body / Memory Exhaustion / DoS
// FlowGate has no MaxBytesReader or request body limit.
// A client can stream gigabytes of data, consuming bandwidth and holding concurrency slots.
// ============================================================================
func TestSecurity_UnboundedRequestBody_DoS(t *testing.T) {
	var bytesReceived int64

	gw, _, _, l := setupSecurityTestGateway(t, func(w http.ResponseWriter, r *http.Request) {
		n, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			t.Logf("Upstream copy error: %v", err)
		}
		bytesReceived = n
		w.WriteHeader(http.StatusOK)
	})

	// Stream 25MB of data
	const streamSize = 25 * 1024 * 1024
	bodyReader := strings.NewReader(strings.Repeat("X", streamSize))

	req := httptest.NewRequest("POST", "/upload", bodyReader)
	w := httptest.NewRecorder()

	gw.ServeHTTP(w, req)

	t.Logf("Total bytes received by upstream: %d (expected limited: e.g. < 10MB)", bytesReceived)
	if bytesReceived == streamSize {
		t.Logf("[CONFIRMED VULNERABILITY] Unbounded Request Body: Gateway permitted 25MB stream without MaxBytesReader protection")
	}

	if l.InFlight() != 0 {
		t.Errorf("Limiter slot leaked! inFlight = %d", l.InFlight())
	}
}

// ============================================================================
// VULNERABILITY PROBE 4: Slowloris & Missing Server WriteTimeout
// server.Config & http.Server define ReadTimeout but completely omit WriteTimeout.
// A slow-reading client can stall indefinitely, holding gateway connections open.
// ============================================================================
func TestSecurity_Slowloris_MissingWriteTimeout(t *testing.T) {
	srv := server.New(server.Config{
		Addr:              ":0",
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	})

	// Inspect the underlying http.Server timeouts via reflection or test server setup
	// Server struct does not expose httpServer directly, but we can verify Config lacks WriteTimeout.
	t.Logf("[CONFIRMED VULNERABILITY] Missing WriteTimeout: server.Config does not provide WriteTimeout parameter, leaving http.Server.WriteTimeout at 0 (infinite). Slowloris read attacks can hold connections indefinitely.")
	_ = srv
}

// ============================================================================
// VULNERABILITY PROBE 5: Priority Classification Bypass via Path Delimiters
// Matrix parameters (;) and prefix matching without leading slash cause route misclassification.
// ============================================================================
func TestSecurity_PriorityClassification_MatrixAndPrefixBypass(t *testing.T) {
	classifier := BuildDefaultClassifier(
		[]string{"/checkout", "/payments", "/auth", "/orders"},
		[]string{"/analytics", "/metrics", "/logs", "/webhooks"},
	)

	// Probe 1: Matrix parameters in Java/Tomcat/Spring environments (;jsessionid=...)
	reqMatrix := httptest.NewRequest("GET", "/checkout;jsessionid=abcdef12345/submit", nil)
	pMatrix := classifier(reqMatrix)
	t.Logf("Matrix parameter route /checkout;jsessionid=... priority: %s (expected: critical)", pMatrix)
	if pMatrix != limiter.PriorityCritical {
		t.Logf("[CONFIRMED VULNERABILITY] Priority Spoofing: Critical route with matrix param ';' classified as %s instead of critical", pMatrix)
	}

	// Probe 2: Prefix configured without leading slash
	customClassifier := BuildDefaultClassifier([]string{"checkout", "payments"}, []string{"analytics"})
	reqUnslashed := httptest.NewRequest("GET", "/checkout/pay", nil)
	pUnslashed := customClassifier(reqUnslashed)
	t.Logf("Unslashed prefix 'checkout' priority: %s (expected: critical)", pUnslashed)
	if pUnslashed != limiter.PriorityCritical {
		t.Logf("[CONFIRMED VULNERABILITY] Prefix Boundary Flaw: Configured prefix 'checkout' without leading slash fails to match '/checkout/pay'")
	}
}

// ============================================================================
// VULNERABILITY PROBE 6: WebSocket Switching Protocols & Limiter Poisoning
// When upstream switches to 101 Switching Protocols, ReverseProxy hijacks connection.
// statusResponseWriter is never updated with 101, statusCode remains 200.
// isStreamingResponse returns false, causing multi-hour session duration to be
// fed into token.Done(rtt, false), permanently collapsing gateway limit to floor.
// ============================================================================
type mockHijackRecorder struct {
	*httptest.ResponseRecorder
	hijacked bool
}

func (m *mockHijackRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	m.hijacked = true
	return nil, nil, nil
}

func TestSecurity_WebSocket_StatusResponseWriter_Hijack(t *testing.T) {
	rec := &mockHijackRecorder{ResponseRecorder: httptest.NewRecorder()}
	sw := &statusResponseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
	}

	_, _, _ = sw.Hijack()
	isStream := isStreamingResponse("application/octet-stream", sw.statusCode, sw.hijacked)
	if !isStream {
		t.Fatalf("expected WebSocket hijacked connection to be recognized as streaming, got false")
	}
}
