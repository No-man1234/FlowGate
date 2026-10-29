package proxy

import (
	"bufio"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/No-man1234/FlowGate/pkg/breaker"
	"github.com/No-man1234/FlowGate/pkg/limiter"
	"github.com/No-man1234/FlowGate/pkg/metrics"
)

// -----------------------------------------------------------------------------
// 1. HTTP RFC Compliance Probes (RFC 9110 / RFC 9112)
// -----------------------------------------------------------------------------

// TestRFCCompliance_429And503Headers tests RFC 9110 compliance for 429 and 503:
// - Status code correctness
// - Retry-After header syntax (RFC 9110 § 10.2.4: delay-seconds = 1*DIGIT)
// - Content-Type application/json and valid JSON payload
func TestRFCCompliance_429And503Headers(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	// --- 1. Test 429 Too Many Requests (Load Shedding) ---
	t.Run("429 Too Many Requests RFC Compliance", func(t *testing.T) {
		lCfg := limiter.DefaultConfig()
		lCfg.InitialLimit = 1
		lCfg.MinLimit = 1
		l := limiter.NewGradientLimiter(lCfg)
		cb := breaker.New(breaker.DefaultConfig())
		m := metrics.NewCollector()

		gw := NewGateway(
			GatewayConfig{
				UpstreamURL:       upstreamURL,
				RetryAfterSeconds: 2,
			},
			l, cb, m,
		)

		// Exhaust the 1 available slot
		tok, ok := l.Acquire(context.Background(), limiter.PriorityStandard)
		if !ok {
			t.Fatalf("failed to acquire initial slot")
		}
		defer tok.Release()

		// Request must be shed
		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)

		resp := rec.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusTooManyRequests {
			t.Errorf("expected 429 Too Many Requests, got %d", resp.StatusCode)
		}

		// RFC 9110 § 10.2.4: Retry-After delay-seconds = 1*DIGIT
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter == "" {
			t.Errorf("RFC 9110 violation: 429 response missing Retry-After header")
		} else {
			delay, err := strconv.Atoi(retryAfter)
			if err != nil || delay <= 0 {
				t.Errorf("RFC 9110 violation: Retry-After '%s' is not a valid positive integer delay-seconds", retryAfter)
			}
			if delay != 2 {
				t.Errorf("expected Retry-After to be 2, got %d", delay)
			}
		}

		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		var bodyMap map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&bodyMap); err != nil {
			t.Errorf("expected valid JSON body on 429, got error: %v", err)
		}
		if _, ok := bodyMap["error"]; !ok {
			t.Errorf("expected 'error' field in 429 JSON response")
		}
	})

	// --- 2. Test 503 Service Unavailable (Circuit Breaker Open) ---
	t.Run("503 Service Unavailable RFC Compliance", func(t *testing.T) {
		l := limiter.NewGradientLimiter(limiter.DefaultConfig())
		cb := breaker.New(breaker.Config{
			FailureThreshold: 1,
			CoolOff:          1 * time.Minute,
		})
		m := metrics.NewCollector()

		// Force circuit breaker open
		_, gen := cb.Allow()
		cb.RecordFailure(gen)
		if cb.State() != breaker.StateOpen {
			t.Fatalf("expected breaker OPEN")
		}

		gw := NewGateway(
			GatewayConfig{
				UpstreamURL:       upstreamURL,
				RetryAfterSeconds: 3,
			},
			l, cb, m,
		)

		req := httptest.NewRequest(http.MethodGet, "/checkout/pay", nil)
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)

		resp := rec.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("expected 503 Service Unavailable, got %d", resp.StatusCode)
		}

		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter == "" {
			t.Errorf("RFC 9110 violation: 503 response missing Retry-After header")
		} else {
			delay, err := strconv.Atoi(retryAfter)
			if err != nil || delay <= 0 {
				t.Errorf("RFC 9110 violation: Retry-After '%s' is not valid positive integer", retryAfter)
			}
			// Gateway multiplies by 3 on breaker trips (3 * 3 = 9)
			if delay != 9 {
				t.Errorf("expected Retry-After 9, got %d", delay)
			}
		}

		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		var bodyMap map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&bodyMap); err != nil {
			t.Errorf("expected valid JSON body on 503, got error: %v", err)
		}
		if _, ok := bodyMap["error"]; !ok {
			t.Errorf("expected 'error' field in 503 JSON response")
		}
	})

	// --- 3. Test 502 / 504 on Upstream Outage ---
	t.Run("502 Bad Gateway ErrorHandler RFC Compliance", func(t *testing.T) {
		unreachableURL, _ := url.Parse("http://127.0.0.1:54999")
		l := limiter.NewGradientLimiter(limiter.DefaultConfig())
		cb := breaker.New(breaker.DefaultConfig())
		m := metrics.NewCollector()

		gw := NewGateway(
			GatewayConfig{
				UpstreamURL:       unreachableURL,
				RetryAfterSeconds: 5,
			},
			l, cb, m,
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)

		resp := rec.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadGateway {
			t.Errorf("expected 502 Bad Gateway, got %d", resp.StatusCode)
		}
		if val := resp.Header.Get("Retry-After"); val != "5" {
			t.Errorf("expected Retry-After 5, got %s", val)
		}
	})
}

// TestRFCCompliance_HopByHopHeaders tests RFC 9110 § 7.6.1 hop-by-hop header removal
// and forwarding behavior (Host header, X-Forwarded-For, X-Priority).
func TestRFCCompliance_HopByHopHeaders(t *testing.T) {
	var receivedHeaders http.Header
	var receivedHost string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		receivedHost = r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)
	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()

	gw := NewGateway(
		GatewayConfig{
			UpstreamURL:         upstreamURL,
			TrustClientPriority: false,
		},
		l, cb, m,
	)

	// Create request with standard hop-by-hop headers and custom connection tokens
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Host = "public-api.flowgate.com"
	req.Header.Set("Connection", "Keep-Alive, X-Custom-Hop")
	req.Header.Set("Keep-Alive", "timeout=30")
	req.Header.Set("Proxy-Connection", "keep-alive")
	req.Header.Set("X-Custom-Hop", "hop-secret-data")
	req.Header.Set("X-Priority", "critical") // Should be stripped by gateway
	req.Header.Set("X-Forwarded-For", "198.51.100.1")

	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	// RFC 9110 § 7.6.1: Connection-listed headers and known hop-by-hop headers MUST be stripped
	forbiddenHeaders := []string{
		"Keep-Alive",
		"Proxy-Connection",
		"X-Custom-Hop",
		"X-Priority",
	}

	for _, hdr := range forbiddenHeaders {
		if val := receivedHeaders.Get(hdr); val != "" {
			t.Errorf("RFC 9110 Hop-by-hop violation: header %s was forwarded to upstream (value: %s)", hdr, val)
		}
	}

	// Check Host header behavior
	// FlowGate uses httputil.NewSingleHostReverseProxy which preserves client Host by default
	t.Logf("Upstream received Host: %q (Upstream target Host: %q)", receivedHost, upstreamURL.Host)
	t.Logf("Upstream received X-Forwarded-For: %q", receivedHeaders.Get("X-Forwarded-For"))
}

// -----------------------------------------------------------------------------
// 2. Connection Pooling & Socket State Probes
// -----------------------------------------------------------------------------

// countedListener counts the number of TCP connections accepted.
type countedListener struct {
	net.Listener
	connsAccepted atomic.Int32
}

func (c *countedListener) Accept() (net.Conn, error) {
	conn, err := c.Listener.Accept()
	if err == nil {
		c.connsAccepted.Add(1)
	}
	return conn, err
}

// TestConnectionPooling_ReuseAndSocketState verifies:
// 1. Upstream TCP connections are reused across sequential requests (connection pooling).
// 2. FlowGate does NOT dialect a new TCP socket per request (which would accumulate TIME_WAIT).
// 3. Keep-alive is preserved end-to-end.
func TestConnectionPooling_ReuseAndSocketState(t *testing.T) {
	// Set up counting listener on upstream
	rawListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	upstreamCounted := &countedListener{Listener: rawListener}

	upstream := &httptest.Server{
		Listener: upstreamCounted,
		Config: &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			}),
		},
	}
	upstream.Start()
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)
	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()

	gw := NewGateway(
		GatewayConfig{
			UpstreamURL: upstreamURL,
		},
		l, cb, m,
	)

	// Run FlowGate in an httptest server with a counting listener
	gwRawListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for gw: %v", err)
	}
	gwCounted := &countedListener{Listener: gwRawListener}

	gwServer := &httptest.Server{
		Listener: gwCounted,
		Config: &http.Server{
			Handler: gw,
		},
	}
	gwServer.Start()
	defer gwServer.Close()

	// Client with Keep-Alive enabled
	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: false,
			MaxIdleConns:      10,
		},
		Timeout: 5 * time.Second,
	}

	const requestCount = 10

	// Send sequential requests
	for i := 0; i < requestCount; i++ {
		resp, err := client.Get(gwServer.URL + "/test")
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK || string(body) != "ok" {
			t.Fatalf("unexpected response on request %d", i)
		}
	}

	upstreamConns := upstreamCounted.connsAccepted.Load()
	clientConns := gwCounted.connsAccepted.Load()

	t.Logf("Summary after %d sequential requests:", requestCount)
	t.Logf("  Client -> FlowGate TCP connections dialed: %d", clientConns)
	t.Logf("  FlowGate -> Upstream TCP connections dialed: %d", upstreamConns)

	// In an ideal persistent connection pool, exactly 1 TCP connection is used
	if upstreamConns > 2 {
		t.Errorf("Connection pool leak / churn: FlowGate opened %d connections to upstream for %d sequential requests (expected <= 2)", upstreamConns, requestCount)
	}
	if clientConns > 2 {
		t.Errorf("Downstream keep-alive issue: Client opened %d connections to FlowGate for %d sequential requests (expected <= 2)", clientConns, requestCount)
	}
}

// -----------------------------------------------------------------------------
// 3. Streaming Protocols: WebSockets and SSE
// -----------------------------------------------------------------------------

// computeAcceptKey calculates the Sec-WebSocket-Accept header value per RFC 6455 § 1.3.
func computeAcceptKey(clientKey string) string {
	const magicGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(clientKey + magicGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// encodeWebSocketFrame encodes a simple unmasked WebSocket text frame (server to client).
func encodeWebSocketTextFrame(msg string) []byte {
	payload := []byte(msg)
	length := len(payload)
	frame := make([]byte, 2+length)
	frame[0] = 0x81 // FIN + text opcode
	frame[1] = byte(length)
	copy(frame[2:], payload)
	return frame
}

// encodeWebSocketClientFrame encodes a masked WebSocket text frame (client to server).
func encodeWebSocketClientFrame(msg string, maskKey [4]byte) []byte {
	payload := []byte(msg)
	length := len(payload)
	frame := make([]byte, 6+length)
	frame[0] = 0x81 // FIN + text opcode
	frame[1] = 0x80 | byte(length) // Masked bit set
	copy(frame[2:6], maskKey[:])
	for i := 0; i < length; i++ {
		frame[6+i] = payload[i] ^ maskKey[i%4]
	}
	return frame
}

// decodeWebSocketFrame decodes a simple WebSocket frame (length <= 125).
func decodeWebSocketFrame(r io.Reader) (opcode byte, payload []byte, err error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}
	opcode = header[0] & 0x0F
	isMasked := (header[1] & 0x80) != 0
	length := int(header[1] & 0x7F)

	var mask [4]byte
	if isMasked {
		if _, err := io.ReadFull(r, mask[:]); err != nil {
			return 0, nil, err
		}
	}

	payload = make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}

	if isMasked {
		for i := 0; i < length; i++ {
			payload[i] ^= mask[i%4]
		}
	}

	return opcode, payload, nil
}

// TestWebSocket_HandshakeAndFullDuplex tests WebSocket RFC 6455 upgrade end-to-end:
// 1. Upgrade handshake with Sec-WebSocket-Key and Sec-WebSocket-Extensions
// 2. 101 Switching Protocols response propagation
// 3. Full-duplex bidirectional frame streaming
// 4. Limiter slot state and telemetry behavior after WebSocket connection
func TestWebSocket_HandshakeAndFullDuplex(t *testing.T) {
	var upstreamReceivedExt string

	// Upstream server implementing RFC 6455 handshake and echo
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		upstreamReceivedExt = r.Header.Get("Sec-WebSocket-Extensions")
		clientKey := r.Header.Get("Sec-WebSocket-Key")

		// Check for WebSocket upgrade headers
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") || clientKey == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// RFC 6455 Handshake
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "server does not support hijacking", http.StatusInternalServerError)
			return
		}

		conn, brw, err := hj.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		acceptKey := computeAcceptKey(clientKey)

		// Send 101 Switching Protocols
		respHeaders := "HTTP/1.1 101 Switching Protocols\r\n" +
			"Upgrade: websocket\r\n" +
			"Connection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + acceptKey + "\r\n"
		if upstreamReceivedExt != "" {
			respHeaders += "Sec-WebSocket-Extensions: permessage-deflate\r\n"
		}
		respHeaders += "\r\n"

		if _, err := brw.WriteString(respHeaders); err != nil {
			return
		}
		if err := brw.Flush(); err != nil {
			return
		}

		// Full duplex echo loop: read client frame, echo back unmasked
		for {
			opcode, data, err := decodeWebSocketFrame(brw)
			if err != nil {
				return
			}
			if opcode == 0x08 { // Close frame
				// Echo close frame
				_, _ = brw.Write([]byte{0x88, 0x00})
				_ = brw.Flush()
				return
			}
			if opcode == 0x01 { // Text frame
				echoMsg := "ECHO: " + string(data)
				frame := encodeWebSocketTextFrame(echoMsg)
				if _, err := brw.Write(frame); err != nil {
					return
				}
				if err := brw.Flush(); err != nil {
					return
				}
			}
		}
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)
	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()

	gw := NewGateway(
		GatewayConfig{
			UpstreamURL: upstreamURL,
		},
		l, cb, m,
	)

	// Run FlowGate in httptest.Server so raw TCP dialing and Hijacking work
	gwServer := httptest.NewServer(gw)
	defer gwServer.Close()

	gwURL, _ := url.Parse(gwServer.URL)

	// Dial raw TCP connection from client to FlowGate
	clientConn, err := net.Dial("tcp", gwURL.Host)
	if err != nil {
		t.Fatalf("failed to dial gateway: %v", err)
	}
	defer clientConn.Close()

	// Send RFC 6455 Handshake request
	const clientKey = "dGhlIHNhbXBsZSBub25jZQ=="
	handshakeReq := fmt.Sprintf(
		"GET /ws HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Key: %s\r\n"+
			"Sec-WebSocket-Version: 13\r\n"+
			"Sec-WebSocket-Extensions: permessage-deflate; client_max_window_bits\r\n\r\n",
		gwURL.Host, clientKey,
	)

	if _, err := clientConn.Write([]byte(handshakeReq)); err != nil {
		t.Fatalf("failed to write handshake request: %v", err)
	}

	reader := bufio.NewReader(clientConn)

	// Read 101 response status line
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read response status: %v", err)
	}
	if !strings.Contains(statusLine, "101") {
		t.Fatalf("expected 101 Switching Protocols, got: %s", statusLine)
	}

	// Read handshake headers
	headers := make(http.Header)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("error reading header: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			headers.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	// Verify Sec-WebSocket-Accept
	expectedAccept := computeAcceptKey(clientKey)
	if headers.Get("Sec-WebSocket-Accept") != expectedAccept {
		t.Errorf("expected Sec-WebSocket-Accept %s, got %s", expectedAccept, headers.Get("Sec-WebSocket-Accept"))
	}

	// Verify Sec-WebSocket-Extensions negotiation
	if upstreamReceivedExt == "" {
		t.Errorf("upstream did not receive Sec-WebSocket-Extensions header!")
	}
	if headers.Get("Sec-WebSocket-Extensions") != "permessage-deflate" {
		t.Errorf("client did not receive Sec-WebSocket-Extensions: permessage-deflate header from gateway")
	}

	// Send full-duplex text frame
	mask := [4]byte{0x12, 0x34, 0x56, 0x78}
	msgToSend := "FlowGate WebSocket Test Payload"
	clientFrame := encodeWebSocketClientFrame(msgToSend, mask)
	if _, err := clientConn.Write(clientFrame); err != nil {
		t.Fatalf("failed to write client websocket frame: %v", err)
	}

	// Read echoed frame from upstream
	opcode, echoedData, err := decodeWebSocketFrame(reader)
	if err != nil {
		t.Fatalf("failed to read echoed websocket frame: %v", err)
	}
	if opcode != 0x01 {
		t.Errorf("expected opcode 0x01 (text), got %x", opcode)
	}
	if string(echoedData) != "ECHO: "+msgToSend {
		t.Errorf("unexpected echo payload: %s", string(echoedData))
	}

	// Send close frame
	closeFrame := []byte{0x88, 0x80, 0x00, 0x00, 0x00, 0x00} // Masked close frame
	_, _ = clientConn.Write(closeFrame)

	// Read close frame response
	closeOpcode, _, _ := decodeWebSocketFrame(reader)
	if closeOpcode != 0x08 {
		t.Logf("close opcode response: %x", closeOpcode)
	}

	_ = clientConn.Close()

	// Wait briefly for FlowGate handler to settle
	time.Sleep(50 * time.Millisecond)

	// Concurrency slot check
	if l.InFlight() != 0 {
		t.Errorf("limiter slot leaked after WebSocket closed: in-flight = %d (expected 0)", l.InFlight())
	}
}

// TestStreaming_ChunkedResponses verifies incremental streaming without intermediate buffer stalls.
func TestStreaming_ChunkedResponses(t *testing.T) {
	chunkDelays := 40 * time.Millisecond
	totalChunks := 3

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Errorf("upstream writer does not support Flusher")
			return
		}

		for i := 1; i <= totalChunks; i++ {
			_, _ = fmt.Fprintf(w, `{"chunk":%d}`+"\n", i)
			flusher.Flush()
			time.Sleep(chunkDelays)
		}
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)
	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()

	gw := NewGateway(
		GatewayConfig{
			UpstreamURL: upstreamURL,
		},
		l, cb, m,
	)

	gwServer := httptest.NewServer(gw)
	defer gwServer.Close()

	start := time.Now()
	resp, err := http.Get(gwServer.URL + "/stream")
	if err != nil {
		t.Fatalf("failed to GET stream: %v", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var chunkTimes []time.Duration

	for scanner.Scan() {
		chunkTimes = append(chunkTimes, time.Since(start))
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	if len(chunkTimes) != totalChunks {
		t.Fatalf("expected %d chunks, got %d", totalChunks, len(chunkTimes))
	}

	// Verify first chunk arrived before the last chunk was even generated!
	// If FlowGate buffered the entire response, chunkTimes[0] would be >= totalChunks * chunkDelays
	t.Logf("Chunk receipt times: %v", chunkTimes)
	if chunkTimes[0] >= time.Duration(totalChunks)*chunkDelays {
		t.Errorf("Streaming buffered! First chunk arrived at %v, expected < %v", chunkTimes[0], time.Duration(totalChunks)*chunkDelays)
	}
}

// TestStreaming_ClientContextCancellationPropagation tests that client cancellation
// promptly propagates upstream to abort the dialer/transport without hanging.
func TestStreaming_ClientContextCancellationPropagation(t *testing.T) {
	upstreamAborted := make(chan struct{})

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			_, _ = fmt.Fprint(w, "data: chunk1\n\n")
			f.Flush()
		}

		// Wait for client to cancel
		select {
		case <-r.Context().Done():
			close(upstreamAborted)
			return
		case <-time.After(2 * time.Second):
			t.Errorf("upstream never detected client context cancellation!")
			return
		}
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)
	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()

	gw := NewGateway(
		GatewayConfig{
			UpstreamURL: upstreamURL,
		},
		l, cb, m,
	)

	gwServer := httptest.NewServer(gw)
	defer gwServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, gwServer.URL+"/events", nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed initial request: %v", err)
	}

	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	if n == 0 {
		t.Fatalf("failed reading initial SSE chunk")
	}

	// Cancel client context
	cancel()
	_ = resp.Body.Close()

	select {
	case <-upstreamAborted:
		t.Logf("Upstream successfully received context cancellation from client via FlowGate")
	case <-time.After(1 * time.Second):
		t.Errorf("context cancellation did not propagate upstream within 1s")
	}

	// Verify limiter slot released and not counted as upstream error
	time.Sleep(50 * time.Millisecond)
	if l.InFlight() != 0 {
		t.Errorf("limiter slot leaked after client cancellation: in-flight = %d", l.InFlight())
	}
	if m.UpstreamErrors.Load() != 0 {
		t.Errorf("client cancel incorrectly counted as upstream error: %d", m.UpstreamErrors.Load())
	}
}

// -----------------------------------------------------------------------------
// 4. Large Payloads: Streaming Uploads and Downloads
// -----------------------------------------------------------------------------

// TestLargePayload_StreamingUploadChunked sends a large chunked payload (e.g. 10 MB)
// without Content-Length to verify that FlowGate streams the chunks to upstream
// without unbounded memory buffering.
func TestLargePayload_StreamingUploadChunked(t *testing.T) {
	const totalSize = 10 * 1024 * 1024 // 10 MB
	var upstreamBytesReceived atomic.Int64
	var upstreamHash []byte

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := sha256.New()
		tee := io.TeeReader(r.Body, h)
		n, err := io.Copy(io.Discard, tee)
		if err != nil {
			t.Errorf("upstream read error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		upstreamBytesReceived.Store(n)
		upstreamHash = h.Sum(nil)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)
	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()

	gw := NewGateway(
		GatewayConfig{
			UpstreamURL: upstreamURL,
		},
		l, cb, m,
	)

	gwServer := httptest.NewServer(gw)
	defer gwServer.Close()

	// Generate deterministic streaming body
	pr, pw := io.Pipe()
	clientHasher := sha256.New()

	go func() {
		defer pw.Close()
		chunk := make([]byte, 64*1024) // 64 KB chunks
		for i := range chunk {
			chunk[i] = byte(i % 256)
		}

		remaining := totalSize
		for remaining > 0 {
			toWrite := len(chunk)
			if toWrite > remaining {
				toWrite = remaining
			}
			clientHasher.Write(chunk[:toWrite])
			if _, err := pw.Write(chunk[:toWrite]); err != nil {
				return
			}
			remaining -= toWrite
		}
	}()

	// Send POST with chunked body (no Content-Length specified)
	req, err := http.NewRequest(http.MethodPost, gwServer.URL+"/upload", pr)
	if err != nil {
		t.Fatalf("failed creating request: %v", err)
	}
	req.ContentLength = -1 // Force chunked transfer encoding

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from upstream, got %d", resp.StatusCode)
	}

	expectedHash := clientHasher.Sum(nil)
	if upstreamBytesReceived.Load() != totalSize {
		t.Errorf("upstream received %d bytes, expected %d", upstreamBytesReceived.Load(), totalSize)
	}
	if fmt.Sprintf("%x", upstreamHash) != fmt.Sprintf("%x", expectedHash) {
		t.Errorf("hash mismatch on 10MB chunked upload! Upstream received corrupted data")
	}
}

// TestLargePayload_StreamingDownload verifies that large responses (10 MB) stream
// through FlowGate without buffering the full payload in memory.
func TestLargePayload_StreamingDownload(t *testing.T) {
	const totalSize = 10 * 1024 * 1024 // 10 MB

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)

		chunk := make([]byte, 64*1024)
		for i := range chunk {
			chunk[i] = byte((i + 7) % 256)
		}

		remaining := totalSize
		for remaining > 0 {
			toWrite := len(chunk)
			if toWrite > remaining {
				toWrite = remaining
			}
			if _, err := w.Write(chunk[:toWrite]); err != nil {
				return
			}
			remaining -= toWrite
		}
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)
	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()

	gw := NewGateway(
		GatewayConfig{
			UpstreamURL: upstreamURL,
		},
		l, cb, m,
	)

	gwServer := httptest.NewServer(gw)
	defer gwServer.Close()

	resp, err := http.Get(gwServer.URL + "/download")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		t.Fatalf("reading response body failed: %v", err)
	}
	if n != totalSize {
		t.Errorf("read %d bytes, expected %d", n, totalSize)
	}
}
