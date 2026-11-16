package fqdp

// FQDPHandshakeError is the wire-level error code used for FQDP handshake failures.
type FQDPHandshakeError uint16

const (
	// ErrUnsupportedMessageType indicates the peer sent an unsupported message type.
	ErrUnsupportedMessageType FQDPHandshakeError = 100
	// ErrInvalidHandshakePayload indicates the handshake payload was malformed.
	ErrInvalidHandshakePayload FQDPHandshakeError = 101
	// ErrHandshakeTokenTruncated indicates the token length exceeded the supplied payload.
	ErrHandshakeTokenTruncated FQDPHandshakeError = 102
	// ErrHandshakeNotImplemented indicates the handshake flow is not yet implemented.
	ErrHandshakeNotImplemented FQDPHandshakeError = 103
	// ErrRateLimited indicates a connection or request rate limit was exceeded.
	ErrRateLimited FQDPHandshakeError = 107
)
