# FQDP — Feature Query & Definition Protocol

**Version 0.1.** This document is the target specification, not a
description of current behavior.

## Contents

1. [Overview](#1-overview)
2. [Quick reference](#2-quick-reference)
3. [Transport and framing](#3-transport-and-framing)
4. [Session lifecycle](#4-session-lifecycle)
5. [Consumption modes](#5-consumption-modes)
6. [Data model](#6-data-model)
7. [Evaluation](#7-evaluation)
8. [Flag messages](#8-flag-messages)
9. [Binary encoding](#9-binary-encoding)
10. [Frame examples](#10-frame-examples)
11. [Conformance](#11-conformance)
12. [Security considerations](#12-security-considerations)
13. [Implementation notes](#13-implementation-notes)
14. [Not in 0.1](#14-not-in-01)

[Appendix A. Glossary](#appendix-a-glossary)

[Appendix B. Example session](#appendix-b-example-session)

---

## 1. Overview

FQDP is a lightweight, language-agnostic protocol for high-throughput feature
flag queries, subscriptions and definition delivery.

- **Transport.** It runs directly over TCP, not over HTTP or WebSocket.
- **Encoding.** The wire format is binary throughout. No JSON is sent on the
  wire.
- **Authentication.** Clients authenticate with a token-based handshake.

### Goals

- Low-latency queries and push updates for large numbers of clients.
- Simple subscription semantics: an initial snapshot, then deltas.
- Two consumption modes (§5):
  - **Value mode.** The server evaluates flags against a context the client
    sends.
  - **Definition mode.** The server ships full flag definitions, and the client
    evaluates them locally, with no round-trip per call.
- Language independence. A compact, fully specified binary encoding and shared
  conformance vectors give independent SDKs byte- and result-identical behavior.
- Forward compatibility. The rule language can grow without breaking SDKs that
  are already deployed (§8.8).
- Sessions you can rely on: an explicit established state, token rotation,
  two-sided credential checks, and health reporting (§4).
- Secure connections through TLS and token authentication.

### Terminology

- Every term in this spec has exactly one meaning; [Appendix
  A](#appendix-a-glossary) is the authority. Product milestones are called
  **releases** ("the first product release"), never versions.
- Three numbers evolve independently:

| Number | Value in this document | Defined in |
| --- | --- | --- |
| Protocol version | 0.1 | §4.7 |
| Evaluator level | 1 | §8.8 |
| Definition `schema_version` | 1 | §9.1 |

- Byte layouts are written as `[type name]`.
- `C→S` means client to server. `S→C` means server to client.
- New to binary protocols? [Appendix A](#appendix-a-glossary) explains every
  term this spec uses, starting with how to read a byte layout.

---

## 2. Quick reference

### 2.1 Constants and defaults

| Name | Value | Where |
| --- | --- | --- |
| Protocol version | `0.1` (`ver_major = 0`, `ver_minor = 1`) | §4.7 |
| Default port | 9001 (deployment-configurable) | §3 |
| `MaximumFrameSize` | 10 MiB (payload), S→C frames | §3 |
| `MaximumClientFrameSize` | 256 KiB (payload), C→S frames, in every state | §3 |
| Handshake timeout | 10 s recommended (configurable) | §4.1 |
| `sync_timeout_s` | ≥ 1, server-configured, announced in the HandshakeAccept | §4.1, §4.2 |
| `heartbeat_interval_s` | announced by the server in the HandshakeAccept | §4.6 |
| Dead-peer timeout | 3 × `heartbeat_interval_s` | §4.6 |
| `credential_check_interval` | ≥ 1 s, 30 s recommended (server-configured), announced in the HandshakeAccept | §4.2, §4.4 |
| Re-authentication window | the earlier of 20% of the token lifetime remaining or 60 s before expiry | §4.3 |
| Max active subscriptions per session | 256 recommended (configurable) | §8.2 |
| Rollout buckets | 100,000 (weights sum to 100000) | §6.5 |
| Bucketing hash | SHA-256, first 8 bytes as big-endian u64 | §6.5 |
| Evaluator level | 1 | §8.8 |
| Definition `schema_version` | 1 | §9.1 |

### 2.2 Limits

| Item | Limit | Binary width |
| --- | --- | --- |
| Flag key | `^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$` | `u16` length |
| Variation id, rule id | `^[A-Za-z0-9._-]{1,64}$` | `u8` length |
| Variations per flag | 1–100 | `u8` count / index |
| Rules per flag | 0–500 | `u16` count |
| Children per condition group | 1–50 | `u8` count |
| Group depth (evaluator level 1) | 1 | — |
| Values per condition | 1–1,000 | `u16` count |
| Segment rules | 0–100 | `u8` count |
| Segment `included` + `excluded` targeting keys | 6,000 total | `u16` counts |
| Encoded definition (flag or segment) | 256 KiB | `u32` length |
| Integer variation values | ±(2^53−1) | `i64` |
| Context `targetingKey` | 1–256 bytes UTF-8 | `u16` length |
| Context attributes | 0–100, names `^[A-Za-z0-9_.-]{1,64}$` | `u8` count / length |
| Encoded context | 64 KiB | `u32` length |
| Keys per Query | 1–1,000 | `u16` count |
| Criteria per Subscribe or DefinitionQuery | 1–1,000 | `u16` count |

The 256 KiB definition cap also applies to segments: 6,000 UUID-length
targeting keys (about 228 KB) fit, but longer keys reach the byte cap first.
The HTTP API rejects a segment that breaks either limit.

### 2.3 Message types

| Type | Name | Direction | Purpose | Section |
| --- | --- | --- | --- | --- |
| 0x01 | Handshake | C→S | Open a session: version, capabilities, token | §4.2 |
| 0x02 | HandshakeAccept | S→C | Token accepted: session id, granted capabilities, heartbeat interval | §4.2 |
| 0x03 | Reauthenticate | C→S | Replace the session's token | §4.3 |
| 0x10 | Query | C→S | Evaluate flags against a context (value mode) | §8.1 |
| 0x11 | QueryResponse | S→C | Evaluation results | §8.1 |
| 0x12 | DefinitionQuery | C→S | One-shot fetch of definitions | §8.5 |
| 0x13 | DefinitionResponse | S→C | Definitions | §8.5 |
| 0x20 | Subscribe | C→S | Subscribe in value or definition mode | §8.2 |
| 0x21 | Unsubscribe | C→S | End a subscription | §8.2 |
| 0x22 | Update | S→C | Value-mode snapshot and deltas | §8.3 |
| 0x23 | DefinitionUpdate | S→C | Definition-mode snapshot and deltas | §8.5 |
| 0x30 | Control | both | Session established, credential check, re-auth result, status change | §4.4 |
| 0x31 | Reject | S→C | Reject one request | §8.9 |
| 0x40 | Heartbeat | both | Keepalive | §4.6 |
| 0x50 | Error | S→C | Connection-level failure, then close | §8.9 |

All other type values are unassigned and get `Error` 100. A server that
receives an S→C type (HandshakeAccept, QueryResponse, DefinitionResponse, Update,
DefinitionUpdate, Reject, Error) also sends `Error` 100.

### 2.4 Enumerations

| Field | Values |
| --- | --- |
| Handshake `client_capabilities` (u16) | bit 0 `CAP_DEFINITIONS`; bits 8–11 `EVAL_LEVEL` (1–15 with bit 0 set, else 0); others 0 |
| HandshakeAccept `server_flags` (u8) | Granted capability bits 0–7 |
| `control_kind` (u8) | 0 `SESSION_ESTABLISHED` (S→C), 1 `CHECK` (C→S), 2 `CHECK_OK` (S→C), 3 `REAUTH_OK` (S→C), 4 `STATUS` (S→C) |
| `server_status` (u8) | 0 `normal`, 1 `degraded` |
| Subscribe `mode` (u8) | 0 value, 1 definition |
| `value_type` (u8) | 0 bool, 1 int, 2 string, 3 json, 0xFF none |
| `reason` (u8) | 0 `flag_disabled`, 1 `rule_match`, 2 `fallthrough`, 3 `flag_not_found`, 4 `evaluation_error` |
| `error_kind` (u8) | 0 none, 1 `malformed_flag` |
| `change_kind` (u8) | 0 upsert, 1 remove |
| `item_kind` (u8) | 0 flag, 1 segment, 2 server-evaluated flag |
| `update_flags` (u8) | bit 0 `SNAPSHOT`, bit 1 `SNAPSHOT_END` |
| `response_flags` (u8) | bit 0 `MORE` (QueryResponse, DefinitionResponse) |
| Reject `ref_kind` (u8) | 0 `query_id`, 1 `subscription_id` |
| Flag `type` (u8) | 0 boolean, 1 integer, 2 string, 3 json |
| `flag_bits` (u8) | bit 0 enabled |
| Opcodes (u8) | 0x00 `in`, 0x01 `contains`, 0x02 `startsWith`, 0x03 `endsWith`, 0x04 `lt`, 0x05 `lte`, 0x06 `gt`, 0x07 `gte`, 0x08 `inSegment`, 0x09–0x7F reserved leaves, 0x80 `all`, 0x81 `any`, 0x82 reserved (`not`), 0x83–0xFF reserved |
| `cond_bits` (u8) | bit 0 negate |
| Condition `value_kind` (u8) | 0 bool, 1 number (float64), 2 string; a context attribute's value kind sets bit 7 for a list |
| `serve_kind` (u8) | 0 variation, 1 rollout |

### 2.5 Error and Reject codes

An **Error** ends the connection: the server closes it after sending. A
**Reject** rejects one request, and the connection stays open. Codes 102, 103
and 125–199 are reserved.

| Code | Name | Sent as | When |
| --- | --- | --- | --- |
| 100 | `ErrUnsupportedMessageType` | Error | Unassigned or unknown message type. |
| 101 | `ErrInvalidHandshakePayload` | Error | The Handshake body does not decode, including a `token_len` that runs past the payload. |
| 102 | — | — | Retired (was `ErrHandshakeTokenTruncated`, now part of 101). Not sent. |
| 103 | — | — | Retired (was `ErrHandshakeNotImplemented`, stub only). Not sent. |
| 104 | `ErrUnsupportedVersion` | Error | The version is not implemented (§4.7). Checked before the rest of the Handshake body is decoded. The message names the supported versions. |
| 105 | `ErrAuthenticationFailed` | Error | Invalid token; not an application-credential service token; unknown or inactive credential; a credential without `flags:read`, at Handshake or Reauthenticate. |
| 106 | `ErrCredentialRevoked` | Error | The session's credential was revoked or deactivated, or lost `flags:read`. Either side of the credential check (§4.4) can detect this. |
| 107 | `ErrRateLimited` | Error | A connection or request rate limit was exceeded (§12). |
| 108 | `ErrFrameTooLarge` | Error | A C→S frame's `length` exceeds `MaximumClientFrameSize` (§3). Sent before the payload is read. |
| 109 | `ErrSessionNotEstablished` | Error | A request arrived before `Control(SESSION_ESTABLISHED)` (§4.1). |
| 110 | `RejDefinitionsNotPermitted` | Reject | A definition-mode request without the granted capability. Also sent for each definition subscription that ends because the credential lost `flags:definitions:read` (§4.3, §4.4). |
| 111 | `RejInvalidContext` | Reject | The context does not decode, has no `targetingKey`, has a bad attribute name, or exceeds limits; a value-mode Subscribe or Query with `context_len = 0`; or a definition-mode Subscribe with `context_len ≠ 0`. |
| 112 | `RejInvalidCriteria` | Reject | The criteria list is empty or longer than §2.2 allows, or a criterion is not a valid key, prefix or `*`. |
| 113 | `RejUnknownMode` | Reject | The Subscribe `mode` is neither 0 nor 1. |
| 114 | `RejDuplicateSubscription` | Reject | The `subscription_id` is not higher than the last one used in the session (§8.2). |
| 115 | `ErrMalformedMessage` | Error | A known message type whose body does not decode: truncated, trailing bytes, an unknown enum value (including `control_kind`), an S→C `control_kind` sent by a client, or a reserved bit or must-be-0 field that is set (§9.1). Exceptions: an unknown Subscribe `mode` gets `Reject` 113, and a definition-mode Subscribe with `context_len ≠ 0` gets `Reject` 111. |
| 116 | `ErrTokenExpired` | Error | The token expired before a successful Reauthenticate. |
| 117 | `ErrServerUnavailable` | Error | The node cannot get in sync within its sync timeout, so it cannot establish the session. The client should try another node. |
| 118 | `ErrIdentityMismatch` | Error | A Reauthenticate token belongs to a different credential or environment than the session. |
| 119 | `ErrHeartbeatTimeout` | Error | No bytes from the client for 3 × `heartbeat_interval_s`. Sent best-effort. |
| 120 | `ErrHandshakeTimeout` | Error | No Handshake arrived within the handshake timeout. |
| 121 | `ErrAlreadyAuthenticated` | Error | A second Handshake on the connection, in any state. Reauthenticate is the way to replace a token. |
| 122 | `RejInvalidRequest` | Reject | An empty key list, an invalid flag key, a `query_id` not higher than the last one used (§8.1), more keys than §2.2 allows, or another per-request limit exceeded. |
| 123 | `RejSubscriptionLimit` | Reject | The session already has the maximum number of active subscriptions. |
| 124 | `ErrServerShuttingDown` | Error | The node is draining or shutting down. The client should reconnect, ideally to another node. |

---

## 3. Transport and framing

- **Transport.** Raw TCP. The default port is 9001.
- **TLS.** Direct TLS, where the TLS handshake runs as soon as the TCP
  connection opens, is strongly recommended in production. Self-signed
  certificates are supported, and SDKs warn on plaintext connections (§12). A
  STARTTLS-style upgrade is not defined.
- **Sessions.** The server keeps one lightweight session per connection. Query
  ids, subscription ids and sequence numbers are scoped to that session.
- **Frames.** Every frame is `[u32 length][payload]`. The payload is
  `[u8 MessageType][body…]`, and `length` counts the payload only.
  - A zero-length frame is ignored.
  - **Size limits differ by direction.** No C→S message needs more than
    `MaximumClientFrameSize` (256 KiB): the largest Handshake is about 128 KiB,
    and the largest Query about 191 KiB. The server checks `length` against this
    limit before it reads or allocates the payload, in every session state, and
    sends `Error` 108 if it is exceeded.
  - S→C frames may be up to `MaximumFrameSize` (10 MiB). A client that receives
    a larger `length` closes the connection.
- **Integers and strings.** All integers are big-endian. A string is written
  `[uN len][UTF-8 bytes]`, where N is given at each field.
- **Partial I/O.** Implementations must handle partial reads and writes on the
  TCP stream.

---

## 4. Session lifecycle

### 4.1 States

```text
CONNECTED ──Handshake──▶ AUTHENTICATED ──Control(SESSION_ESTABLISHED)──▶ ESTABLISHED ──▶ CLOSED
   │  (HandshakeAccept sent)     │                                            │ ▲
   │                          └──Error 117 (not in sync in time)           │ │ Reauthenticate → Control(REAUTH_OK)
   └──Error 120 (no Handshake in time)                                     └─┘ Control(CHECK) → Control(CHECK_OK)
```

- **Any state.** A Heartbeat (§4.6) is accepted in every state. These checks
  come before the state checks below:
  - an unassigned or S→C message type gets `Error` 100 (§2.3);
  - any Handshake after the first gets `Error` 121.
- **CONNECTED.** TCP (and TLS) is up. The server accepts only a Handshake (and
  Heartbeats): any other known message gets `Error` 109. If no Handshake arrives
  within the handshake timeout, the server sends `Error` 120.
- **AUTHENTICATED.** The server has validated the token and sent the
  HandshakeAccept. It now finishes setting up the session: registering it, and
  making sure the node is in sync: that it has caught up with the cluster
  leader, so it serves consistent data. A request in this state gets `Error`
  109; Heartbeats keep flowing as usual. The client waits at most
  `sync_timeout_s` plus `heartbeat_interval_s` (§4.2) in this state.
- **ESTABLISHED.** The server has sent `Control(SESSION_ESTABLISHED)`. The
  client may now send requests.
  - If the node cannot get in sync within its sync timeout, it sends `Error` 117
    instead, so the client can try another node.

### 4.2 Handshake and HandshakeAccept

#### Handshake (0x01)

```text
[u8 0x01]
[u8 ver_major]                  // 0
[u8 ver_minor]                  // 1
[u16 client_capabilities]
[u16 token_len][token]          // UTF-8
[u16 client_id_len][client_id]  // optional, 0 if absent
```

- **`client_id`.** A free-form label for the client (for example
  `checkout-service@1.4.2`). It has no protocol meaning; the server records it
  in session logs and the session registry (§13) to help operators identify
  connections.
- **Token type.** The token must be an application-credential service token
  (§12), which is scoped to exactly one environment. Any other token, such as a
  human login token, gets `Error` 105.
- **Scope.** The credential must hold `flags:read`. A credential without it, for
  example one with only write scopes, gets `Error` 105.

| Capability bit | Name | Meaning |
| --- | --- | --- |
| 0 | `CAP_DEFINITIONS` | The client wants definition mode. The server grants it only if the credential holds `flags:definitions:read`. |
| 1–7 | reserved | Must be 0. |
| 8–11 | `EVAL_LEVEL` | A 4-bit unsigned value: the highest evaluator level the client implements (§8.8). 1–15 when bit 0 is set; must be 0 when bit 0 is clear. |
| 12–15 | reserved | Must be 0. |

#### HandshakeAccept (0x02)

```text
[u8 0x02]
[u8 ver_major]                    // accepted version
[u8 ver_minor]
[16 bytes session_id]             // opaque, e.g. a UUID
[u8 server_flags]                 // capability bits 0–7 that were granted
[u16 heartbeat_interval_s]         // ≥ 1
[u16 credential_check_interval_s]  // ≥ 1, server-configured
[u16 sync_timeout_s]               // ≥ 1, server-configured
```

- **`heartbeat_interval_s`.** Must be at least 1. A client that receives 0
  treats the HandshakeAccept as malformed and closes the connection.
- **`credential_check_interval_s`.** The session's `credential_check_interval`,
  set by the server. The client uses it to pace its `CHECK`s (§4.4). Must be
  at least 1. A client that receives 0 treats the HandshakeAccept as malformed
  and closes the connection.
- **`sync_timeout_s`.** The node's sync timeout (§4.1). If neither
  `Control(SESSION_ESTABLISHED)` nor `Error` 117 has arrived within
  `sync_timeout_s` plus `heartbeat_interval_s` after the HandshakeAccept, the
  client closes the connection and tries another node. Must be at least 1. A
  client that receives 0 treats the HandshakeAccept as malformed and closes the
  connection.
- **Failure.** The server reads `ver_major` and `ver_minor` first. A version it
  doesn't implement gets `Error` 104 before the rest of the body is decoded
  (§4.7). A token that fails validation gets `Error` 105.
- **Success.** The server sends the HandshakeAccept, then
  `Control(SESSION_ESTABLISHED)` once the session is established.
- **Definitions refused.** If the client asked for definition mode but
  `server_flags` bit 0 is clear, the request was refused. The session then works
  in value mode only.

### 4.3 Reauthenticate (0x03)

Reauthenticate replaces the session's token without reconnecting. Subscriptions,
snapshots and sequence numbers survive token rotation.

```text
[u8 0x03]
[u16 token_len][token]
```

- **Identity.** The new token must belong to the same credential and environment
  as the session; otherwise the server sends `Error` 118. An invalid token, or a
  credential without `flags:read`, gets `Error` 105.
- **Success.** The server replies `Control(REAUTH_OK)` with the new
  `token_ttl_s`.
- **Scope changes** take effect immediately. Scopes belong to the credential,
  not the token: the server reads them from the store.
  - If the credential no longer holds `flags:definitions:read`, every definition
    subscription ends with a `Reject` 110 that references it. Later definition
    requests also get `Reject` 110. The server-side credential check (§4.4)
    applies the same rule without a Reauthenticate. The SDK then switches those
    flags to server evaluation (§8.8).
  - Gaining a scope never upgrades the session: only a Handshake can grant
    `CAP_DEFINITIONS`.
- **Timing.** The client re-authenticates before `token_ttl_s` runs out. It
  should start at the earlier of two moments: when less than 20% of the token's
  lifetime remains, or 60 seconds before expiry. Both are measured from the
  first `token_ttl_s` received for that token (in `SESSION_ESTABLISHED` or
  `REAUTH_OK`). The fresh token is obtained the same way as the first one, for
  example through the HTTP API's client-credentials endpoint.
- **Expiry.** If the token expires first, the server sends `Error` 116 and
  closes the connection.

### 4.4 Control (0x30) and credential checks

Control carries four things: session establishment, credential checks, the
result of a re-authentication, and server status changes. It flows in both
directions and has one fixed layout.

```text
[u8 0x30]
[u8 control_kind]      // §2.4
[u32 ref_id]           // CHECK: client-chosen; CHECK_OK echoes it; otherwise 0
[u8 server_status]     // S→C: 0 normal, 1 degraded; C→S: 0
[u32 token_ttl_s]      // S→C: seconds until the session's token expires;
                       // C→S: 0
```

| `control_kind` | Direction | Meaning |
| --- | --- | --- |
| 0 `SESSION_ESTABLISHED` | S→C | Sent exactly once, after the HandshakeAccept. The session is established. |
| 1 `CHECK` | C→S | Ask the server to re-validate the credential and report the server status. |
| 2 `CHECK_OK` | S→C | Reply to `CHECK`: the credential is still valid. If it was revoked, the server sends `Error` 106 instead. |
| 3 `REAUTH_OK` | S→C | Reply to a successful Reauthenticate. |
| 4 `STATUS` | S→C | Sent unsolicited when the server's `server_status` changes. |

**Credential validity is checked from both sides.**

- **Interval.** `credential_check_interval` is the value the server announced
  in the HandshakeAccept (§4.2).
- **Server side.** The server re-validates a session's credential, its active
  state and its scopes, whenever the store reports a change to it, and in any
  case at least every `credential_check_interval`. This also covers sessions
  that only receive pushes.
  - A revoked or deactivated credential, or one that lost `flags:read`, gets
    `Error` 106, and the connection is closed.
  - A credential that lost `flags:definitions:read` ends the session's
    definition subscriptions with `Reject` 110, as in §4.3.
- **Client side.**
  - The client may send `Control(CHECK)` at any time. The server re-validates
    immediately and replies `Control(CHECK_OK)` or `Error` 106. `CHECK`s count
    toward the per-session request rate limit (§12).
  - While no other request is outstanding, SDKs should send a `CHECK` at least
    every `credential_check_interval`.
  - If no `CHECK_OK` arrives within 3 × `heartbeat_interval_s`, the client
    closes the connection and reconnects. The timer starts when the `CHECK` is
    sent. It pauses while a frame is partly received, so a large snapshot frame
    on a slow link does not trigger it. Heartbeats and other complete frames do
    not reset it, so a server that keeps the connection alive but never answers
    is still caught.

### 4.5 Server status and session health

- **Server status.** `server_status = 1` (*degraded*) means the node is still
  serving but cannot guarantee fresh data, for example because it has lost
  cluster quorum. SDKs surface this to the application. They may reconnect
  elsewhere, but are not required to.
- **Session health.** A session is **healthy** when all of these hold:
  - it is ESTABLISHED;
  - its token has not expired;
  - its credential passed the last check.

  A session is **expiring** when it is ESTABLISHED and inside its
  re-authentication window (§4.3). The server tracks healthy, expiring and
  total sessions as part of its own health reporting (§13).

### 4.6 Heartbeat (0x40)

```text
[u8 0x40]
```

- **When to send.** Either peer sends a Heartbeat once it has sent nothing for
  `heartbeat_interval_s`. No reply is needed. Heartbeats are valid in every
  session state (§4.1), including while the client waits for
  `SESSION_ESTABLISHED`.
- **Dead peers.** A peer that has received no bytes for 3 ×
  `heartbeat_interval_s` considers the connection dead and closes it. The server
  first tries, best-effort, to send `Error` 119. Counting bytes rather than
  whole frames keeps a large frame on a slow link from looking like a dead peer:
  a peer cannot send a Heartbeat in the middle of a frame.
- **Scope.** Heartbeats only prove that the connection is alive. Credential
  validity and server status are checked with Control (§4.4).

### 4.7 Protocol versioning

This specification is protocol **0.1**. While the major version is `0`:

- **Breaking changes.** Any minor bump may change the wire format.
- **Exact versions only.** The server accepts a Handshake only for a `0.minor`
  it implements exactly. It may implement more than one during a transition. Any
  other version gets `Error` 104, and the connection is closed.
- **No minor-version negotiation.** The rule language grows through evaluator
  levels (§8.8), which need no protocol bump.
- **Stable in every version.** So that a version mismatch is always reported as
  `Error` 104, these never change, in 0.x or later:
  - the frame header: `[u32 length][u8 MessageType]` (§3);
  - the first two bytes of the Handshake body: `[u8 ver_major][u8 ver_minor]`;
  - the Error layout: `[u8 0x50][u16 code][u16 msg_len][msg]` (§8.9).

  The server checks the version before it decodes the rest of the Handshake, so
  a body in another version's layout still gets `Error` 104, not 101.

**1.0** is reserved for when FQDP is considered feature-complete for a
releasable product. From 1.0 onward, a different major version is rejected, and
minor differences within one major version must stay compatible.

---

## 5. Consumption modes

A connection only ever sees one environment: the one its credential is scoped
to. FQDP accepts only application-credential service tokens (§4.2), and each
is scoped to exactly one environment. Environment is therefore never a rule
attribute.

| | Value mode (server-evaluated) | Definition mode (client-evaluated) |
| --- | --- | --- |
| Messages | Query / QueryResponse, Subscribe(mode=0) / Update | DefinitionQuery / DefinitionResponse, Subscribe(mode=1) / DefinitionUpdate |
| Client needs | Framing plus the context encoder (§9.5) | Framing, the definition decoder (§9.3–9.4) and the evaluator (§6–7) |
| Context leaves the app? | Yes, sent with Query/Subscribe | No, except for server-evaluated flags (§8.8) |
| Per-call cost | One round-trip (Query), or none with a cached Update | Local, no network |
| Credential scope | `flags:read` | `flags:read` + `flags:definitions:read` |
| Typical client | Polyglot or thin services, and languages without an SDK | Official SDKs |

---

## 6. Data model

This section shows the model in its **HTTP API and export JSON form**, which is
what the admin API, the UI and export/import use. FQDP never carries this JSON.
It carries the binary encoding in §9, which maps one-to-one onto the same
model.

Fields that only matter to humans are not sent over FQDP. These are rule
`description` and segment `name`.

### 6.1 Flag definition

```json
{
  "schemaVersion": 1,
  "key": "checkout.new-flow",
  "environmentId": "prod",
  "type": "string",
  "enabled": true,
  "revision": 1042,
  "salt": "checkout.new-flow",
  "variations": [
    { "id": "control",   "value": "v1" },
    { "id": "treatment", "value": "v2" }
  ],
  "offVariation": "control",
  "rules": [
    {
      "id": "beta-testers",
      "description": "Everyone in the beta segment",
      "match": "all",
      "conditions": [ { "op": "inSegment", "values": ["beta"] } ],
      "serve": { "variation": "treatment" }
    },
    {
      "id": "benelux-50",
      "match": "any",
      "conditions": [
        { "attribute": "country", "op": "in", "values": ["NL", "BE", "LU"] }
      ],
      "serve": {
        "rollout": {
          "bucketBy": "targetingKey",
          "weights": [
            { "variation": "control",   "weight": 50000 },
            { "variation": "treatment", "weight": 50000 }
          ]
        }
      }
    }
  ],
  "fallthrough": { "variation": "control" }
}
```

| Field | Rules |
| --- | --- |
| `schemaVersion` | `1`. |
| `key` | Must match the flag-key pattern (§2.2). |
| `type` | `boolean`, `integer`, `string` or `json`. It cannot be changed after creation. |
| `enabled` | The kill switch. When `false`, the flag serves `offVariation` regardless of rules. |
| `revision` | Raft log index of the last write. Read-only. |
| `salt` | The bucketing salt. Defaults to the flag `key`. See the notes below the table. |
| `variations` | Unique ids. Each `value` must match `type`. A `boolean` flag has exactly `[{id:"true",value:true},{id:"false",value:false}]`. |
| `offVariation` | Required. The id of an existing variation. |
| `rules` | Evaluated in array order. Each `id` is unique within the flag. It is reported as `ruleId` and referenced in audit entries. |
| `rules[].match` + `rules[].conditions` | The rule's root condition group (§6.4). `match` defaults to `all`. |
| `rules[].serve`, `fallthrough` | Exactly one of `{ "variation": id }` or `{ "rollout": … }` (§6.5). |

Notes on `salt`:

- Because it defaults to the flag `key`, a context lands in the same bucket in
  every environment, and promoting a flag keeps its rollout membership.
- Changing it reshuffles every context. Only an explicit, audited "reset
  rollout" action should do that.

### 6.2 Segment

```json
{
  "schemaVersion": 1,
  "key": "beta",
  "environmentId": "prod",
  "name": "Beta testers",
  "revision": 1038,
  "included": ["user-1", "user-2"],
  "excluded": ["user-9"],
  "rules": [
    {
      "id": "staff",
      "match": "all",
      "conditions": [ { "attribute": "email", "op": "endsWith", "values": ["@example.com"] } ]
    }
  ]
}
```

- **Membership** is decided in this order:
  1. If the context's targeting key is in `included`, the context is a member.
  2. Otherwise, if the targeting key is in `excluded`, it is not a member.
  3. Otherwise, if any segment rule matches, it is a member.
  4. Otherwise, it is not a member.
- **Segment rules** are root groups (§6.4), with no `serve`. They must not use
  `inSegment`, so segments do not recurse.
- **Deletion.** A segment cannot be deleted while any flag in its environment
  references it.

### 6.3 Conditions

```json
{ "attribute": "plan", "op": "in", "values": ["pro", "enterprise"], "negate": false }
```

| `op` | Value kind | Matches when |
| --- | --- | --- |
| `in` | string, number, boolean | The attribute equals any of `values`. Equality is strict, with no coercion: `"5"` ≠ `5`. |
| `contains` | string | The attribute contains any of `values` as a substring. |
| `startsWith` / `endsWith` | string | The attribute starts or ends with any of `values`. |
| `lt` / `lte` / `gt` / `gte` | number | A numeric comparison (IEEE-754 float64) against any of `values`. |
| `inSegment` | string (no `attribute`) | The context is a member of any segment listed in `values` (segment keys). |

These semantics are normative and identical in every evaluator:

- **`attribute`.** Either `targetingKey` (the context's targeting key) or a
  top-level attribute name. There are no nested paths.
- **`values`.** All values in one condition are of one value kind: all strings,
  all numbers or all booleans.
- **List attributes.** If the context attribute is a list, the condition matches
  when any element matches. `negate` inverts that overall result, not each
  element: `{groups in ["a"], negate}` is false for `["a", "b"]`. An empty list
  counts as a missing attribute, since it has no value kind.
- **Missing or mismatched attributes.** If the attribute is missing, or its
  value kind does not fit the operator, the condition is **false**, and `negate`
  is **not** applied. A missing attribute never widens a rule's audience.
- **`negate`.** When the attribute is present and its value kind fits,
  `negate: true` inverts the result.
- **`inSegment`.** It has no attribute and always counts as present, so
  `negate: true` inverts it: `{inSegment beta, negate}` matches every context
  that is not a member of `beta`.
- **Numbers.** NaN and ±Inf are never valid values, in conditions or in contexts
  (§9.1). `-0` and `0` compare equal.
- **Strings.** String comparisons are case-sensitive and byte-exact on UTF-8,
  with no Unicode normalization.

Regex, semver and date operators are not part of evaluator level 1. Regex in
particular is excluded because its semantics differ across languages.

### 6.4 Condition groups

A rule's predicate is a **condition tree**. Each element of the tree is a
**clause**, which is either a leaf or a group:

- A **leaf** is one condition (§6.3).
- A **group** is `{ "match": "all" | "any", "conditions": [ clause, … ] }`.

The rule object itself is the root group. An entry in `conditions` that has a
`match` field is a group; one that has an `op` field is a leaf. An entry with
both, or with neither, is invalid.

- **Semantics.**
  - `all` is true when every child is true; `any` is true when at least one
    child is true.
  - Groups are never empty.
  - Evaluation has no side effects, so evaluators may short-circuit and may
    evaluate children in any order.
- **Evaluator level 1.** Depth is 1: the root group's children are all leaves.
  The HTTP API rejects a nested group on write with `400`.
- **Future nesting.** The shape below is already defined, but rejected at
  level 1. A later level will lift the depth limit (with a limit on total clauses)
  without changing the JSON shape, the binary layout or the semantics. Every
  level-1 definition stays valid at later levels.

```json
{
  "id": "nl-pro-or-staff",
  "match": "any",
  "conditions": [
    { "match": "all", "conditions": [
        { "attribute": "country", "op": "in", "values": ["NL"] },
        { "attribute": "plan",    "op": "in", "values": ["pro"] } ] },
    { "op": "inSegment", "values": ["staff"] }
  ],
  "serve": { "variation": "treatment" }
}
```

- **`not` group.** A single-child `not` group (opcode `0x82`) is reserved.
  Before it can be defined, its interaction with the missing-attribute rule
  (§6.3) has to be settled. Until then, leaf-level `negate` is the way to
  express negation.

### 6.5 Percentage rollout and bucketing

```json
"rollout": {
  "bucketBy": "targetingKey",
  "weights": [ { "variation": "control", "weight": 50000 }, { "variation": "treatment", "weight": 50000 } ]
}
```

- **Weights.** Integers from 0 to 100000 that sum to exactly **100000**. Each
  variation appears at most once.
- **`bucketBy`.** Defaults to `targetingKey`. It names the attribute whose value
  is hashed.
  - **Usable values** are a string, or an integer-valued number within
    ±(2^53−1). A number is converted to its plain decimal form, such as `42`,
    with no exponent and no `.0`. `-0` is written as `0`.
  - **Anything else is unusable:** a boolean, a list (even of one string), a
    number with a fractional part, or a number outside ±(2^53−1), which
    languages format differently. No evaluator converts these to text.

#### Bucketing (normative)

```text
input  = UTF-8( salt + "." + bucketValue )
digest = SHA-256(input)
n      = big-endian uint64 of digest[0:8]
bucket = n mod 100000                      // 0 ≤ bucket < 100000
```

- **Choosing the variation.** The context gets the first variation, in array
  order, whose cumulative weight is greater than `bucket`.
- **Missing `bucketBy` attribute.** If it is missing or unusable, `bucket = 0`.
  This can only happen with a custom `bucketBy`, since every context has a
  non-empty `targetingKey` (§6.6).

**Vectors.** These use `salt = "checkout.new-flow"` and the 50/50 rollout above,
so `control` covers buckets [0, 50000) and `treatment` covers [50000, 100000):

| bucketValue | digest[0:8] | bucket | variation |
| --- | --- | --- | --- |
| `user-123` | `6b4499fee63c64f4` | 74356 | treatment |
| `user-456` | `60fe5b62acc73e58` | 49880 | control |
| `user-789` | `84596b483f834821` | 21569 | control |

### 6.6 Context

```json
{ "targetingKey": "user-123", "attributes": { "country": "NL", "plan": "pro", "beta": true, "groups": ["a", "b"] } }
```

- **`targetingKey`.** Required, 1–256 bytes. It identifies the subject the flag
  is evaluated for, such as a user or an account. There is no context without
  one.
- **Attribute names.** Unique within the context. The name `targetingKey` is
  reserved and cannot be used for an attribute.
- **Attribute values.** A string, a number, a boolean, or a flat list of values
  of a single value kind.
- **One subject.** A context describes one subject. Composite contexts, which
  describe several subjects at once (for example a user and their organization),
  are not defined.
- **Limits.** See §2.2.

---

## 7. Evaluation

### 7.1 Algorithm (normative)

Given a flag definition `F` (or none) and a context `C`, evaluation takes the
first step that applies. The **fallback value** is the value the application
passes in, to be returned when evaluation cannot produce one.

0. **`C` is invalid** (SDK-local only). Return the fallback value, with reason
   `evaluation_error` and `errorKind: invalid_context`. A server never reaches
   this step: it rejects an invalid context with `Reject` 111.
1. **`F` is missing.** Return the fallback value, with reason `flag_not_found`.
2. **`F` is malformed.** Return the fallback value, with reason
   `evaluation_error` and `errorKind: malformed_flag`. Malformed means anything
   the strict-decoding rules in §9.1 reject (including an unknown
   `schema_version`, a bad index and bad weights), plus a reference to a segment
   the client does not hold.
3. **`F.enabled == false`.** Return `offVariation`, with reason `flag_disabled`.
4. **A rule matches.** Try the rules in order. The first rule whose root group
   (§6.4) is true decides the result: its serve, with reason `rule_match` and
   `ruleId` set.
5. **Fallthrough.** Return `fallthrough`, with reason `fallthrough`.

### 7.2 Reasons and error kinds

| Reason | u8 | Meaning |
| --- | --- | --- |
| `flag_disabled` | 0 | Kill switch off; served `offVariation`. |
| `rule_match` | 1 | Rule `ruleId` matched. |
| `fallthrough` | 2 | No rule matched; served `fallthrough`. |
| `flag_not_found` | 3 | No such flag in this environment. |
| `evaluation_error` | 4 | See `errorKind`. |
| `not_ready_fallback` | — | SDK-local only: no data yet (`client_not_ready`), or disconnected with no last-known-good copy. |

| Error kind | u8 | Meaning |
| --- | --- | --- |
| none | 0 | — |
| `malformed_flag` | 1 | The definition failed decoding or validation. |
| `invalid_context` | — | SDK-local only: the context failed validation (§7.1 step 0). |
| `wrong_type` | — | SDK-local only: a typed accessor was called on a flag of another type. |
| `client_not_ready` | — | SDK-local only: no snapshot received yet. |

---

## 8. Flag messages

### 8.1 Query and QueryResponse (value mode)

#### Query (0x10)

```text
[u8 0x10]
[u32 query_id]                // client-chosen; strictly increasing in the session
[u32 context_len]             // > 0
[context_len bytes context]   // §9.5
[u16 key_count]               // 1–1,000 (§2.2)
repeated: [u16 key_len][key]
```

#### QueryResponse (0x11)

It contains one item per requested key, in request order.

```text
[u8 0x11]
[u32 query_id]
[u8 response_flags]           // bit0 MORE: further frames for this query_id follow
[u16 item_count]
repeated:
  [u16 key_len][key]
  [u64 revision]              // §8.3 effective revision; 0 when reason = flag_not_found
  [u8 reason]
  [u8 error_kind]
  [u8 value_type]             // 0xFF = none
  [value]                     // §9.1 value encoding for value_type; absent with 0xFF
  [u8 variation_id_len][variation_id]
  [u8 rule_id_len][rule_id]   // 0 unless reason = rule_match
```

The item uses the same field order as an Update upsert (§8.3).

- **Context.** A Query always carries a context with a targeting key (§6.6).
  `context_len = 0` gets `Reject` 111.
- **Concurrent queries.** Several queries may be in flight at once. Responses
  are matched by `query_id`.
- **Query ids.** Each `query_id` is higher than the previous one in the session,
  so the server only remembers the last. Query and DefinitionQuery share one id
  space, since a Reject refers to either with `ref_kind 0`. An id that is not
  higher gets `Reject` 122. A client that runs out of ids reconnects.
- **Size.** A response that would exceed `MaximumFrameSize` is split across
  frames with `MORE`, in request order. The last frame has `MORE` clear. All
  frames of one response reflect one Raft log index.
- **Rejection.** An invalid Query gets a Reject (111 or 122), not a
  QueryResponse.

### 8.2 Subscribe and Unsubscribe

#### Subscribe (0x20)

It uses one layout for both modes.

```text
[u8 0x20]
[u32 subscription_id]        // client-chosen; strictly increasing in the session
[u8 mode]                    // 0 value, 1 definition
[u32 context_len][context]   // value mode: > 0; definition mode: must be 0
[u16 criteria_count]         // 1–1,000 (§2.2)
repeated: [u16 len][criterion]   // §8.4
```

- **Confirmation.** There is no separate confirmation message. The snapshot
  (§8.6) confirms the subscription. It is always sent, and always ends with a
  `SNAPSHOT_END` frame, even when nothing matches.
- **Rejection.** The server answers with a Reject carrying one of these codes:
  - 110: definition mode not granted.
  - 111: invalid context, a missing context in value mode, or a context in
    definition mode.
  - 112: invalid criteria.
  - 113: unknown mode.
  - 114: `subscription_id` not higher than the last one used in this session.
  - 123: subscription limit reached.

  Both modes share one layout, so the server decodes the whole body first. A
  body that does not decode is still `Error` 115.

#### Unsubscribe (0x21)

```text
[u8 0x21]
[u32 subscription_id]
```

There is no reply, and an unknown id is ignored. Frames already in flight may
still arrive; the client discards them. Because ids only go up, a later
Subscribe can never reuse the id: one that is not higher gets `Reject` 114.

### 8.3 Update (value mode)

```text
[u8 0x22]
[u32 subscription_id]
[u64 sequence]               // per subscription: 1, then +1 per frame
[u8 update_flags]
[u16 change_count]
repeated:
  [u8 change_kind]
  [u16 key_len][key]
  [u64 revision]             // effective revision, see below
  if upsert:
    [u8 reason]
    [u8 error_kind]
    [u8 value_type]
    [value]                  // §9.1 value encoding for value_type; absent with 0xFF
    [u8 variation_id_len][variation_id]
    [u8 rule_id_len][rule_id]
```

- **Re-evaluation.** The server re-evaluates the subscription's context whenever
  a matched flag, or a segment it references, changes.
- **Effective revision.** The `revision` of every flag item is the highest of
  the flag's revision and the revisions of the segments it references. This
  holds in every message: Update, QueryResponse, DefinitionResponse and
  DefinitionUpdate, for `item_kind` 0 and 2. A segment item carries the
  segment's own revision. A segment edit therefore raises the revision of every
  flag that depends on it, so the client's revision check (§8.6) accepts the new
  result.
- **When an Update is sent.** Only when the served value, variation id, reason
  or ruleId changes. A write that leaves this context's result unchanged sends
  nothing, even though it raises the effective revision. Revisions only go up,
  so the next Update still passes the client's revision check (§8.6).

### 8.4 Criteria

A criterion takes one of three forms:

- An exact flag key, such as `checkout.new-flow`.
- A prefix ending in `*`, such as `checkout.*`.
- `*` alone, meaning all flags in the environment.

### 8.5 Definition mode

#### DefinitionQuery (0x12)

It is a one-shot fetch, intended for short-lived processes.

```text
[u8 0x12]
[u32 query_id]               // shares the §8.1 id space; strictly increasing
[u16 criteria_count]         // 1–1,000 (§2.2)
repeated: [u16 len][criterion]
```

#### DefinitionResponse (0x13)

```text
[u8 0x13]
[u32 query_id]
[u8 response_flags]          // bit0 MORE: further frames for this query_id follow
[u16 item_count]
repeated:
  [u8 item_kind]             // 0 flag (§9.3), 1 segment (§9.4), 2 server-evaluated flag (§8.8)
  [u16 key_len][key]
  [u64 revision]             // flags: effective revision (§8.3); segments: own revision
  [u32 def_len][definition]  // 0 ⇒ exact-key criterion not found; always 0 for item_kind 2
```

An exact key that is not found is sent as `item_kind 0`, `revision 0`,
`def_len 0`.

#### DefinitionUpdate (0x23)

```text
[u8 0x23]
[u32 subscription_id]
[u64 sequence]
[u8 update_flags]
[u16 change_count]
repeated:
  [u8 change_kind]
  [u8 item_kind]
  [u16 key_len][key]
  [u64 revision]             // flags: effective revision (§8.3); segments: own revision
  [u32 def_len][definition]  // 0 on remove and for item_kind 2
```

#### Guarantees

- **Segment closure.** Results and subscriptions always include every segment
  that the returned flags reference. Clients never subscribe to segments
  directly.
- **Segment changes.** A segment change is pushed to every definition
  subscription that has a flag referencing that segment.
- **Ordering.**
  - A segment upsert precedes any flag upsert that references it. This holds
    within a frame and across the frames of one snapshot.
  - A flag that stops referencing a segment is sent before that segment's
    remove.
  - A client that applies changes in order therefore never holds a dangling
    reference.
- **Per-subscription state.** Client state is kept per subscription. A segment
  remove on one subscription says nothing about another that still references
  the segment. An SDK that merges subscriptions into one store must count
  segment references per subscription.
- **Size.** A response or snapshot that would exceed `MaximumFrameSize` is split
  across frames, using `MORE` (QueryResponse, DefinitionResponse), or `SNAPSHOT`
  / `SNAPSHOT_END`. All frames of one response or snapshot reflect one Raft log
  index (§8.6).
- **Readiness.** An SDK reports ready once `SNAPSHOT_END` has arrived for every
  initial subscription.

### 8.6 Snapshots, reconnect and revisions

- **Snapshots.** Every Subscribe, in either mode, begins with a snapshot of
  everything that matches.
  - Every snapshot frame carries `SNAPSHOT`, and the last one also carries
    `SNAPSHOT_END`.
  - A snapshot always has at least one frame. When nothing matches, it is a
    single `SNAPSHOT|SNAPSHOT_END` frame with `change_count = 0`.
  - An exact-key criterion that matches no flag is left out of the snapshot. The
    SDK reports such a key as `flag_not_found` locally. (A one-shot
    DefinitionResponse still reports it with `def_len 0`.)
  - A snapshot reflects the store at one Raft log index, even when it spans
    several frames.
  - Deltas follow the snapshot. They cover every write after the snapshot's
    index, and no write is skipped or sent twice.
- **Reconnect.** The client performs a new Handshake, re-subscribes, and
  receives a fresh snapshot. Any key it held that is absent when `SNAPSHOT_END`
  arrives is dropped. There is no backfill across sessions.
- **Revisions.**
  - A snapshot replaces the client's state for that subscription
    unconditionally, whatever revisions it held before, including a persisted
    last-known-good cache. This keeps clients correct after revisions restart
    low, for example after a Raft data-directory wipe or a backup restore (§13).
  - Within a session, a client applies a delta upsert only if its `revision` is
    greater than the revision it holds for that key. Revisions are Raft log
    indexes, so they can be compared across server nodes of one cluster.
  - A remove drops the key and the revision held for it, so a later upsert of
    the same key is always applied. A segment that leaves a subscription's view
    and comes back carries its unchanged own revision.
- **Sequences.** Sequence numbers are per subscription. They start at 1 and go
  up by exactly 1 per frame. TCP preserves order, so a gap or regression is a
  protocol violation, and the client closes the connection.

### 8.7 Flag message lifecycle summary

| Client wants | Send | Expect |
| --- | --- | --- |
| A value, once | Query | QueryResponse, or Reject |
| Values, pushed | Subscribe(mode 0) | Update snapshot, then deltas; or Reject |
| Definitions, once | DefinitionQuery | DefinitionResponse frames, or Reject |
| Definitions, pushed | Subscribe(mode 1) | DefinitionUpdate snapshot, then deltas; or Reject |
| Stop a subscription | Unsubscribe | — |

### 8.8 Evaluator levels

The evaluator level is how the rule language grows without breaking SDKs that
are already deployed.

- **What a level is.** An integer that names a cumulative set of evaluator
  features. **Level 1** is everything this specification defines.
  - Any addition to what an evaluator must understand (nesting, new operators,
    new clause forms) raises the level.
  - A change to the binary layout bumps `schema_version` instead (§9.1).
- **Recorded levels.** Each flag and segment records the minimum level needed to
  evaluate it. The server computes it on write and stores it in the definition
  header. A flag's **effective level** is the highest of its own level and the
  levels of the segments it references.
- **Server behaviour.** If a flag's effective level is higher than the session's
  `EVAL_LEVEL`, the server sends it as `item_kind 2` (*server-evaluated*): key
  and revision only. It also withholds referenced segments that the client
  cannot evaluate.
- **SDK behaviour.** The SDK evaluates a server-evaluated flag through a
  value-mode Query on the same connection, using the caller's context.
  - It may cache results per `(revision, context)`, where `revision` is the
    item's effective revision (§8.3), so segment changes invalidate the cache.
    The cache lives for one session: the SDK clears it when a new session is
    established, since revisions can restart low (§8.6, §13).
  - The cache must be bounded (for example an LRU, which drops the least
    recently used entry); its size is SDK-configurable.
  - Results stay correct; only local evaluation is lost, and only for that flag.
  - The SDK must make this visible, through a log line or metric.
- **After `Reject` 110.** When a definition subscription ends because the
  credential lost `flags:definitions:read` (§4.3, §4.4), the SDK:
  - discards the definitions it held for that subscription, including any
    persisted last-known-good copy, since the operator has withdrawn access to
    them;
  - treats every flag that subscription covered as server-evaluated, through the
    same Query path as above. No later revisions arrive for those flags, so it
    caches each result per context for at most `credential_check_interval`,
    in the bounded cache above;
  - stays ready, and makes the switch visible through a log line or metric.
- **Level changes.** When a write changes a flag's effective level relative to a
  session, the next DefinitionUpdate switches the item between `item_kind` 0 and
  2, in either direction. That write (to the flag or to a segment it references)
  raises the item's effective revision (§8.3), which both item kinds carry, so
  the switch passes the client's revision check.

### 8.9 Error and Reject

#### Error (0x50)

It reports a connection-level failure. The server closes the connection
after sending it.

```text
[u8 0x50]
[u16 code]
[u16 msg_len][msg]
```

#### Reject (0x31)

It rejects a single request. The connection and the session's other
subscriptions stay open.

```text
[u8 0x31]
[u8 ref_kind]               // 0 query_id (Query or DefinitionQuery), 1 subscription_id
[u32 ref_id]
[u16 code]
[u16 msg_len][msg]
```

- **Ending a subscription.** Only code 110 ends an active subscription. Any
  other Reject rejects only the request it refers to, even when its `ref_id`
  equals an active subscription (for example a duplicate id that gets 114). That
  subscription keeps running.

All codes are listed in §2.5.

---

## 9. Binary encoding

### 9.1 Design rules

- **No field names on the wire.** The schema version fixes the field order.
  - Each definition starts with `[u8 schema_version][u8 eval_level]`, so a
    cached copy describes itself. SDKs may persist definitions in exactly this
    form as their last-known-good store.
  - Adding, removing or reordering fields bumps `schema_version`. There are no
    optional or extension fields within a schema version.
- **Integer widths follow the limits** in §2.2.
- **References are indexes.** Variations are referenced by their `u8` index into
  the flag's variation table. Segments are referenced by key.
- **Value encoding.** Flag values are encoded by flag type. The same encoding is
  used for the `value` field of QueryResponse and Update, where `value_type`
  selects it and no extra length prefix is added:
  - `bool`: 1 byte (`00` or `01`).
  - `int`: 8 bytes, two's-complement int64.
  - `string`: `[u32 len][UTF-8]`.
  - `json`: `[u32 len][UTF-8 JSON text]`. The content is opaque application
    data, in **normal form**: minified, member names sorted ascending by UTF-8
    bytes, no duplicate member names, numbers kept exactly as written, and no
    trailing newline. Strings are escaped as Go's `encoding/json` writes them
    with HTML escaping off: `\"`, `\\`, `\b`, `\f`, `\n`, `\r` and `\t`; other
    characters below U+0020 as `\u00xx` (lowercase hex); U+2028 and U+2029 as
    `\u2028` and `\u2029`; everything else as raw UTF-8. The server normalizes
    a value on write and stores and sends only that form. SDKs never re-encode
    it, and the conformance vectors (§11) pin the exact bytes.
- **Condition value kinds.**
  - `0`: bool, 1 byte.
  - `1`: number, IEEE-754 float64, 8 bytes.
  - `2`: string, `[u16 len][UTF-8]`.
- **Self-tagging clauses.** The first byte of every clause is its opcode (§2.4).
  A decoder therefore always knows how to parse the rest of the clause.
  - Nesting needs no layout change.
  - Because the tag doubles as the operator byte, a tree costs no more bytes
    than a flat list.
- **Canonical form.** Every value has exactly one encoding, so encoders are
  byte-identical:
  - context attributes are sorted ascending by name bytes;
  - `salt_len` is 0 when the salt equals the flag key;
  - segment `included` and `excluded` targeting keys are sorted ascending by
    bytes (§9.4).
- **Bounded recursion.** A decoder enforces its level's maximum group depth
  before recursing (1 at level 1). A deeper tree is `malformed_flag`.
- **Strict decoding.** A decoder:
  - checks every length against the bytes remaining;
  - never reads past `def_len` or `context_len`;
  - rejects unknown enum values, out-of-range indexes and trailing bytes;
  - rejects reserved bits that are set, and fields marked "must be 0" that are
    not;
  - rejects float64 values that are NaN or ±Inf;
  - rejects strings that are not valid UTF-8;
  - rejects lists that break the canonical form: unsorted or duplicate context
    attribute names, or `included` / `excluded` entries;
  - rejects an explicit `salt` equal to the flag key, whose canonical form is
    `salt_len = 0` (the key comes from the item envelope, §9.2);
  - rejects anything that breaks a §2.2 limit or a §6 rule. Evaluators must not
    try to make sense of such input. In particular, a definition is invalid if
    it has:
    - an `op` whose `value_kind` does not fit it (§6.3): `lt`–`gte` need
      numbers; `contains`, `startsWith`, `endsWith` and `inSegment` need
      strings;
    - `inSegment` with `attr_len ≠ 0`, or any other leaf with `attr_len = 0`;
    - a condition with `value_count = 0`, or a group with `child_count = 0`;
    - a rule whose root clause is not a group;
    - a rollout with `bucket_by_len = 0` (the default is written out as
      `targetingKey`, as in §9.6);
    - duplicate variation ids, rule ids, or variations within one rollout;
    - a `boolean` flag whose variation table is not exactly `true`, `false`
      (§6.1).

  A definition that fails these checks is reported as `malformed_flag`. A
  context the server receives that fails them gets `Reject` 111. Any other
  message the server receives that fails them gets `Error` 115 (101 for the
  Handshake), except the two Subscribe cases listed under 115 in §2.5. A
  client that receives such a message closes the connection.

### 9.2 Message items

A flag's or segment's key and revision travel in the message item envelope
(§8.5), not inside the definition. The environment is implied by the
credential.

### 9.3 Flag definition

```text
[u8 schema_version]              // 1
[u8 eval_level]                  // 1
[u8 type]
[u8 flag_bits]                   // bit0 enabled
[u16 salt_len][salt]             // 0 ⇒ salt = flag key
[u8 variation_count]
repeated variation:
  [u8 id_len][id]
  [value]                        // per type, §9.1
[u8 off_variation]               // index
[serve fallthrough]
[u16 rule_count]
repeated rule:
  [u8 id_len][id]
  [group]                        // root clause; must be a group
  [serve]

clause: first byte is the opcode
  group (0x80 all, 0x81 any):
    [u8 opcode]
    [u8 child_count]
    repeated [clause]            // level 1: every child is a leaf
  leaf (0x00–0x7F):
    [condition]

condition:
  [u8 op]                        // opcode
  [u8 cond_bits]                 // bit0 negate
  [u8 attr_len][attribute]       // 0 for inSegment
  [u8 value_kind]                // inSegment uses 2 (segment keys)
  [u16 value_count]
  repeated [value]

serve:
  [u8 serve_kind]                // 0 variation, 1 rollout
  if variation: [u8 variation_index]
  if rollout:
    [u8 bucket_by_len][bucket_by]
    [u8 weight_count]
    repeated: [u8 variation_index][u32 weight]   // sum = 100000
```

### 9.4 Segment

```text
[u8 schema_version]              // 1
[u8 eval_level]                  // 1
[u16 included_count]
repeated: [u16 len][targeting key] // sorted ascending by bytes
[u16 excluded_count]
repeated: [u16 len][targeting key] // sorted ascending by bytes
[u8 rule_count]
repeated rule:
  [u8 id_len][id]
  [group]                        // as in §9.3; op 0x08 (inSegment) is invalid
```

`included` and `excluded` are sorted, so an SDK can binary-search them in
place. The search must compare raw UTF-8 bytes. Native string comparison in
JavaScript and Java compares UTF-16 code units, which orders characters from
U+E000 upward differently from characters outside the Basic Multilingual Plane,
so it can miss targeting keys that are present.

### 9.5 Context

```text
[u16 targeting_key_len][targeting_key]
[u8 attr_count]
repeated attribute:              // sorted ascending by name bytes
  [u8 name_len][name]
  [u8 value_kind]                // bit7 set ⇒ list
  if scalar: [value]
  if list:   [u16 count] repeated [value]   // homogeneous; element value kind = value_kind & 0x7F
```

### 9.6 Worked example: the §6.1 flag

The flag in §6.1 encodes to **134 bytes**. The same content as minified JSON,
without the fields the envelope carries (`key`, `environmentId`, `revision`) and
the human-only fields (rule `description`, segment `name`), is 598 bytes. For
comparison:

- the §6.2 segment is 64 bytes, against 190 as JSON;
- the §6.6 context is 58 bytes, against 101 as JSON.

```text
01                                  schema_version 1
01                                  eval_level 1
02                                  type string
01                                  enabled
00 00                               salt_len 0 ⇒ salt = "checkout.new-flow"
02                                  2 variations
07 63 6f 6e 74 72 6f 6c             [0] id "control"
00 00 00 02 76 31                       value "v1"
09 74 72 65 61 74 6d 65 6e 74       [1] id "treatment"
00 00 00 02 76 32                       value "v2"
00                                  offVariation [0]
00 00                               fallthrough: variation [0]
00 02                               2 rules
0c 62 65 74 61 2d 74 65 73 74 65 72 73    rule "beta-testers"
80 01                                     group all, 1 child
08 00 00 02 00 01 00 04 62 65 74 61       inSegment, values ["beta"]
00 01                                     serve variation [1]
0a 62 65 6e 65 6c 75 78 2d 35 30          rule "benelux-50"
81 01                                     group any, 1 child
00 00 07 63 6f 75 6e 74 72 79             in, attribute "country"
02 00 03 00 02 4e 4c 00 02 42 45 00 02 4c 55   strings ["NL","BE","LU"]
01 0c 74 61 72 67 65 74 69 6e 67 4b 65 79   serve rollout, bucketBy "targetingKey"
02 00 00 00 c3 50 01 00 00 c3 50          [0]→50000, [1]→50000
```

---

## 10. Frame examples

Every example below is a complete frame: the length prefix followed by the
payload.

**Handshake.** Version 0.1; `CAP_DEFINITIONS` with `EVAL_LEVEL` 1
(capabilities `0x0101`); token `tok-ABC`; no client id.

```text
00 00 00 10                 length 16
01                          Handshake
00 01                       version 0.1
01 01                       capabilities: EVAL_LEVEL=1, CAP_DEFINITIONS
00 07 74 6f 6b 2d 41 42 43  token "tok-ABC"
00 00                       no client id
```

**HandshakeAccept.** Session id `00..0f`; definitions granted; heartbeat every 15
seconds; credential check every 30 seconds; sync timeout 30 seconds.

```text
00 00 00 1a  02  00 01
00 01 02 03 04 05 06 07 08 09 0a 0b 0c 0d 0e 0f
01           server_flags: CAP_DEFINITIONS granted
00 0f        heartbeat_interval_s = 15
00 1e        credential_check_interval_s = 30
00 1e        sync_timeout_s = 30
```

**Control(SESSION_ESTABLISHED).** Normal status; the token is valid for 3,600
seconds.

```text
00 00 00 0b  30  00  00 00 00 00  00  00 00 0e 10
```

**Control(CHECK) and Control(CHECK_OK).** The client sends `ref_id` 7. The
server replies degraded, with 1,200 seconds left on the token.

```text
00 00 00 0b  30  01  00 00 00 07  00  00 00 00 00
00 00 00 0b  30  02  00 00 00 07  01  00 00 04 b0
```

**Reauthenticate and Control(REAUTH_OK).** New token `tok-XYZ`, valid for 3,600
seconds.

```text
00 00 00 0a  03  00 07 74 6f 6b 2d 58 59 5a
00 00 00 0b  30  03  00 00 00 00  00  00 00 0e 10
```

**Query.** `query_id` 7; context `{targetingKey:"u1"}`; flag key `f.a`.

```text
00 00 00 15  10  00 00 00 07
00 00 00 05  00 02 75 31 00          context: targeting key "u1", 0 attributes
00 01  00 03 66 2e 61                1 flag key: "f.a"
```

**QueryResponse.** `f.a` is boolean `true` via `fallthrough` at revision 5;
`f.b` was not found. A single frame (`MORE` clear).

```text
00 00 00 31  11  00 00 00 07  00  00 02
00 03 66 2e 61  00 00 00 00 00 00 00 05  02 00  00  01  04 74 72 75 65  00
00 03 66 2e 62  00 00 00 00 00 00 00 00  03 00  ff      00              00
```

**Subscribe.** Definition mode; `subscription_id` 1; criterion `checkout.*`.

```text
00 00 00 18  20  00 00 00 01  01  00 00 00 00  00 01  00 0a 63 68 65 63 6b 6f 75 74 2e 2a
```

**Update.** Value mode; `subscription_id` 2; sequence 1; a single-frame snapshot
with one upsert.

```text
00 00 00 28  22  00 00 00 02  00 00 00 00 00 00 00 01  03  00 01
00  00 03 66 2e 61  00 00 00 00 00 00 00 05
02 00 00  01  04 74 72 75 65  00
```

**DefinitionUpdate.** `subscription_id` 1; a single-frame snapshot carrying the
§6.2 segment and then the §6.1 flag. The payload is 267 bytes.

```text
00 00 01 0b  23  00 00 00 01  00 00 00 00 00 00 00 01  03  00 02
00 01  00 04 62 65 74 61  00 00 00 00 00 00 04 0e  00 00 00 40  <64 bytes, §9.4>
00 00  00 11 63 68 65 63 6b 6f 75 74 2e 6e 65 77 2d 66 6c 6f 77
       00 00 00 00 00 00 04 12  00 00 00 86  <134 bytes, §9.6>
```

**Unsubscribe.** `subscription_id` 1.

```text
00 00 00 05  21  00 00 00 01
```

**Reject.** Subscription 1 rejected with `RejInvalidCriteria` (112).

```text
00 00 00 1a  31  01  00 00 00 01  00 70  00 10 69 6e 76 61 6c 69 64 20 63 72 69 74 65 72 69 61
```

**Heartbeat.**

```text
00 00 00 01  40
```

**Error.** `ErrAuthenticationFailed` (105).

```text
00 00 00 1a  50  00 69  00 15 61 75 74 68 65 6e 74 69 63 61 74 69 6f 6e 20 66 61 69 6c 65 64
```

---

## 11. Conformance

A language-neutral vector set under `docs/fqdp-vectors/` holds four categories
of case:

- **Evaluation cases.** Each case is a flag or segment plus one or more
  contexts, given both as JSON and as expected binary hex, together with the
  expected `{variationId, reason, ruleId, errorKind}`. Together the cases cover:
  - every operator;
  - `negate` with missing attributes;
  - `negate` on `inSegment` (matches non-members);
  - `bucketBy` on a number attribute of `-0` (bucketed as `"0"`);
  - `bucketBy` on a number attribute of 2^60 (unusable, bucket 0);
  - `bucketBy` on a boolean attribute and on a list attribute (both unusable,
    bucket 0);
  - a segment whose `included` keys mix a character from U+E000 upward with a
    character outside the Basic Multilingual Plane (byte-order binary search,
    §9.4);
  - list attributes, including an empty list with `negate` (counts as missing)
    and a non-empty list with `negate` (inverts the overall result);
  - segment precedence;
  - `all` and `any`;
  - rule order;
  - the kill switch;
  - the bucketing vectors (§6.5);
  - a bucket exactly on a cumulative weight boundary (it gets the next
    variation);
  - a rollout with a zero-weight variation (never served);
  - a `json` variation containing `<`, a tab and U+2028, with its exact
    normal-form bytes (§9.1).
- **Malformed binary cases.** Each of these must decode as `malformed_flag`:
  - truncated lengths;
  - a bad index;
  - an unknown op;
  - trailing bytes;
  - a nested group at level 1;
  - a NaN or ±Inf condition value;
  - an unsorted segment `included` list;
  - a set reserved bit in `flag_bits`;
  - a string that is not valid UTF-8;
  - `lt` with string values;
  - a condition with zero values;
  - a group with zero children;
  - a rollout with `bucket_by_len 0`;
  - rollout weights that sum to something other than 100000;
  - a weight above 100000 whose `u32` sum wraps to 100000 (`4294967295` and
    `100001`);
  - an explicit salt equal to the flag key.
- **Frame cases.** The frames in §10, plus a level-2 flag offered to a level-1
  session, which must arrive as `item_kind 2`.
- **Session case.** The Appendix B session, replayed in order: a server must
  send exactly the S→C frames shown, and an SDK must reach the evaluation
  results shown.
- **Lifecycle cases.** Each of these must produce the result shown:

  | Case | Result |
  | --- | --- |
  | Request before `SESSION_ESTABLISHED` | 109 |
  | Handshake for version 0.2 whose body is not valid 0.1 | 104 |
  | C→S frame with `length` over `MaximumClientFrameSize`, before the Handshake | 108 |
  | Handshake with a credential without `flags:read` | 105 |
  | Second Handshake, before or after `SESSION_ESTABLISHED` | 121 |
  | Client sends an Update (S→C type) | 100 |
  | Client sends `Control(CHECK_OK)` | 115 |
  | `query_id` not higher than the previous one | `Reject` 122 |
  | Reauthenticate with another credential's token | 118 |
  | Token expiry without Reauthenticate | 116 |
  | Revocation found by the server-side check | 106 |
  | Revocation found by `Control(CHECK)` | 106 |
  | Unknown `control_kind` | 115 |
  | Subscribe with `mode` 2 | `Reject` 113 |
  | Definition-mode Subscribe with `context_len > 0` | `Reject` 111 |
  | Heartbeat before `SESSION_ESTABLISHED` | accepted; no Error sent |
  | Reauthenticate with a token without `flags:read` | 105 |
  | Subscribe that matches nothing | one `SNAPSHOT\|SNAPSHOT_END` frame, `change_count = 0` |
  | Reauthenticate that drops `flags:definitions:read` | `Reject` 110 for every definition subscription |
  | `flags:definitions:read` removed from the credential in the store | `Reject` 110 for every definition subscription |
  | `flags:read` removed from the credential in the store | 106 |
  | Handshake with `CAP_DEFINITIONS` and `EVAL_LEVEL` 0 | 101 |
  | HandshakeAccept with `credential_check_interval_s` or `sync_timeout_s` 0 | client closes the connection |
  | No `SESSION_ESTABLISHED` or 117 within `sync_timeout_s` + `heartbeat_interval_s` | client closes the connection |
  | A flag stops, then resumes, referencing a segment whose revision is unchanged | the segment upsert is applied |

The Go server (evaluator and codec) and every SDK run the same vectors. That is
the guarantee that both modes give identical answers and that every encoder is
byte-compatible.

---

## 12. Security considerations

- **Tokens.** The Handshake token is a short-lived bearer token: the service
  token from the HTTP API's OAuth2 client-credentials endpoint. Human login
  tokens are not accepted (`Error` 105), because they can span several
  environments.
  - Scopes decide what the session may do: `flags:read` for value mode, and
    `flags:definitions:read` as well for definition mode.
  - The credential's environment decides which flags the session sees.
- **Token lifetime.** A session outlives its token only through Reauthenticate
  (§4.3). An expired token gets `Error` 116.
- **Token logging.** Servers and SDKs never log tokens, in whole or in part. A
  token is a bearer credential, so anyone who reads it from a log can use it
  until it expires.
- **Revocation and scope removal.** Both take effect through the two-sided check
  (§4.4): no later than `credential_check_interval` after the change, even on a
  push-only session. Revocation, or losing
  `flags:read`, gets `Error` 106 and the connection is closed. Losing
  `flags:definitions:read` ends definition subscriptions with `Reject` 110.
- **Definition exposure.** Definition mode exposes every rule and every segment
  member list in the environment. Do not give `flags:definitions:read` to
  less-trusted clients.
- **TLS.** Strongly recommended in production. Servers may present certificates
  from a public CA, a private CA, or **self-signed** certificates. Clients and
  SDKs must support both:
  - **Custom trust roots:** a configurable CA bundle, used instead of or
    alongside the system roots.
  - **Certificate pinning:** trusting a specific server certificate or server
    public key (SPKI) by its SHA-256 fingerprint.

  Turning verification off entirely may be offered for local development only,
  and SDKs must log a warning when it is used. SDKs must log the same warning
  when they connect without TLS, since the token then travels in clear text.
- **Abuse limits.**
  - Rate-limit new and unauthenticated connections per IP, and requests per
    session (including `Control(CHECK)`). The response is `Error` 107.
    Per-session request limits must leave room for the Queries SDKs send for
    server-evaluated flags (§8.8).
  - Enforce `MaximumClientFrameSize` before reading a payload, so a peer that
    hasn't authenticated cannot make the server hold large buffers (`Error`
    108).
  - Enforce the handshake timeout (`Error` 120).
  - Cap active subscriptions per session (`Reject` 123).
- **Decoders.** All decoders follow the strict-decoding and bounded-recursion
  rules in §9.1.

---

## 13. Implementation notes

These notes are not normative.

- **Durability.** Flag and segment state lives in the Raft-replicated store; the
  protocol only delivers it. Revisions are Raft log indexes.
- **Shared package.** One Go package (proposed: `backend/internal/flageval`)
  holds the model, write-time validation (§6), evaluation (§7) and the codec
  (§9). The HTTP API validates with it, and the FQDP server evaluates and
  encodes with it.
- **Encode once.** The server encodes each flag and segment once per write, and
  caches the result by `(environment, key, revision)`, so fan-out to subscribers
  is a plain copy.
- **Consistent snapshots.** Build a snapshot from one store view, and register
  the subscription for deltas at that same index before releasing the view
  (§8.6).
- **HTTP API.**
  - Flags are read and written as the §6.1 JSON document.
  - A full-definition `PUT` carries the `revision` it was based on and gets
    `409` if that revision is stale.
  - Segments get their own CRUD endpoints and permissions.
  - The credential scope catalog gains `flags:definitions:read`.
- **JSON normal form in Go.** To produce the §9.1 normal form, decode with
  `json.Decoder.UseNumber` (so large integers keep every digit) and encode with
  `SetEscapeHTML(false)` (so `<`, `>` and `&` are not rewritten). Go's encoder
  already writes a map's members sorted by name bytes. `Encoder.Encode`
  appends a newline; strip it.
- **Audit.** Audit before/after values are whole definitions, which gives diffs
  and reverts a natural unit.
- **Store migration.** The model changes break the store's command and Raft
  snapshot format. Under the pre-release stance, the Raft data directory is
  wiped on upgrade.
- **Session registry and health.** The server keeps each session's lifecycle
  state and last check results (§4). Healthy, expiring and total session counts
  feed the server's own health reporting, such as a cluster status endpoint and
  the admin UI's cluster health page.
- **Revocation push.** Applying a credential change in the store, including a
  scope change, triggers the server-side credential check. A periodic sweep
  covers anything the trigger misses.
- **Sync.** `SESSION_ESTABLISHED` is the hook for node sync. A node that is not
  in sync with the cluster leader withholds it and eventually sends `Error` 117.
  A node that is draining ends its sessions with `Error` 124.
- **Debugging.** During development, a decoding tool (for example `fqdpdump`)
  built on the reference codec takes the place of a text encoding.
- **Current code.** `backend/internal/fqdp/message_types.go` still defines an
  Admin message type (`0xF0`), and still names 0x30 and 0x31 `MessageTypeAck`
  and `MessageTypeNack`. The Admin type should be removed, and the other two
  renamed to Control and Reject, when the protocol is implemented.
  `backend/internal/fqdp/errors.go` still defines the retired codes 102 and 103.
  `MessageTypeHandshakeAck` should be renamed `MessageTypeHandshakeAccept`, and
  the constants for Reject codes use the `Rej` prefix (§2.5).

---

## 14. Not in 0.1

- **Application-originated writes** over FQDP, and change-attribution metadata
  (actor, comment, origin).
- **Flag prerequisites** and dependencies between flags.
- **Rule-language extensions**, which arrive as later evaluator levels:
  - nested condition groups and the `not` group;
  - regex, semver and date operators.
- **Larger targeting models:** composite contexts, and segments beyond the §2.2
  limits ("big segments").
- **Revision-based delta resume** on reconnect.
- **A text/JSON encoding.** Use the decoding tool instead (§13).
- **An Admin channel.** The earlier `0xF0` type is removed. Operational actions
  go through the HTTP API.

---

## Appendix A. Glossary

This appendix is not normative. It explains the terms the spec uses, for readers
who have not worked with binary protocols before. Where a term is defined in the
spec itself, the **See** column points there; the spec wins if the two ever
disagree.

### A.1 How to read a byte layout

FQDP sends bytes, not text. Every message is written in this spec as a list of
fields, each in square brackets: `[u32 subscription_id]` means "the next 4
bytes are an unsigned number called `subscription_id`". Fields follow each
other with no gaps, separators or names. The reader knows where each field ends
only because the layout says how wide it is.

Example: the Unsubscribe frame from §10, read byte by byte.

```text
00 00 00 05   [u32 length]           = 5: five payload bytes follow
21            [u8 MessageType]       = 0x21: Unsubscribe
00 00 00 01   [u32 subscription_id]  = 1
```

Each pair of hex digits is one byte. A u32 takes four bytes and is read
most-significant byte first (big-endian), so `00 00 00 05` is 5 and
`00 00 0e 10` is 3,600.

Some fields hold several yes/no switches, one per bit. Bits are numbered from
the lowest-value bit: bit 0 has value 1, bit 1 has value 2, bit 8 has value 256
(`0x0100`). In the Handshake example (§10), capabilities `01 01` is the u16
`0x0101` = bit 0 (`CAP_DEFINITIONS`) plus the value 1 in bits 8–11
(`EVAL_LEVEL` = 1).

### A.2 Binary encoding terms

| Term | Meaning | See |
| --- | --- | --- |
| `[type name]` | One field in a byte layout: its type (how many bytes and how to read them), then its name. | §1 |
| `[uN len][bytes]` | A length-prefixed field: first a uN saying how many bytes follow, then those bytes. Strings are always written this way. | §3 |
| `repeated:` | The fields below it appear as many times as the count field just before says. | §8 |
| `if X:` | The fields below it appear only when the condition holds; otherwise they take no bytes at all. | §8.3, §9.3 |
| Bit | The smallest unit: 0 or 1. Eight bits make a byte. | A.1 |
| Bit 0, bit N | Bit numbering, counted from the lowest-value bit. Bit N has value 2^N. | A.1 |
| Bit field, flag bits | A u8 or u16 whose individual bits each mean something on their own (`flag_bits`, `update_flags`, `cond_bits`, `client_capabilities`, `server_flags`, `response_flags`). Combined with OR: `SNAPSHOT \| SNAPSHOT_END` is `0x03`. | §2.4 |
| Byte | 8 bits; a number from 0 to 255 (`0x00`–`0xFF`). Also called an octet. | — |
| Big-endian | The most significant byte comes first: the u16 300 is `01 2c`. Also called network byte order. All FQDP integers are big-endian. | §3 |
| Hex, `0x…` | Base-16 notation. Two hex digits are one byte: `0x2a` = 42. Frame dumps in §9.6 and §10 are hex. | — |
| `u8`, `u16`, `u32`, `u64` | Unsigned integers of 1, 2, 4 and 8 bytes. Largest values: 255; 65,535; 4,294,967,295; about 1.8 × 10^19. | §3 |
| `i64` | A signed 8-byte integer in two's complement. Used for `int` flag values. | §9.1 |
| Two's complement | The usual way to store negative integers: −1 is all `FF` bytes, −2 ends in `FE`. Every mainstream language uses it. | §9.1 |
| `float64`, IEEE-754 | The standard 8-byte floating-point number (a "double"). All numbers in conditions and contexts use it. | §6.3, §9.1 |
| NaN, ±Inf | Special float64 values meaning "not a number" and infinity. Never valid in FQDP. | §6.3 |
| `-0` | float64 has a separate negative zero. FQDP treats it as equal to `0`, and buckets it as `"0"`. | §6.3, §6.5 |
| ±(2^53−1) | 9,007,199,254,740,991: the largest integer a float64 (and JavaScript) stores exactly. The safe range for integer values. | §2.2 |
| KiB, MiB | 1,024 bytes and 1,048,576 bytes. | §2.1 |
| UTF-8 | The byte encoding of Unicode text used for every FQDP string. ASCII characters take 1 byte; others take 2–4. | §3 |
| Valid UTF-8 | A byte sequence that follows UTF-8's rules. Invalid sequences are rejected rather than repaired. | §9.1 |
| UTF-16, code unit | The internal string form of JavaScript and Java, in 2-byte units. Comparing UTF-16 strings can give a different order than comparing their UTF-8 bytes. | §9.4 |
| BMP | Basic Multilingual Plane: Unicode characters up to U+FFFF. Characters beyond it (such as most emoji) take 4 UTF-8 bytes. | §9.4 |
| Sorted ascending by bytes | Compare two strings byte by byte as unsigned numbers; the first difference decides, and a shorter prefix sorts first. | §9.1 |
| Count field | A field such as `[u16 key_count]` that says how many repeated items follow. | §8 |
| Index | A position in a table, counting from 0. Variations are referenced by index instead of by id. | §9.1 |
| Enum value | A number that stands for one of a fixed set of choices, such as `mode` 0 = value, 1 = definition. Unlisted numbers are invalid. | §2.4 |
| Reserved | A bit, value or code that has no meaning yet but may get one later. Senders set reserved bits to 0, and receivers reject a set one. | §2.4, §9.1 |
| Must be 0 | A field that has to hold 0 in this case, such as `server_status` in a C→S Control. | §4.4, §9.1 |
| Truncated | The data ends before a field the layout promises. | §9.1 |
| Trailing bytes | Bytes left over after the last field the layout describes. Rejected. | §9.1 |
| Encoder, decoder, codec | Code that turns a model into bytes (encoder), bytes back into a model (decoder), or both (codec). | §9 |
| Strict decoding | Rejecting anything that is not exactly valid, instead of guessing. It keeps SDKs in different languages in agreement. | §9.1 |
| Canonical form | The one allowed encoding of a value, so two correct encoders produce the same bytes. | §9.1 |
| Byte-identical | Two encodings are exactly the same bytes. | §1, §9.1 |
| Normal form (JSON) | The single text form the server stores `json` flag values in: minified, member names sorted, numbers as written. | §9.1 |
| Member name | The name of a field in a JSON object (RFC 8259's term). | §9.1 |
| Opaque | Carried but never interpreted by FQDP, such as `json` values and the session id. | §9.1, §4.2 |
| Self-tagging | Each clause starts with a byte (its opcode) that says what form of clause it is, so a decoder knows how to read the rest. | §9.1 |
| Opcode | The first byte of a clause: an operator for a leaf, or `all` / `any` for a group. | §2.4 |
| Bounded recursion | A decoder refuses to nest deeper than its level allows, so hostile input cannot exhaust its stack. | §9.1 |
| SHA-256, digest | A standard hash function, and its 32-byte output. Used for bucketing and certificate fingerprints. | §6.5, §12 |
| `digest[0:8]` | The first 8 bytes of the digest (positions 0 to 7). | §6.5 |
| `mod` | The remainder after division: `74356 = n mod 100000`. | §6.5 |
| Conformance vectors | Shared test cases (inputs plus expected bytes and results) that every implementation must pass. | §11 |

### A.3 Connections and framing

| Term | Meaning | See |
| --- | --- | --- |
| TCP | The reliable, ordered byte stream FQDP runs on. It delivers bytes in order but knows nothing about message boundaries. | §3 |
| Partial read / write | One read from a TCP socket can return part of a frame, or several frames; one write can send only part. Code must loop until a whole frame is in. | §3 |
| Frame | One complete FQDP message on the wire: `[u32 length][payload]`. | §3 |
| Length prefix | The `[u32 length]` that starts every frame, so the reader knows how many bytes to wait for. It counts the payload only. | §3 |
| Payload | Everything in a frame after the length: the message type byte plus the body. | §3 |
| Body | The fields of a message after its message type byte. | §3 |
| Message type | The first payload byte; it says which message this is (`0x10` Query, and so on). | §2.3 |
| `MaximumFrameSize` | The largest payload the server sends: 10 MiB. | §2.1, §3 |
| `MaximumClientFrameSize` | The largest payload a client may send: 256 KiB. Checked before the server reads the payload. | §2.1, §3 |
| Zero-length frame | A frame whose length is 0. Ignored. | §3 |
| `C→S`, `S→C` | Sent by the client to the server, or by the server to the client. | §1 |
| TLS, direct TLS | Encryption for the connection. "Direct" means TLS starts as soon as TCP connects, before any FQDP byte. | §3 |
| STARTTLS | Starting in plain text and switching to TLS later. Not used by FQDP. | §3 |
| Plaintext connection | A connection without TLS; the token can be read by anyone on the network path. | §12 |
| CA, trust roots, CA bundle | Certificate authorities a client trusts to vouch for a server certificate, and the file listing them. | §12 |
| Self-signed certificate | A certificate no CA vouches for. Trusted only through a custom bundle or pinning. | §12 |
| Certificate pinning, fingerprint | Trusting one exact certificate or server public key (SPKI), identified by its SHA-256 hash (the fingerprint). | §12 |
| Port 9001 | The default TCP port. | §3 |

### A.4 Sessions and authentication

| Term | Meaning | See |
| --- | --- | --- |
| Connection | One TCP (and TLS) link between a client and a server node. | §3 |
| Session | The server-side state for one connection: its token, ids, subscriptions and checks. One per connection. | §3, §4 |
| Session id | A 16-byte opaque id the server assigns in the HandshakeAccept, mainly for logs. | §4.2 |
| Session states | CONNECTED (TCP up, no Handshake yet), AUTHENTICATED (token accepted, HandshakeAccept sent, node not yet confirmed in sync), ESTABLISHED (`SESSION_ESTABLISHED` sent; requests allowed), CLOSED. | §4.1 |
| Handshake, HandshakeAccept | The client's first message (version, capabilities, token) and the server's acceptance reply. | §4.2 |
| `SESSION_ESTABLISHED` | The Control message that moves a session to ESTABLISHED. Only after it may the client send requests. | §4.1, §4.4 |
| Request | Any C→S message other than Handshake and Heartbeat: Query, DefinitionQuery, Subscribe, Unsubscribe, Reauthenticate, `Control(CHECK)`. Only allowed once the session is ESTABLISHED. | §4.1 |
| Handshake timeout | How long the server waits for a Handshake after the connection opens. | §4.1 |
| Sync timeout, `sync_timeout_s` | How long a node may take to get in sync before it sends `Error` 117. Announced in the HandshakeAccept, so the client knows how long to wait. | §4.1, §4.2 |
| In sync | A node that has caught up with the cluster leader, so it serves consistent data. | §4.1, §13 |
| Ready | An SDK that has received `SNAPSHOT_END` for every initial subscription. Only SDKs are "ready". | §8.5 |
| Token | The string the client presents to prove who it is. | §4.2, §12 |
| Bearer token | A token that works for whoever holds it, like a key. That is why it must never be logged. | §12 |
| Service token | A token issued to an application, not a person, through the HTTP API's OAuth2 client-credentials endpoint. The only token FQDP accepts. | §4.2, §12 |
| Application credential | The account a service token belongs to. Scoped to exactly one environment; holds the scopes. | §4.2 |
| Client-credentials (OAuth2) | The standard flow where an application trades its credential for a short-lived token. | §4.3, §12 |
| Scope | A permission held by a credential. `flags:read` allows value mode; `flags:definitions:read` also allows definition mode. | §4.2, §12 |
| Token rotation, Reauthenticate | Replacing a session's token with a fresh one without reconnecting. | §4.3 |
| `token_ttl_s` | Seconds until the session's current token expires. | §4.4 |
| Credential check | Re-validating that the session's credential still exists, is active, and holds its scopes. Done by both sides. | §4.4 |
| `CHECK`, `CHECK_OK` | The client's request for a credential check, and the server's "still valid" reply. | §4.4 |
| `credential_check_interval` | How often a session's credential is re-checked. Set by the server and announced in the HandshakeAccept. | §4.2, §4.4 |
| Revocation | An admin disabling a credential, or removing its scopes. | §4.4, §12 |
| Capability | An optional feature the client asks for in the Handshake and the server grants or refuses (`CAP_DEFINITIONS`). | §4.2 |
| Granted | A capability the server accepted, shown by its bit in `server_flags`. | §4.2 |
| `client_id` | A free-form label the client sends (such as `checkout-service@1.4.2`) to help operators. No protocol meaning. | §4.2 |
| Heartbeat | An empty message sent when a peer has been quiet, to show the connection is alive. | §4.6 |
| `heartbeat_interval_s` | How long a peer may stay silent before it sends a Heartbeat. Set by the server. | §4.6 |
| Dead peer | A peer from which no bytes have arrived for 3 × `heartbeat_interval_s`. The connection is closed. | §4.6 |
| Peer | Either end of the connection, client or server. | §4.6 |
| Server status | `normal`, or `degraded`: still serving but possibly with stale data. Sent in Control. | §4.5 |
| Session health | Whether a session is **healthy**: ESTABLISHED, has an unexpired token, and passed its last check. | §4.5 |
| Expiring session | An ESTABLISHED session inside its re-authentication window. | §4.3, §4.5 |
| Draining | A node that is shutting down and asks clients to go elsewhere (`Error` 124). | §13 |
| Rate limit | A cap on how many connections or requests are allowed per time unit. | §12 |

### A.5 Messages and delivery

| Term | Meaning | See |
| --- | --- | --- |
| Value mode | The server evaluates flags and sends results. | §5 |
| Definition mode | The server sends flag definitions, and the SDK evaluates them locally. | §5 |
| Query, QueryResponse | A one-shot value-mode request and its results. | §8.1 |
| DefinitionQuery, DefinitionResponse | A one-shot definition-mode fetch and its definitions. | §8.5 |
| Subscribe, Unsubscribe | Start or stop a stream of pushed results (value mode) or definitions (definition mode). | §8.2 |
| Subscription | One active Subscribe: its criteria, mode and (value mode) context. | §8.2 |
| Update, DefinitionUpdate | The pushed frames of a value-mode or definition-mode subscription. | §8.3, §8.5 |
| `query_id` | A client-chosen number that ties a QueryResponse or Reject to its Query or DefinitionQuery. | §8.1 |
| `subscription_id` | A client-chosen number that names a subscription in its frames and Rejects. | §8.2 |
| Strictly increasing | Each new id must be higher than every earlier one in the session; reuse is rejected. | §8.1, §8.2 |
| Id space | The set of ids that must not collide. Query and DefinitionQuery share one; subscriptions have their own. | §8.1 |
| `ref_kind`, `ref_id` | In a Reject, which sort of id it refers to and the id itself. In a Control message, `ref_id` echoes a `CHECK`. | §8.9, §4.4 |
| Criterion, criteria | Which flags a request covers: an exact flag key, a prefix ending in `*`, or `*` for all. | §8.4 |
| Prefix criterion | A criterion like `checkout.*` that matches every flag key starting with `checkout.`. | §8.4 |
| Snapshot | The first frames of a subscription: everything that currently matches. | §8.6 |
| `SNAPSHOT`, `SNAPSHOT_END` | Bits in `update_flags` marking snapshot frames and the last one. | §8.6 |
| Delta | A later change after the snapshot: one flag or segment added, changed or removed. | §8.6 |
| Upsert | "Update or insert": set this item to the given value whether or not the client already had it. | §2.4 |
| Remove (change) | The item leaves the subscription's view: it was deleted from the store, or (for a segment) no flag in the subscription needs it any more. | §8.5 |
| `change_kind` | Whether a change is an upsert or a remove. | §2.4 |
| `item_kind` | What an item is: a flag, a segment, or a server-evaluated flag. | §2.4, §8.5 |
| `MORE` | A bit meaning "more frames for this `query_id` follow". | §8.1 |
| Sequence number | A per-subscription frame counter: 1, 2, 3… A gap means a bug, and the client disconnects. | §8.6 |
| Revision | The Raft log index of the last write to a flag or segment. Higher means newer. | §6.1, §8.6 |
| Effective revision | For a flag: the highest of its own revision and those of the segments it references. Lets segment edits count as flag changes. | §8.3 |
| Segment closure | The rule that a definition response always includes every segment its flags need. | §8.5 |
| Dangling reference | A reference to something the client does not have, such as a missing segment. | §8.5 |
| Server-evaluated flag | A flag too new for the SDK's evaluator; sent as key and revision only (`item_kind 2`), and evaluated with a Query. | §8.8 |
| Last-known-good (LKG) | Definitions or values an SDK saved earlier, used when it cannot reach a server. | §8.6, §9.1 |
| Backfill, resume | Replaying missed changes after a reconnect. FQDP does not do this; a fresh snapshot replaces it. | §8.6, §14 |
| Error | A connection-level failure. The server sends it, then closes the connection. | §8.9 |
| Reject | One request was rejected, and the connection stays open. Only code 110 also ends an active subscription. | §8.9 |
| Control | A multi-purpose session message: session established, credential check, re-auth result, server status. | §4.4 |
| `control_kind` | Which of those purposes a Control message serves. | §4.4 |
| Round-trip | Sending a request and waiting for the reply. Value-mode Queries cost one; local evaluation costs none. | §5 |
| Fan-out | Sending the same change to many subscribers. | §13 |

### A.6 Flags and evaluation

| Term | Meaning | See |
| --- | --- | --- |
| Feature flag | A named switch whose value the application reads at run time, so behavior can change without a deploy. | §1 |
| Flag key | A flag's unique name within an environment, such as `checkout.new-flow`. Bare "key" in this spec always means a flag or segment key. | §2.2, §6.1 |
| Environment | An isolated set of flags and segments, such as `prod`. A credential sees exactly one. | §5 |
| Flag definition | Everything needed to evaluate a flag: type, variations, rules, rollouts. | §6.1 |
| Flag type | `boolean`, `integer`, `string` or `json`. Fixed when the flag is created. | §6.1 |
| Variation | One possible value of a flag, with an id (`control`, `treatment`). | §6.1 |
| Serve | What a rule or fallthrough returns: one variation, or a rollout. | §6.1 |
| Kill switch, `enabled` | When a flag is disabled, it serves `offVariation` and skips all rules. | §6.1 |
| `offVariation` | The variation served when the flag is disabled. | §6.1 |
| Fallthrough | What a flag serves when it is enabled but no rule matches. | §6.1, §7.1 |
| Rule | A condition tree plus a serve. Rules are tried in order; the first match wins. | §6.1 |
| Condition | A single test: an attribute, an operator, a list of values, and an optional `negate`. | §6.3 |
| Operator (`op`) | The test a condition applies: `in`, `contains`, `startsWith`, `endsWith`, `lt`, `lte`, `gt`, `gte`, `inSegment`. | §6.3 |
| `negate` | Invert a condition's result, but only when the attribute is present and of a fitting value kind. | §6.3 |
| Condition tree | The structure of a rule's conditions, made of clauses: groups containing leaves. | §6.4 |
| Clause | One element of a condition tree: a leaf or a group. | §6.4 |
| Leaf | A clause holding one condition. | §6.4 |
| Group | A clause that combines its children with `all` (every child true) or `any` (at least one true). | §6.4 |
| Root group | The top group of a rule, made from the rule's own `match` and `conditions`. | §6.4 |
| Depth | How deeply groups nest. Evaluator level 1 allows depth 1: the root group with leaves only. | §6.4 |
| Short-circuit | Stopping evaluation of a group once its result is known. | §6.4 |
| Segment | A reusable audience: explicit `included` / `excluded` targeting keys plus rules. | §6.2 |
| Membership | Whether a context belongs to a segment. | §6.2 |
| Context | What a flag is evaluated for: a `targetingKey` (usually a user or account id) plus attributes. It describes one subject. | §6.6 |
| Targeting key | The context's required unique id, `targetingKey` (the OpenFeature term). Conditions and `bucketBy` refer to it by that name. | §6.6 |
| Composite context | A context that describes several subjects at once. Not defined in 0.1. | §6.6, §14 |
| Attribute | A named fact about the context, such as `country` or `plan`. | §6.6 |
| Value kind | The kind of a condition value or context attribute: bool, number or string (`value_kind`). | §6.3, §9.1 |
| List attribute | An attribute that holds a list; a condition matches if any element matches. | §6.3 |
| Missing attribute | The context has no such attribute. A condition on it is false, and `negate` does not flip it. | §6.3 |
| Rollout | A serve that splits contexts between variations by weight. | §6.5 |
| Weight | A variation's share of a rollout, out of 100,000 (so 50,000 = 50%). | §6.5 |
| Bucket | A number from 0 to 99,999 computed from a context, which picks the variation in a rollout. | §6.5 |
| Bucketing | The hash calculation that turns a context into a bucket. The same context always lands in the same bucket. | §6.5 |
| `bucketBy` | The attribute whose value is hashed for bucketing. Defaults to `targetingKey`. | §6.5 |
| Salt | Text mixed into the bucketing hash. Defaults to the flag key; changing it reshuffles everyone. | §6.1, §6.5 |
| Evaluation | Running a flag's definition against a context to get a variation and a reason. | §7.1 |
| Evaluator | The code that performs evaluation, on the server or in an SDK. | §7 |
| Reason | Why evaluation gave its result: `flag_disabled`, `rule_match`, `fallthrough`, `flag_not_found`, `evaluation_error`. | §7.2 |
| Error kind | The detail behind reason `evaluation_error`, such as `malformed_flag`. | §7.2 |
| Malformed flag | A definition that fails strict decoding or validation. Evaluates to the fallback value. | §7.1, §9.1 |
| Fallback value | The value the application passes in, returned when evaluation cannot produce one. | §7.1 |
| SDK | A client library for one language that speaks FQDP for the application. | §5 |
| SDK-local | A reason or error kind that only an SDK produces; it never travels on the wire. | §7.2 |
| Write-time validation | Checks the HTTP API runs when a flag is saved, so bad definitions never reach the store. | §6, §13 |
| Audit entry | The record of who changed a definition, with before and after values. | §13 |

### A.7 Versions and cluster

| Term | Meaning | See |
| --- | --- | --- |
| Protocol version | The FQDP wire format version, `ver_major.ver_minor` (0.1). | §4.7 |
| Evaluator level | A number naming which rule-language features an evaluator understands. Level 1 is this spec. | §8.8 |
| `EVAL_LEVEL` | The highest evaluator level an SDK implements, sent in bits 8–11 of `client_capabilities`. | §4.2 |
| Effective level | For a flag: the highest of its own level and those of the segments it references. | §8.8 |
| `schema_version` | The version of a definition's binary layout. | §9.1 |
| Raft | The consensus algorithm the server cluster uses to agree on every write, in the same order. | §13 |
| Raft log index | The position of a write in Raft's ordered log. FQDP uses it as the revision. | §6.1, §8.6 |
| Leader | The cluster node that orders writes. Other nodes copy from it. | §4.1 |
| Caught up | A node that has applied every write the leader has. | §4.1 |
| Quorum | A majority of cluster nodes. Without it, a node cannot be sure its data is current. | §4.5 |
| Node (server) | One server process in the cluster. | §4.1 |
| Store | The Raft-replicated database holding flags, segments and credentials. | §13 |
| Raft snapshot | Raft's compacted copy of the whole store. Unrelated to subscription snapshots, which are always called just "snapshot". | §13 |
| Release | A product milestone, such as "the first product release". Never called a version. | §1 |

### A.8 Naming rule

Every term in this spec has exactly one meaning, and this glossary is the
authority. A new term that would reuse an existing word gets a new word
instead.

Renamed in this draft, for readers of earlier drafts and for code that still
uses the old names:

| Old name | New name | Why |
| --- | --- | --- |
| condition-tree node | clause | "Node" means only a Raft cluster member. |
| node code | opcode | Follows from "clause". |
| item version | revision | "Version" means only the protocol version or `schema_version`. |
| v1, v1.x | the first product release, a later release | Same reason. |
| context `key` | `targetingKey` | "Key" means only a flag or segment key. |
| Ack (0x30), `ack_kind` | Control, `control_kind` | It also carries `CHECK`, a request, and "ack" is a TCP term. |
| Nack (0x31) | Reject | Pairs with Control. |
| `SESSION_READY` | `SESSION_ESTABLISHED` | "Ready" means only SDK ready. |
| node ready, readiness timeout | in sync, sync timeout | Same reason. |
| caller's default | fallback value | "Default" means only a setting's default. |
| `server_status` `healthy` | `normal` | "Healthy" means only session health. |
| `change_kind` `delete` | `remove` | "Delete" means only deleting from the store. |
| reason `error` | `evaluation_error` | "Error" means only the Error message. |
| context kind, multi-kind | one subject, composite context | "Kind" alone means only a value kind. |
| store snapshot | Raft snapshot | "Snapshot" means only a subscription snapshot. |
| JSON object keys | member names | Same reason as `targetingKey`. |
| HandshakeAck (0x02) | HandshakeAccept | Same reason as Control. |
| `default_fallback` | `not_ready_fallback` | Same reason as fallback value. |
| `Err` prefix on Reject codes | `Rej` prefix | A Reject is not an Error. |
| public key (pinning) | server public key (SPKI) | Same reason as `targetingKey`. |

---

## Appendix B. Example session

This appendix is not normative. It follows one definition-mode SDK through a
whole session, from TCP connect to close, as complete frames. It reuses the
§6.1 flag, the §6.2 segment and the §10 frames, so every byte can be checked
against the rest of the spec; if they ever disagree, the spec wins.

### B.1 Setup

- **Credential.** Holds `flags:read` and `flags:definitions:read` in `prod`. The
  SDK obtained service token `tok-ABC` (valid 3,600 s) from the HTTP API.
- **Store.** Segment `beta` (§6.2) at revision 1038, and flag
  `checkout.new-flow` (§6.1) at revision 1042. Its effective revision is
  max(1042, 1038) = 1042 (§8.3).
- **Server.** `heartbeat_interval_s` 15, `credential_check_interval` 30 s,
  `sync_timeout_s` 30.
- **SDK.** Implements evaluator level 1; subscribes to `checkout.*`.

### B.2 Timeline

| # | t (s) | Dir | Frame | Session effect |
| --- | --- | --- | --- | --- |
| 1 | 0 | C→S | Handshake | CONNECTED; the token is checked |
| 2 | 0 | S→C | HandshakeAccept | AUTHENTICATED; definitions granted |
| 3 | 0 | S→C | Control(`SESSION_ESTABLISHED`) | ESTABLISHED; token TTL 3,600 s |
| 4 | 0 | C→S | Subscribe (definition, id 1) | — |
| 5 | 0 | S→C | DefinitionUpdate (snapshot) | SDK is ready |
| 6 | 0 | — | *local evaluation* | `user-123` → `treatment` |
| 7 | 0 | C→S | Query (id 1) | — |
| 8 | 0 | S→C | QueryResponse | `user-456` → `control` |
| 9 | 15 | both | Heartbeat | repeats whenever a peer has been silent 15 s |
| 10 | 30 | C→S | Control(`CHECK`, ref 1) | — |
| 11 | 30 | S→C | Control(`CHECK_OK`, ref 1) | token TTL 3,570 s; repeats every 30 s with ref 2, 3, … |
| 12 | 95 | S→C | DefinitionUpdate (delta) | an admin turned the kill switch off |
| 13 | 95 | — | *local evaluation* | `user-123` → `control`, `flag_disabled` |
| 14 | 2880 | C→S | Reauthenticate | re-authentication window opens (§4.3) |
| 15 | 2880 | S→C | Control(`REAUTH_OK`) | token TTL 3,600 s |
| 16 | 2900 | C→S | Unsubscribe (id 1) | — |
| 17 | 2900 | — | TCP close | CLOSED; FQDP has no goodbye message |

### B.3 Frames

**1. Handshake.** As in §10, plus `client_id`.

```text
00 00 00 26                 length 38
01                          Handshake
00 01                       version 0.1
01 01                       capabilities: EVAL_LEVEL=1, CAP_DEFINITIONS
00 07 74 6f 6b 2d 41 42 43  token "tok-ABC"
00 16 63 68 65 63 6b 6f 75 74 2d 73 65 72 76 69 63 65 40 31 2e 34 2e 32
                            client_id "checkout-service@1.4.2"
```

**2. HandshakeAccept** and **3. Control(SESSION_ESTABLISHED).** Identical to
§10.

```text
00 00 00 1a  02  00 01  00 01 02 03 04 05 06 07 08 09 0a 0b 0c 0d 0e 0f  01  00 0f  00 1e  00 1e
00 00 00 0b  30  00  00 00 00 00  00  00 00 0e 10
```

**4. Subscribe.** Identical to §10: definition mode, `subscription_id` 1,
criterion `checkout.*`.

```text
00 00 00 18  20  00 00 00 01  01  00 00 00 00  00 01  00 0a 63 68 65 63 6b 6f 75 74 2e 2a
```

**5. DefinitionUpdate, snapshot.** The §10 frame, written out. The segment
comes before the flag that references it (§8.5). This single frame carries
`SNAPSHOT|SNAPSHOT_END`, so the SDK is now ready.

```text
00 00 01 0b  23  00 00 00 01  00 00 00 00 00 00 00 01  03  00 02
                                     length 267; sub 1; seq 1; SNAPSHOT|SNAPSHOT_END; 2 changes
00 01  00 04 62 65 74 61             upsert, segment "beta"
00 00 00 00 00 00 04 0e  00 00 00 40 revision 1038; 64 bytes:
01 01                                  schema_version 1, eval_level 1
00 02  00 06 75 73 65 72 2d 31  00 06 75 73 65 72 2d 32   included ["user-1","user-2"]
00 01  00 06 75 73 65 72 2d 39         excluded ["user-9"]
01  05 73 74 61 66 66  80 01           1 rule "staff": group all, 1 child
03 00 05 65 6d 61 69 6c  02 00 01  00 0c 40 65 78 61 6d 70 6c 65 2e 63 6f 6d
                                       endsWith "email", strings ["@example.com"]
00 00  00 11 63 68 65 63 6b 6f 75 74 2e 6e 65 77 2d 66 6c 6f 77
                                     upsert, flag "checkout.new-flow"
00 00 00 00 00 00 04 12  00 00 00 86 revision 1042; 134 bytes:
<the 134 bytes of §9.6>
```

**6. Local evaluation.** The SDK evaluates `checkout.new-flow` for
`{targetingKey:"user-123", attributes:{country:"NL"}}` with no network traffic
(§7.1):

1. The flag is present, well-formed and enabled.
2. Rule `beta-testers`: `user-123` is in neither `included` nor `excluded`, and
   rule `staff` is false because `email` is missing. Not a member, so no match.
3. Rule `benelux-50`: `country` `NL` is in the list, so the rule matches. Its
   rollout hashes `checkout.new-flow.user-123` to bucket 74356 (§6.5), which is
   `treatment`.

Result: value `"v2"`, variation `treatment`, reason `rule_match`, ruleId
`benelux-50`.

**7. Query.** The application also asks the server directly, for `user-456` in
`BE`, and includes a key that does not exist.

```text
00 00 00 48  10  00 00 00 01         length 72; Query; query_id 1
00 00 00 18                          context_len 24
00 08 75 73 65 72 2d 34 35 36        targeting key "user-456"
01  07 63 6f 75 6e 74 72 79  02  00 02 42 45   1 attribute: country = "BE"
00 02                                2 flag keys
00 11 63 68 65 63 6b 6f 75 74 2e 6e 65 77 2d 66 6c 6f 77   "checkout.new-flow"
00 10 63 68 65 63 6b 6f 75 74 2e 6d 69 73 73 69 6e 67      "checkout.missing"
```

**8. QueryResponse.** `user-456` matches `benelux-50` and lands in bucket 49880
(§6.5), which is `control`. The missing key gets `flag_not_found`.

```text
00 00 00 5e  11  00 00 00 01  00  00 02   length 94; query_id 1; MORE clear; 2 items
00 11 63 68 65 63 6b 6f 75 74 2e 6e 65 77 2d 66 6c 6f 77   "checkout.new-flow"
00 00 00 00 00 00 04 12              effective revision 1042
01  00  02  00 00 00 02 76 31        rule_match; no error; string "v1"
07 63 6f 6e 74 72 6f 6c              variation "control"
0a 62 65 6e 65 6c 75 78 2d 35 30     rule "benelux-50"
00 10 63 68 65 63 6b 6f 75 74 2e 6d 69 73 73 69 6e 67      "checkout.missing"
00 00 00 00 00 00 00 00              revision 0
03  00  ff  00  00                   flag_not_found; no error; no value; no variation; no rule
```

**9. Heartbeat.** Each peer sends one after 15 s without sending anything.

```text
00 00 00 01  40
```

**10–11. Control(CHECK) and Control(CHECK_OK).** `ref_id` 1; status normal;
3,570 s left on the token.

```text
00 00 00 0b  30  01  00 00 00 01  00  00 00 00 00
00 00 00 0b  30  02  00 00 00 01  00  00 00 0d f2
```

**12. DefinitionUpdate, delta.** An admin disables the flag; the write is Raft
log index 1051. The segment did not change, so only the flag is sent. Its
definition is the §9.6 bytes with `flag_bits` (byte 4) changed from `01` to
`00`.

```text
00 00 00 b7  23  00 00 00 01  00 00 00 00 00 00 00 02  00  00 01
                                     length 183; sub 1; seq 2; no snapshot bits; 1 change
00 00  00 11 63 68 65 63 6b 6f 75 74 2e 6e 65 77 2d 66 6c 6f 77
                                     upsert, flag "checkout.new-flow"
00 00 00 00 00 00 04 1b  00 00 00 86 revision 1051; 134 bytes:
01 01 02 00                            schema_version 1; eval_level 1; string; disabled
<the remaining 130 bytes of §9.6, from salt_len on>
```

The SDK accepts it: the sequence is the next one (2), and 1051 is higher than
the 1042 it holds (§8.6).

**13. Local evaluation.** The same `user-123` context now stops at step 3 of
§7.1: value `"v1"`, variation `control`, reason `flag_disabled`.

**14–15. Reauthenticate and Control(REAUTH_OK).** At t = 2880, 720 s (20%) of
the token's lifetime remains. That is earlier than 60 s before expiry, so the
SDK rotates now (§4.3). Identical to §10.

```text
00 00 00 0a  03  00 07 74 6f 6b 2d 58 59 5a
00 00 00 0b  30  03  00 00 00 00  00  00 00 0e 10
```

**16. Unsubscribe.** Identical to §10. No reply follows; the SDK then closes the
TCP connection.

```text
00 00 00 05  21  00 00 00 01
```
