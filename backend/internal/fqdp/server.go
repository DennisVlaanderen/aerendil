// Package fqdp implements the FQDP binary protocol server described in
// docs/fqdp.md. Currently a stub: only Handshake is parsed; no session,
// Query, Subscribe, or Update handling exists yet. New connections are
// rate-limited per IP; per-session request limits (§12) await real sessions.
package fqdp

import (
	"bufio"
	"context"
	"encoding/binary"
	stdErrors "errors"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

const (
	// DefaultFQDPAddr is the address StartTCPServer listens on when the
	// caller doesn't override it.
	DefaultFQDPAddr = ":9001"
	// MaximumFrameSize caps a frame's payload so a bad length prefix can't
	// force an unbounded allocation in readFrame.
	MaximumFrameSize = 10 << 20 // 10 MiB
)

// StartTCPServer serves FQDP connections on addr, each on its own
// goroutine, until ctx is canceled. Each source IP may open at most limit
// new connections per window.
func StartTCPServer(ctx context.Context, addr string, limit int, window time.Duration) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	log.Printf("fqdp tcp server listening on %s", addr)
	serve(ctx, listener, newConnLimiter(limit, window))
	return nil
}

func serve(ctx context.Context, listener net.Listener, limiter *connLimiter) {
	defer listener.Close()

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("accept error: %v", err)
				continue
			}
		}
		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		if n := limiter.count(limitKey(ip)); n > limiter.limit {
			// Log only the first breach per window so a flood can't flood the logs.
			if n == limiter.limit+1 {
				log.Printf("fqdp connection rate limit exceeded for %s; closing further connections this window", ip)
			}
			sendError(conn, uint16(ErrRateLimited), "connection rate limit exceeded")
			conn.Close()
			continue
		}
		go serveConnection(conn)
	}
}

// limitKey groups IPv6 addresses by /64, since one host usually owns a whole
// /64 and could otherwise pick a fresh source address per connection.
func limitKey(ip string) string {
	if p := net.ParseIP(ip); p != nil && p.To4() == nil {
		return p.Mask(net.CIDRMask(64, 128)).String()
	}
	return ip
}

// connLimiter counts new connections per IP in fixed windows.
// shortcut: fixed window allows up to 2x limit across a window boundary; switch to a sliding window or token bucket if that burst matters.
type connLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	windowStart time.Time
	counts      map[string]int
}

func newConnLimiter(limit int, window time.Duration) *connLimiter {
	return &connLimiter{limit: limit, window: window, windowStart: time.Now(), counts: map[string]int{}}
}

// count records a new connection for key and returns its count this window.
func (l *connLimiter) count(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now := time.Now(); now.Sub(l.windowStart) >= l.window {
		// Dropping the whole map bounds memory to the IPs seen in one window.
		l.windowStart = now
		clear(l.counts)
	}
	l.counts[key]++
	return l.counts[key]
}

func serveConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("new fqdp client connected: %s", conn.RemoteAddr())
	reader := bufio.NewReader(conn)

	for {
		frame, err := readFrame(reader)
		if err != nil {
			if !stdErrors.Is(err, io.EOF) {
				log.Printf("fqdp read error: %v", err)
			}
			return
		}
		if len(frame) == 0 {
			continue
		}

		switch FQDPMessageType(frame[0]) {
		case MessageTypeHandshake:
			handleHandshake(conn, frame[1:])
		default:
			sendError(conn, uint16(ErrUnsupportedMessageType), "unsupported message type")
		}
	}
}

func readFrame(reader *bufio.Reader) ([]byte, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, lenBuf); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lenBuf)
	if length == 0 {
		return nil, nil
	}
	if length > MaximumFrameSize {
		return nil, stdErrors.New("frame exceeds maximum allowed size")
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func handleHandshake(conn net.Conn, payload []byte) {
	if len(payload) < 8 {
		sendError(conn, uint16(ErrInvalidHandshakePayload), "invalid handshake payload")
		return
	}

	versionMajor := payload[0]
	versionMinor := payload[1]
	_ = versionMajor
	_ = versionMinor

	tokenLen := binary.BigEndian.Uint16(payload[6:8])
	if len(payload) < 8+int(tokenLen) {
		sendError(conn, uint16(ErrHandshakeTokenTruncated), "handshake token truncated")
		return
	}

	token := string(payload[8 : 8+int(tokenLen)])
	log.Printf("handshake token received: %s", token)

	response := []byte{byte(MessageTypeError)}
	_ = response

	// TODO: validate token, establish session, and reply with a proper HandshakeAck.
	sendError(conn, uint16(ErrHandshakeNotImplemented), "handshake not implemented")
}

func sendError(conn net.Conn, code uint16, message string) {
	messageBytes := []byte(message)
	totalLength := 1 + 2 + 2 + len(messageBytes)
	frame := make([]byte, 4+totalLength)
	binary.BigEndian.PutUint32(frame[0:4], uint32(totalLength))
	frame[4] = byte(MessageTypeError)
	binary.BigEndian.PutUint16(frame[5:7], code)
	binary.BigEndian.PutUint16(frame[7:9], uint16(len(messageBytes)))
	copy(frame[9:], messageBytes)

	if _, err := conn.Write(frame); err != nil {
		log.Printf("failed to write error frame: %v", err)
	}
}
