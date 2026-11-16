package fqdp

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer lets the test read log output written by server goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func startTestServer(t *testing.T, limit int, window time.Duration) (string, *syncBuffer) {
	t.Helper()
	logs := &syncBuffer{}
	prev := log.Writer()
	log.SetOutput(logs)
	t.Cleanup(func() { log.SetOutput(prev) })

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		serve(ctx, listener, newConnLimiter(limit, window))
	}()
	t.Cleanup(func() { cancel(); <-done })
	return listener.Addr().String(), logs
}

// handshakeFrame is a minimal valid Handshake: version 0.1, zero-length token.
func handshakeFrame() []byte {
	payload := make([]byte, 9)
	payload[0] = byte(MessageTypeHandshake)
	payload[2] = 1
	frame := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(frame[0:4], uint32(len(payload)))
	copy(frame[4:], payload)
	return frame
}

// readErrorCode reads one Error frame and returns its code.
func readErrorCode(t *testing.T, conn net.Conn) uint16 {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		t.Fatalf("read error frame length: %v", err)
	}
	payload := make([]byte, binary.BigEndian.Uint32(lenBuf))
	if _, err := io.ReadFull(conn, payload); err != nil || len(payload) < 3 {
		t.Fatalf("read error frame payload (%d bytes): %v", len(payload), err)
	}
	if FQDPMessageType(payload[0]) != MessageTypeError {
		t.Fatalf("message type = %#x, want Error", payload[0])
	}
	return binary.BigEndian.Uint16(payload[1:3])
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func handshakeCode(t *testing.T, addr string) uint16 {
	t.Helper()
	conn := dial(t, addr)
	if _, err := conn.Write(handshakeFrame()); err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	return readErrorCode(t, conn)
}

func TestConnectionsUnderLimitKeepHandshakeBehavior(t *testing.T) {
	addr, _ := startTestServer(t, 3, time.Minute)
	for i := range 3 {
		if code := handshakeCode(t, addr); code != uint16(ErrHandshakeNotImplemented) {
			t.Fatalf("connection %d: code = %d, want %d", i, code, ErrHandshakeNotImplemented)
		}
	}
}

func TestConnectionOverLimitIsRejectedAndLogged(t *testing.T) {
	addr, logs := startTestServer(t, 2, time.Minute)
	for range 2 {
		handshakeCode(t, addr)
	}

	conn := dial(t, addr)
	if code := readErrorCode(t, conn); code != uint16(ErrRateLimited) {
		t.Fatalf("code = %d, want %d", code, ErrRateLimited)
	}
	if _, err := conn.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("read after reject = %v, want EOF (connection closed)", err)
	}
	if !strings.Contains(logs.String(), "rate limit") {
		t.Fatalf("breach not logged; logs:\n%s", logs.String())
	}

	// A second rejection in the same window is not logged again.
	if code := readErrorCode(t, dial(t, addr)); code != uint16(ErrRateLimited) {
		t.Fatalf("second reject code = %d, want %d", code, ErrRateLimited)
	}
	if n := strings.Count(logs.String(), "rate limit"); n != 1 {
		t.Fatalf("breach logged %d times, want 1; logs:\n%s", n, logs.String())
	}
}

func TestLimitResetsAfterWindow(t *testing.T) {
	addr, _ := startTestServer(t, 1, 50*time.Millisecond)
	handshakeCode(t, addr)
	time.Sleep(100 * time.Millisecond)
	if code := handshakeCode(t, addr); code != uint16(ErrHandshakeNotImplemented) {
		t.Fatalf("code after window = %d, want %d", code, ErrHandshakeNotImplemented)
	}
}

func TestLimitKeyGroupsIPv6By64(t *testing.T) {
	cases := map[string]string{
		"203.0.113.7":           "203.0.113.7",
		"::ffff:203.0.113.7":    "::ffff:203.0.113.7",
		"2001:db8:1:2:aaaa::1":  "2001:db8:1:2::",
		"2001:db8:1:2:bbbb::99": "2001:db8:1:2::",
		"2001:db8:1:3::1":       "2001:db8:1:3::",
	}
	for in, want := range cases {
		if got := limitKey(in); got != want {
			t.Errorf("limitKey(%q) = %q, want %q", in, got, want)
		}
	}
}
