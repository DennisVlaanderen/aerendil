package fqdp

// FQDPMessageType represents the type of a FQDP message.
type FQDPMessageType uint8

const (
	// MessageTypeHandshake is sent by the client to open a session,
	// carrying the protocol version and an auth token.
	MessageTypeHandshake FQDPMessageType = 0x01
	// MessageTypeHandshakeAck is the server's reply to a Handshake,
	// carrying the negotiated version and a session id.
	MessageTypeHandshakeAck FQDPMessageType = 0x02
	// MessageTypeQuery is sent by the client to request the current value
	// of one or more flags by key.
	MessageTypeQuery FQDPMessageType = 0x10
	// MessageTypeQueryResponse is the server's reply to a Query, carrying
	// each requested flag's value and version.
	MessageTypeQueryResponse FQDPMessageType = 0x11
	// MessageTypeSubscribe is sent by the client to request push updates
	// for flags matching the given criteria.
	MessageTypeSubscribe FQDPMessageType = 0x20
	// MessageTypeUnsubscribe is sent by the client to cancel a previously
	// created subscription.
	MessageTypeUnsubscribe FQDPMessageType = 0x21
	// MessageTypeUpdate is sent by the server to push flag changes
	// matching an active subscription.
	MessageTypeUpdate FQDPMessageType = 0x22
	// MessageTypeAck is sent by either peer to acknowledge receipt of a
	// prior message, referencing its id or sequence number.
	MessageTypeAck FQDPMessageType = 0x30
	// MessageTypeNack is sent by the server to reject a message the
	// client sent, referencing its id or sequence number.
	MessageTypeNack FQDPMessageType = 0x31
	// MessageTypeHeartbeat is an empty-payload keepalive sent by either
	// peer to detect a dead connection.
	MessageTypeHeartbeat FQDPMessageType = 0x40
	// MessageTypeError is sent by the server with an error code and
	// message; the connection may be closed afterward.
	MessageTypeError FQDPMessageType = 0x50
	// MessageTypeAdmin carries restricted operational commands (e.g. a
	// forced refresh), gated by the sender's token/ACL.
	MessageTypeAdmin FQDPMessageType = 0xF0
)
