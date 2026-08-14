// Package fqdp implements the FQDP binary protocol server described in
// docs/fqdp.md. Currently a stub: only Handshake is parsed; no session,
// Query, Subscribe, or Update handling exists yet.
package fqdp

import (
	"bufio"
	"context"
	"encoding/binary"
	stdErrors "errors"
	"io"
	"log"
	"net"
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
// goroutine, until ctx is canceled.
func StartTCPServer(ctx context.Context, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("fqdp tcp server listening on %s", addr)

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				log.Printf("accept error: %v", err)
				continue
			}
		}
		go serveConnection(conn)
	}
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
	if len(payload) < 6 {
		sendError(conn, uint16(ErrInvalidHandshakePayload), "invalid handshake payload")
		return
	}

	versionMajor := payload[0]
	versionMinor := payload[1]
	_ = versionMajor
	_ = versionMinor

	tokenLen := binary.BigEndian.Uint16(payload[4:6])
	if len(payload) < int(6+tokenLen) {
		sendError(conn, uint16(ErrHandshakeTokenTruncated), "handshake token truncated")
		return
	}

	token := string(payload[6 : 6+tokenLen])
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
