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

---

## 1. Overview

FQDP is a lightweight, language-agnostic protocol for high-throughput feature
flag queries, subscriptions and definition delivery.

- **Transport.** It runs at OSI Layer 4 over raw TCP sockets, not over HTTP or WebSocket.
- **Encoding.** The wire format is binary throughout. No JSON is sent on the wire.
- **Authentication.** Clients authenticate with a token-based handshake.

**Goals**

- Low-latency queries and push updates for large numbers of clients.
- Simple subscription semantics: an initial snapshot, then deltas.
- Two consumption modes (§5):
  - **Value mode.** The server evaluates flags against a context the client sends.
  - **Definition mode.** The server ships full flag definitions, and the client evaluates them locally, with no round-trip per call.
- Language independence. A compact, fully specified binary encoding and shared conformance vectors give independent SDKs byte- and result-identical behavior.
- Forward compatibility. The rule language can grow without breaking SDKs that are already deployed (§8.8).
- Sessions you can rely on: an explicit established state, token rotation, two-sided credential checks, and health reporting (§4).
- Secure connections through TLS and token authentication.

**Terminology**

- **"v1"** means the first iteration of the product feature set, matching the "Must-have (v1)" priority in the product backlog. **"v1.x"** means a later iteration.
- Neither term refers to FQDP protocol version 1.0.
- Three version numbers evolve independently:

| Number | Value in this document | Defined in |
| --- | --- | --- |
| Protocol version | 0.1 | §4.7 |
| Evaluator level | 1 | §8.8 |
| Definition `schema_version` | 1 | §9.1 |

- Byte layouts are written as `[type name]`.
- `C→S` means client to server. `S→C` means server to client.

---

## 2. Quick reference

### 2.1 Constants and defaults

| Name | Value | Where |
| --- | --- | --- |
| Protocol version | `0.1` (`ver_major = 0`, `ver_minor = 1`) | §4.7 |
| Default port | 9001 (deployment-configurable) | §3 |
| `MaximumFrameSize` | 10 MiB (payload) | §3 |
| Handshake timeout | 10 s recommended (configurable) | §4.1 |
| Readiness timeout | deployment-configured | §4.1 |
| `heartbeat_interval_s` | announced by the server in the HandshakeAck | §4.6 |
| Dead-peer timeout | 3 × `heartbeat_interval_s` | §4.6 |
| `credential_check_interval` | 30 s recommended; client-chosen in the Handshake, capped by the server, announced in the HandshakeAck | §4.2, §4.4 |
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
| Segment `included` + `excluded` keys | 10,000 total | `u16` counts |
| Encoded definition (flag or segment) | 256 KiB | `u32` length |
| Integer variation values | ±(2^53−1) | `i64` |
| Context `key` | 1–256 bytes UTF-8 | `u16` length |
| Context attributes | 0–100, names `^[A-Za-z0-9_.-]{1,64}$` | `u8` count / length |
| Encoded context | 64 KiB | `u32` length |
| Keys per Query | 1–1,000 | `u16` count |
| Criteria per Subscribe or DefinitionQuery | 1–1,000 | `u16` count |

### 2.3 Message types

| Type | Name | Direction | Purpose | Section |
| --- | --- | --- | --- | --- |
| 0x01 | Handshake | C→S | Open a session: version, capabilities, token | §4.2 |
| 0x02 | HandshakeAck | S→C | Token accepted: session id, granted capabilities, heartbeat interval | §4.2 |
| 0x03 | Reauthenticate | C→S | Replace the session's token | §4.3 |
| 0x10 | Query | C→S | Evaluate flags against a context (value mode) | §8.1 |
| 0x11 | QueryResponse | S→C | Evaluation results | §8.1 |
| 0x12 | DefinitionQuery | C→S | One-shot fetch of definitions | §8.5 |
| 0x13 | DefinitionResponse | S→C | Definitions | §8.5 |
| 0x20 | Subscribe | C→S | Subscribe in value or definition mode | §8.2 |
| 0x21 | Unsubscribe | C→S | End a subscription | §8.2 |
| 0x22 | Update | S→C | Value-mode snapshot and deltas | §8.3 |
| 0x23 | DefinitionUpdate | S→C | Definition-mode snapshot and deltas | §8.5 |
| 0x30 | Ack | both | Session ready, credential check, re-auth result, status change | §4.4 |
| 0x31 | Nack | S→C | Reject one request | §8.9 |
| 0x40 | Heartbeat | both | Keepalive | §4.6 |
| 0x50 | Error | S→C | Connection-level failure, then close | §8.9 |

All other type values are unassigned and get `Error` 100. A server that
receives an S→C type (HandshakeAck, QueryResponse, DefinitionResponse, Update,
DefinitionUpdate, Nack, Error) also sends `Error` 100.

### 2.4 Enumerations

| Field | Values |
| --- | --- |
| Handshake `client_capabilities` (u16) | bit 0 `CAP_DEFINITIONS`; bits 8–11 `EVAL_LEVEL` (0 means 1); others 0 |
| HandshakeAck `server_flags` (u8) | Granted capability bits 0–7 |
| `ack_kind` (u8) | 0 `SESSION_READY` (S→C), 1 `CHECK` (C→S), 2 `CHECK_OK` (S→C), 3 `REAUTH_OK` (S→C), 4 `STATUS` (S→C) |
| `server_status` (u8) | 0 healthy, 1 degraded |
| Subscribe `mode` (u8) | 0 value, 1 definition |
| `value_type` (u8) | 0 bool, 1 int, 2 string, 3 json, 0xFF none |
| `reason` (u8) | 0 `flag_disabled`, 1 `rule_match`, 2 `fallthrough`, 3 `flag_not_found`, 4 `error` |
| `error_kind` (u8) | 0 none, 1 `malformed_flag` |
| `change_kind` (u8) | 0 upsert, 1 delete |
| `item_kind` (u8) | 0 flag, 1 segment, 2 server-evaluated flag |
| `update_flags` (u8) | bit 0 `SNAPSHOT`, bit 1 `SNAPSHOT_END` |
| `response_flags` (u8) | bit 0 `MORE` (QueryResponse, DefinitionResponse) |
| Nack `ref_kind` (u8) | 0 `query_id`, 1 `subscription_id` |
| Flag `type` (u8) | 0 boolean, 1 integer, 2 string, 3 json |
| `flag_bits` (u8) | bit 0 enabled |
| Node codes (u8) | 0x00 `in`, 0x01 `contains`, 0x02 `startsWith`, 0x03 `endsWith`, 0x04 `lt`, 0x05 `lte`, 0x06 `gt`, 0x07 `gte`, 0x08 `inSegment`, 0x09–0x7F reserved leaves, 0x80 `all`, 0x81 `any`, 0x82 reserved (`not`), 0x83–0xFF reserved |
| `cond_bits` (u8) | bit 0 negate |
| Condition `value_kind` (u8) | 0 bool, 1 number (float64), 2 string; context attribute kinds set bit 7 for a list |
| `serve_kind` (u8) | 0 variation, 1 rollout |

### 2.5 Error codes

An **Error** ends the connection: the server closes it after sending. A **Nack**
rejects one request, and the connection stays open. Codes 125–199 are reserved.

| Code | Name | Sent as | When |
| --- | --- | --- | --- |
| 100 | `ErrUnsupportedMessageType` | Error | Unassigned or unknown message type. |
| 101 | `ErrInvalidHandshakePayload` | Error | The Handshake body does not decode. |
| 102 | `ErrHandshakeTokenTruncated` | Error | `token_len` runs past the payload. |
| 103 | `ErrHandshakeNotImplemented` | Error | Stub server only. Retired once the Handshake is implemented. |
| 104 | `ErrUnsupportedVersion` | Error | The version is not implemented (§4.7). The message names the supported versions. |
| 105 | `ErrAuthenticationFailed` | Error | Invalid token; not an application-credential service token; unknown or inactive credential; a credential without `flags:read`, at Handshake or Reauthenticate. |
| 106 | `ErrCredentialRevoked` | Error | The session's credential was revoked or deactivated, or lost `flags:read`. Either side of the credential check (§4.4) can detect this. |
| 107 | `ErrRateLimited` | Error | A connection or request rate limit was exceeded (§12). |
| 108 | `ErrFrameTooLarge` | Error | `length` exceeds `MaximumFrameSize`. |
| 109 | `ErrSessionNotEstablished` | Error | A request arrived before `Ack(SESSION_READY)` (§4.1). |
| 110 | `ErrDefinitionsNotPermitted` | Nack | A definition-mode request without the granted capability. Also sent for each definition subscription that ends because the credential lost `flags:definitions:read` (§4.3, §4.4). |
| 111 | `ErrInvalidContext` | Nack | The context does not decode, has no `key`, has a bad attribute name, or exceeds limits. |
| 112 | `ErrInvalidCriteria` | Nack | The criteria list is empty, or a criterion is not a valid key, prefix or `*`. |
| 113 | `ErrUnknownMode` | Nack | The Subscribe `mode` is neither 0 nor 1. |
| 114 | `ErrDuplicateSubscription` | Nack | The `subscription_id` is not higher than the last one used in the session (§8.2). |
| 115 | `ErrMalformedMessage` | Error | A known message type whose body does not decode: truncated, trailing bytes, an unknown enum value (including `ack_kind`), an S→C `ack_kind` sent by a client, or a reserved bit or must-be-0 field that is set (§9.1). |
| 116 | `ErrTokenExpired` | Error | The token expired before a successful Reauthenticate. |
| 117 | `ErrServerUnavailable` | Error | The node cannot establish the session within its readiness timeout. The client should try another node. |
| 118 | `ErrIdentityMismatch` | Error | A Reauthenticate token belongs to a different credential or environment than the session. |
| 119 | `ErrHeartbeatTimeout` | Error | No frame from the client for 3 × `heartbeat_interval_s`. Sent best-effort. |
| 120 | `ErrHandshakeTimeout` | Error | No Handshake arrived within the handshake timeout. |
| 121 | `ErrAlreadyAuthenticated` | Error | A second Handshake on the connection, in any state. Reauthenticate is the way to replace a token. |
| 122 | `ErrInvalidRequest` | Nack | An empty key list, an invalid flag key, a `query_id` not higher than the last one used (§8.1), more keys or criteria than §2.2 allows, or another per-request limit exceeded. |
| 123 | `ErrSubscriptionLimit` | Nack | The session already has the maximum number of active subscriptions. |
| 124 | `ErrServerShuttingDown` | Error | The node is draining or shutting down. The client should reconnect, ideally to another node. |

---

## 3. Transport and framing

- **Transport.** Raw TCP. The default port is 9001.
- **TLS.** Direct TLS, where the TLS handshake runs as soon as the TCP connection opens, is strongly recommended in production. Self-signed certificates are supported, and SDKs warn on plaintext connections (§12). A STARTTLS-style upgrade is not defined.
- **Sessions.** The server keeps one lightweight session per connection. Query ids, subscription ids and sequence numbers are scoped to that session.
- **Frames.** Every frame is `[u32 length][payload]`. The payload is `[u8 MessageType][body…]`, and `length` counts the payload only.
  - A zero-length frame is ignored.
  - A frame whose `length` exceeds `MaximumFrameSize` gets `Error` 108.
- **Integers and strings.** All integers are big-endian. A string is written `[uN len][UTF-8 bytes]`, where N is given at each field.
- **Partial I/O.** Implementations must handle partial reads and writes on the TCP stream.

---

## 4. Session lifecycle

### 4.1 States

```
CONNECTED ──Handshake──▶ AUTHENTICATED ──Ack(SESSION_READY)──▶ ESTABLISHED ──▶ CLOSED
   │  (HandshakeAck sent)     │                                   │ ▲
   │                          └──Error 117 (not ready in time)    │ │ Reauthenticate → Ack(REAUTH_OK)
   └──Error 120 (no Handshake in time)                            └─┘ Ack(CHECK)      → Ack(CHECK_OK)
```

- **Any state.** A Heartbeat (§4.6) is accepted in every state. These checks come before the state checks below:
  - an unassigned or S→C message type gets `Error` 100 (§2.3);
  - any Handshake after the first gets `Error` 121.
- **CONNECTED.** TCP (and TLS) is up. The server accepts only a Handshake (and Heartbeats): any other known message gets `Error` 109. If no Handshake arrives within the handshake timeout, the server sends `Error` 120.
- **AUTHENTICATED.** The server has validated the token and sent the HandshakeAck. It now finishes setting up the session: registering it, and making sure the node can serve consistent data (for example, that it has caught up with the cluster leader). A request in this state gets `Error` 109; Heartbeats keep flowing as usual.
- **ESTABLISHED.** The server has sent `Ack(SESSION_READY)`. The client may now assume a healthy connection and send requests.
  - If the node cannot become ready within its readiness timeout, it sends `Error` 117 instead, so the client can try another node.

### 4.2 Handshake and HandshakeAck

**Handshake (0x01)**

```
[u8 0x01]
[u8 ver_major]                  // 0
[u8 ver_minor]                  // 1
[u16 client_capabilities]
[u16 credential_check_interval_s]  // 0 = server default
[u16 token_len][token]          // UTF-8
[u16 client_id_len][client_id]  // optional, 0 if absent
```

- **`credential_check_interval_s`.** A client-side setting (SDK default 30 s) that the client announces here. The server uses `min(client value, server maximum)` as this session's `credential_check_interval` (§4.4), so a client can shorten but never lengthen the time to detect a revocation (§12).
- **`client_id`.** A free-form label for the client (for example `checkout-service@1.4.2`). It has no protocol meaning; the server records it in session logs and the session registry (§13) to help operators identify connections.
- **Token kind.** The token must be an application-credential service token (§12), which is scoped to exactly one environment. Any other token, such as a human login token, gets `Error` 105.
- **Scope.** The credential must hold `flags:read`. A credential without it, for example one with only write scopes, gets `Error` 105.

| Capability bit | Name | Meaning |
| --- | --- | --- |
| 0 | `CAP_DEFINITIONS` | The client wants definition mode. The server grants it only if the credential holds `flags:definitions:read`. |
| 1–7 | reserved | Must be 0. |
| 8–11 | `EVAL_LEVEL` | A 4-bit unsigned value: the highest evaluator level the client implements (§8.8). `0` means 1. Ignored when bit 0 is clear. |
| 12–15 | reserved | Must be 0. |

**HandshakeAck (0x02)**

```
[u8 0x02]
[u8 ver_major]                    // accepted version
[u8 ver_minor]
[16 bytes session_id]             // opaque, e.g. a UUID
[u8 server_flags]                 // capability bits 0–7 that were granted
[u16 heartbeat_interval_s]         // ≥ 1
[u16 credential_check_interval_s]  // the agreed value
```

- **`heartbeat_interval_s`.** Must be at least 1. A client that receives 0 treats the HandshakeAck as malformed and closes the connection.
- **`credential_check_interval_s`.** The session's `credential_check_interval`: `min(client value, server maximum)`, or the server default when the client sent 0. The client uses it to pace its `CHECK`s (§4.4).
- **Failure.** A version the server doesn't implement gets `Error` 104. A token that fails validation gets `Error` 105.
- **Success.** The server sends the HandshakeAck, then `Ack(SESSION_READY)` once the session is established.
- **Definitions refused.** If the client asked for definition mode but `server_flags` bit 0 is clear, the request was refused. The session then works in value mode only.

### 4.3 Reauthenticate (0x03)

Reauthenticate replaces the session's token without reconnecting. Subscriptions,
snapshots and sequence numbers survive token rotation.

```
[u8 0x03]
[u16 token_len][token]
```

- **Identity.** The new token must belong to the same credential and environment as the session; otherwise the server sends `Error` 118. An invalid token, or a credential without `flags:read`, gets `Error` 105.
- **Success.** The server replies `Ack(REAUTH_OK)` with the new `token_ttl_s`.
- **Scope changes** take effect immediately. Scopes belong to the credential, not the token: the server reads them from the store.
  - If the credential no longer holds `flags:definitions:read`, every definition subscription ends with a `Nack` 110 that references it. Later definition requests also get `Nack` 110. The server-side credential check (§4.4) applies the same rule without a Reauthenticate.
  - Gaining a scope never upgrades the session: only a Handshake can grant `CAP_DEFINITIONS`.
- **Timing.** The client re-authenticates before `token_ttl_s` runs out. It should start at the earlier of two moments: when less than 20% of the token's lifetime remains, or 60 seconds before expiry. Both are measured from the first `token_ttl_s` received for that token (in `SESSION_READY` or `REAUTH_OK`). The fresh token is obtained the same way as the first one, for example through the HTTP API's client-credentials endpoint.
- **Expiry.** If the token expires first, the server sends `Error` 116 and closes the connection.

### 4.4 Ack (0x30) and credential checks

Ack carries four things: session establishment, credential checks, the result of
a re-authentication, and server status changes. It flows in both directions and
has one fixed layout.

```
[u8 0x30]
[u8 ack_kind]          // §2.4
[u32 ref_id]           // CHECK: client-chosen; CHECK_OK echoes it; otherwise 0
[u8 server_status]     // S→C: 0 healthy, 1 degraded; C→S: 0
[u32 token_ttl_s]      // S→C: seconds until the session's token expires,
                       //      0xFFFFFFFF = no expiry; C→S: 0
```

| `ack_kind` | Direction | Meaning |
| --- | --- | --- |
| 0 `SESSION_READY` | S→C | Sent exactly once, after the HandshakeAck. The session is established. |
| 1 `CHECK` | C→S | Ask the server to re-validate the credential and report its health. |
| 2 `CHECK_OK` | S→C | Reply to `CHECK`: the credential is still valid. If it was revoked, the server sends `Error` 106 instead. |
| 3 `REAUTH_OK` | S→C | Reply to a successful Reauthenticate. |
| 4 `STATUS` | S→C | Sent unsolicited when the server's `server_status` changes. |

**Credential validity is checked from both sides.**

- **Interval.** `credential_check_interval` is per session: the value agreed in the Handshake and announced in the HandshakeAck (§4.2).
- **Server side.** The server re-validates a session's credential, its active state and its scopes, whenever the store reports a change to it, and in any case at least every `credential_check_interval`. This also covers sessions that only receive pushes.
  - A revoked or deactivated credential, or one that lost `flags:read`, gets `Error` 106, and the connection is closed.
  - A credential that lost `flags:definitions:read` ends the session's definition subscriptions with `Nack` 110, as in §4.3.
- **Client side.**
  - The client may send `Ack(CHECK)` at any time. The server re-validates immediately and replies `Ack(CHECK_OK)` or `Error` 106.
  - While no other request is outstanding, SDKs should send a `CHECK` at least every `credential_check_interval`.
  - If no `CHECK_OK` arrives within 3 × `heartbeat_interval_s`, counted from the last byte received, the client closes the connection and reconnects. Counting from the last byte keeps a large snapshot frame on a slow link from triggering it.

### 4.5 Server status and session health

- **Server status.** `server_status = 1` (*degraded*) means the node is still serving but cannot guarantee fresh data, for example because it has lost cluster quorum. SDKs surface this to the application. They may reconnect elsewhere, but are not required to.
- **Session health.** A session is **healthy** when all of these hold:
  - it is ESTABLISHED;
  - its token has not expired;
  - its credential passed the last check;
  - the client has sent a frame within 3 × `heartbeat_interval_s`.

  The server tracks healthy, expiring and total sessions as part of its own health reporting (§13).

### 4.6 Heartbeat (0x40)

```
[u8 0x40]
```

- **When to send.** Either peer sends a Heartbeat once it has sent nothing for `heartbeat_interval_s`. No reply is needed. Heartbeats are valid in every session state (§4.1), including while the client waits for `SESSION_READY`.
- **Dead peers.** A peer that has received no frame for 3 × `heartbeat_interval_s` considers the connection dead and closes it. The server first tries, best-effort, to send `Error` 119.
- **Scope.** Heartbeats only prove that the connection is alive. Credential validity and server health are checked with Ack (§4.4).

### 4.7 Protocol versioning

This specification is protocol **0.1**. While the major version is `0`:

- **Breaking changes.** Any minor bump may change the wire format.
- **Exact versions only.** The server accepts a Handshake only for a `0.minor` it implements exactly. It may implement more than one during a transition. Any other version gets `Error` 104, and the connection is closed.
- **No minor-version negotiation.** The rule language grows through evaluator levels (§8.8), which need no protocol bump.

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
  "version": 1042,
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
          "bucketBy": "key",
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
| `version` | Raft log index of the last write. Read-only. |
| `salt` | The bucketing salt. Defaults to `key`. See the notes below the table. |
| `variations` | Unique ids. Each `value` must match `type`. A `boolean` flag has exactly `[{id:"true",value:true},{id:"false",value:false}]`. |
| `offVariation` | Required. The id of an existing variation. |
| `rules` | Evaluated in array order. Each `id` is unique within the flag. It is reported as `ruleId` and referenced in audit entries. |
| `rules[].match` + `rules[].conditions` | The rule's root condition group (§6.4). `match` defaults to `all`. |
| `rules[].serve`, `fallthrough` | Exactly one of `{ "variation": id }` or `{ "rollout": … }` (§6.5). |

Notes on `salt`:

- Because it defaults to `key`, a context lands in the same bucket in every environment, and promoting a flag keeps its rollout membership.
- Changing it reshuffles every context. Only an explicit, audited "reset rollout" action should do that.

### 6.2 Segment

```json
{
  "schemaVersion": 1,
  "key": "beta",
  "environmentId": "prod",
  "name": "Beta testers",
  "version": 1038,
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
  1. If the context key is in `included`, the context is a member.
  2. Otherwise, if the key is in `excluded`, it is not a member.
  3. Otherwise, if any segment rule matches, it is a member.
  4. Otherwise, it is not a member.
- **Segment rules** are root groups (§6.4), with no `serve`. They must not use `inSegment`, so segments do not recurse.
- **Deletion.** A segment cannot be deleted while any flag in its environment references it.

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

- **`attribute`.** Either `key` (the context key) or a top-level attribute name. There are no nested paths.
- **`values`.** All values in one condition are of one kind: all strings, all numbers or all booleans.
- **List attributes.** If the context attribute is a list, the condition matches when any element matches. An empty list counts as a missing attribute, since it has no kind.
- **Missing or mismatched attributes.** If the attribute is missing, or its kind does not fit the operator, the condition is **false**, and `negate` is **not** applied. A missing attribute never widens a rule's audience.
- **`negate`.** When the attribute is present and kind-compatible, `negate: true` inverts the result.
- **`inSegment`.** It has no attribute and always counts as present, so `negate: true` inverts it: `{inSegment beta, negate}` matches every context that is not a member of `beta`.
- **Numbers.** NaN and ±Inf are never valid values, in conditions or in contexts (§9.1). `-0` and `0` compare equal.
- **Strings.** String comparisons are case-sensitive and byte-exact on UTF-8, with no Unicode normalization.

Regex, semver and date operators are not part of evaluator level 1. Regex in
particular is excluded because its semantics differ across languages.

### 6.4 Condition groups

A rule's predicate is a **condition tree**:

- A **leaf** is one condition (§6.3).
- A **group** is `{ "match": "all" | "any", "conditions": [ node, … ] }`.

The rule object itself is the root group. An entry in `conditions` that has a
`match` field is a group; one that has an `op` field is a leaf. An entry with
both, or with neither, is invalid.

- **Semantics.**
  - `all` is true when every child is true; `any` is true when at least one child is true.
  - Groups are never empty.
  - Evaluation has no side effects, so evaluators may short-circuit and may evaluate children in any order.
- **Evaluator level 1.** Depth is 1: the root group's children are all leaves. The HTTP API rejects a nested group on write with `400`.
- **Future nesting.** The shape below is already defined, but rejected at level 1. A later level will lift the depth limit (with a total-node limit) without changing the JSON shape, the binary layout or the semantics. Every level-1 definition stays valid at later levels.

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

- **`not` group.** A single-child `not` group (node code `0x82`) is reserved. Before it can be defined, its interaction with the missing-attribute rule (§6.3) has to be settled. Until then, leaf-level `negate` is the way to express negation.

### 6.5 Percentage rollout and bucketing

```json
"rollout": {
  "bucketBy": "key",
  "weights": [ { "variation": "control", "weight": 50000 }, { "variation": "treatment", "weight": 50000 } ]
}
```

- **Weights.** Non-negative integers that sum to exactly **100000**. Each variation appears at most once.
- **`bucketBy`.** Defaults to `key`. It must name a string attribute or an integer-valued number attribute within ±(2^53−1). A number is converted to its plain decimal form, such as `42`, with no exponent and no `.0`. `-0` is written as `0`. A larger number is unusable: languages format it differently.

**Bucketing (normative)**

```
input  = UTF-8( salt + "." + bucketValue )
digest = SHA-256(input)
n      = big-endian uint64 of digest[0:8]
bucket = n mod 100000                      // 0 ≤ bucket < 100000
```

- **Choosing the variation.** The context gets the first variation, in array order, whose cumulative weight is greater than `bucket`.
- **Missing `bucketBy` attribute.** If it is missing or unusable, `bucket = 0`. This can only happen with a custom `bucketBy`, since every context has a non-empty `key` (§6.6).

**Vectors.** These use `salt = "checkout.new-flow"` and the 50/50 rollout above,
so `control` covers buckets [0, 50000) and `treatment` covers [50000, 100000):

| bucketValue | digest[0:8] | bucket | variation |
| --- | --- | --- | --- |
| `user-123` | `6b4499fee63c64f4` | 74356 | treatment |
| `user-456` | `60fe5b62acc73e58` | 49880 | control |
| `user-789` | `84596b483f834821` | 21569 | control |

### 6.6 Context

```json
{ "key": "user-123", "attributes": { "country": "NL", "plan": "pro", "beta": true, "groups": ["a", "b"] } }
```

- **`key`.** Required, 1–256 bytes. There is no empty or keyless context.
- **Attribute names.** Unique within the context. The name `key` is reserved and cannot be used for an attribute.
- **Attribute values.** A string, a number, a boolean, or a flat list of a single one of those kinds.
- **Kind.** A context has a single kind. Multi-kind contexts are not defined.
- **Limits.** See §2.2.

---

## 7. Evaluation

### 7.1 Algorithm (normative)

Given a flag definition `F` (or none) and a context `C`, evaluation takes the
first step that applies:

0. **`C` is invalid** (SDK-local only). Return the caller's default, with reason `error` and `errorKind: invalid_context`. A server never reaches this step: it rejects an invalid context with `Nack` 111.
1. **`F` is missing.** Return the caller's default, with reason `flag_not_found`.
2. **`F` is malformed.** Return the caller's default, with reason `error` and `errorKind: malformed_flag`. Malformed means failed decoding, an unknown `schema_version`, a dangling variation or segment reference, or bad weights.
3. **`F.enabled == false`.** Return `offVariation`, with reason `flag_disabled`.
4. **A rule matches.** Try the rules in order. The first rule whose root group (§6.4) is true decides the result: its serve, with reason `rule_match` and `ruleId` set.
5. **Fallthrough.** Return `fallthrough`, with reason `fallthrough`.

### 7.2 Reasons and error kinds

| Reason | u8 | Meaning |
| --- | --- | --- |
| `flag_disabled` | 0 | Kill switch off; served `offVariation`. |
| `rule_match` | 1 | Rule `ruleId` matched. |
| `fallthrough` | 2 | No rule matched; served `fallthrough`. |
| `flag_not_found` | 3 | No such flag in this environment. |
| `error` | 4 | See `errorKind`. |
| `default_fallback` | — | SDK-local only: no data yet (`client_not_ready`), or disconnected with no last-known-good copy. |

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

**Query (0x10)**

```
[u8 0x10]
[u32 query_id]                // client-chosen; strictly increasing in the session
[u32 context_len]             // > 0
[context_len bytes context]   // §9.5
[u16 key_count]               // 1–1,000 (§2.2)
repeated: [u16 key_len][key]
```

**QueryResponse (0x11)** contains one item per requested key, in request order.

```
[u8 0x11]
[u32 query_id]
[u8 response_flags]           // bit0 MORE: further frames for this query_id follow
[u16 item_count]
repeated:
  [u16 key_len][key]
  [u8 reason]
  [u8 error_kind]
  [u8 value_type]             // 0xFF = none
  [value]                     // §9.1 value encoding for value_type; absent with 0xFF
  [u8 variation_id_len][variation_id]
  [u8 rule_id_len][rule_id]   // 0 unless reason = rule_match
  [u64 version]               // §8.3 effective version; 0 when reason = flag_not_found
```

- **Context.** A Query always carries a context with a key (§6.6). `context_len = 0` gets `Nack` 111.
- **Concurrent queries.** Several queries may be in flight at once. Responses are matched by `query_id`.
- **Query ids.** Each `query_id` is higher than the previous one in the session, so the server only remembers the last. Query and DefinitionQuery share one id space, since a Nack refers to either with `ref_kind 0`. An id that is not higher gets `Nack` 122. A client that runs out of ids reconnects.
- **Size.** A response that would exceed `MaximumFrameSize` is split across frames with `MORE`, in request order. The last frame has `MORE` clear.
- **Rejection.** An invalid Query gets a Nack (111 or 122), not a QueryResponse.

### 8.2 Subscribe and Unsubscribe

**Subscribe (0x20)** uses one layout for both modes.

```
[u8 0x20]
[u32 subscription_id]        // client-chosen; strictly increasing in the session
[u8 mode]                    // 0 value, 1 definition
[u32 context_len][context]   // value mode: > 0; definition mode: must be 0
[u16 criteria_count]         // 1–1,000 (§2.2)
repeated: [u16 len][criterion]   // §8.4
```

- **Confirmation.** There is no separate acknowledgement. The snapshot (§8.6) confirms the subscription. It is always sent, and always ends with a `SNAPSHOT_END` frame, even when nothing matches.
- **Rejection.** The server answers with a Nack carrying one of these codes:
  - 110: definition mode not granted.
  - 111: invalid context.
  - 112: invalid criteria.
  - 113: unknown mode.
  - 114: `subscription_id` not higher than the last one used in this session.
  - 122: more criteria than §2.2 allows.
  - 123: subscription limit reached.

**Unsubscribe (0x21)**

```
[u8 0x21]
[u32 subscription_id]
```

There is no reply, and an unknown id is ignored. Frames already in flight may
still arrive; the client discards them. Because ids only go up, a later
Subscribe can never reuse the id: one that is not higher gets `Nack` 114.

### 8.3 Update (value mode)

```
[u8 0x22]
[u32 subscription_id]
[u64 sequence]               // per subscription: 1, then +1 per frame
[u8 update_flags]
[u16 change_count]
repeated:
  [u8 change_kind]
  [u16 key_len][key]
  [u64 version]              // effective version, see below
  if upsert:
    [u8 reason]
    [u8 error_kind]
    [u8 value_type]
    [value]                  // §9.1 value encoding for value_type; absent with 0xFF
    [u8 variation_id_len][variation_id]
    [u8 rule_id_len][rule_id]
```

- **Re-evaluation.** The server re-evaluates the subscription's context whenever a matched flag, or a segment it references, changes.
- **Effective version.** The `version` of every flag item is the highest of the flag's version and the versions of the segments it references. This holds in every message: Update, QueryResponse, DefinitionResponse and DefinitionUpdate, for `item_kind` 0 and 2. A segment item carries the segment's own version. A segment edit therefore raises the version of every flag that depends on it, so the client's version check (§8.6) accepts the new result.
- **When an Update is sent.** Only when the result (variation, reason or ruleId) or the effective version changes.

### 8.4 Criteria

A criterion takes one of three forms:

- An exact flag key, such as `checkout.new-flow`.
- A prefix ending in `*`, such as `checkout.*`.
- `*` alone, meaning all flags in the environment.

### 8.5 Definition mode

**DefinitionQuery (0x12)** is a one-shot fetch, intended for short-lived
processes.

```
[u8 0x12]
[u32 query_id]               // shares the §8.1 id space; strictly increasing
[u16 criteria_count]         // 1–1,000 (§2.2)
repeated: [u16 len][criterion]
```

**DefinitionResponse (0x13)**

```
[u8 0x13]
[u32 query_id]
[u8 response_flags]          // bit0 MORE: further frames for this query_id follow
[u16 item_count]
repeated:
  [u8 item_kind]             // 0 flag (§9.3), 1 segment (§9.4), 2 server-evaluated flag (§8.8)
  [u16 key_len][key]
  [u64 version]              // flags: effective version (§8.3); segments: own version
  [u32 def_len][definition]  // 0 ⇒ exact-key criterion not found; always 0 for item_kind 2
```

**DefinitionUpdate (0x23)**

```
[u8 0x23]
[u32 subscription_id]
[u64 sequence]
[u8 update_flags]
[u16 change_count]
repeated:
  [u8 change_kind]
  [u8 item_kind]
  [u16 key_len][key]
  [u64 version]              // flags: effective version (§8.3); segments: own version
  [u32 def_len][definition]  // 0 on delete and for item_kind 2
```

**Guarantees**

- **Segment closure.** Results and subscriptions always include every segment that the returned flags reference. Clients never subscribe to segments directly.
- **Segment changes.** A segment change is pushed to every definition subscription that has a flag referencing that segment.
- **Ordering.**
  - A segment upsert precedes any flag upsert that references it. This holds within a frame and across the frames of one snapshot.
  - A flag that stops referencing a segment is sent before that segment's delete.
  - A client that applies changes in order therefore never holds a dangling reference.
- **Per-subscription state.** Client state is kept per subscription. A segment delete on one subscription says nothing about another that still references the segment. An SDK that merges subscriptions into one store must count segment references per subscription.
- **Size.** A response or snapshot that would exceed `MaximumFrameSize` is split across frames, using `MORE` (QueryResponse, DefinitionResponse), or `SNAPSHOT` / `SNAPSHOT_END`.
- **Readiness.** An SDK reports ready once `SNAPSHOT_END` has arrived for every initial subscription.

### 8.6 Snapshots, reconnect and versions

- **Snapshots.** Every Subscribe, in either mode, begins with a snapshot of everything that matches.
  - Every snapshot frame carries `SNAPSHOT`, and the last one also carries `SNAPSHOT_END`.
  - A snapshot always has at least one frame. When nothing matches, it is a single `SNAPSHOT|SNAPSHOT_END` frame with `change_count = 0`.
  - An exact-key criterion that matches no flag is left out of the snapshot. The SDK reports such a key as `flag_not_found` locally. (A one-shot DefinitionResponse still reports it with `def_len 0`.)
  - Deltas follow the snapshot.
- **Reconnect.** The client performs a new Handshake, re-subscribes, and receives a fresh snapshot. Any key it held that is absent when `SNAPSHOT_END` arrives is deleted. There is no backfill across sessions.
- **Versions.**
  - A snapshot replaces the client's state for that subscription unconditionally, whatever versions it held before, including a persisted last-known-good cache. This keeps clients correct after versions restart low, for example after a Raft data-directory wipe or a backup restore (§13).
  - Within a session, a client applies a delta upsert only if its `version` is greater than the version it holds for that key. Versions are Raft log indexes, so they can be compared across server nodes of one cluster.
- **Sequences.** Sequence numbers are per subscription. They start at 1 and go up by exactly 1 per frame. TCP preserves order, so a gap or regression is a protocol violation, and the client closes the connection.

### 8.7 Flag message lifecycle summary

| Client wants | Send | Expect |
| --- | --- | --- |
| A value, once | Query | QueryResponse, or Nack |
| Values, pushed | Subscribe(mode 0) | Update snapshot, then deltas; or Nack |
| Definitions, once | DefinitionQuery | DefinitionResponse frames, or Nack |
| Definitions, pushed | Subscribe(mode 1) | DefinitionUpdate snapshot, then deltas; or Nack |
| Stop a subscription | Unsubscribe | — |

### 8.8 Evaluator levels

The evaluator level is how the rule language grows without breaking SDKs that
are already deployed.

- **What a level is.** An integer that names a cumulative set of evaluator features. **Level 1** is everything this specification defines.
  - Any addition to what an evaluator must understand (nesting, new operators, new node kinds) raises the level.
  - A change to the binary layout bumps `schema_version` instead (§9.1).
- **Recorded levels.** Each flag and segment records the minimum level needed to evaluate it. The server computes it on write and stores it in the definition header. A flag's **effective level** is the highest of its own level and the levels of the segments it references.
- **Server behaviour.** If a flag's effective level is higher than the session's `EVAL_LEVEL`, the server sends it as `item_kind 2` (*server-evaluated*): key and version only. It also withholds referenced segments that the client cannot evaluate.
- **SDK behaviour.** The SDK evaluates a server-evaluated flag through a value-mode Query on the same connection, using the caller's context.
  - It may cache results per `(version, context)`, where `version` is the item's effective version (§8.3), so segment changes invalidate the cache.
  - Results stay correct; only local evaluation is lost, and only for that flag.
  - The SDK must make this visible, through a log line or metric.
- **Level changes.** When a write changes a flag's effective level relative to a session, the next DefinitionUpdate switches the item between kind 0 and kind 2, in either direction. That write (to the flag or to a segment it references) raises the item's effective version (§8.3), which both kinds carry, so the switch passes the client's version check.

### 8.9 Error and Nack

**Error (0x50)** reports a connection-level failure. The server closes the
connection after sending it.

```
[u8 0x50]
[u16 code]
[u16 msg_len][msg]
```

**Nack (0x31)** rejects a single request. The connection and the session's other
subscriptions stay open.

```
[u8 0x31]
[u8 ref_kind]               // 0 query_id (Query or DefinitionQuery), 1 subscription_id
[u32 ref_id]
[u16 code]
[u16 msg_len][msg]
```

All codes are listed in §2.5.

---

## 9. Binary encoding

### 9.1 Design rules

- **No field names on the wire.** The schema version fixes the field order.
  - Each definition starts with `[u8 schema_version][u8 eval_level]`, so a cached copy describes itself. SDKs may persist definitions in exactly this form as their last-known-good store.
  - Adding, removing or reordering fields bumps `schema_version`. There are no optional or extension fields within a version.
- **Integer widths follow the limits** in §2.2.
- **References are indexes.** Variations are referenced by their `u8` index into the flag's variation table. Segments are referenced by key.
- **Value encoding.** Flag values are encoded by flag type. The same encoding is used for the `value` field of QueryResponse and Update, where `value_type` selects it and no extra length prefix is added:
  - `bool`: 1 byte (`00` or `01`).
  - `int`: 8 bytes, two's-complement int64.
  - `string`: `[u32 len][UTF-8]`.
  - `json`: `[u32 len][UTF-8 JSON text]`. The content is opaque application data.
- **Condition value kinds.**
  - `0`: bool, 1 byte.
  - `1`: number, IEEE-754 float64, 8 bytes.
  - `2`: string, `[u16 len][UTF-8]`.
- **Self-tagging nodes.** The first byte of every condition node is its node code (§2.4). A decoder therefore always knows how to parse the rest of the node.
  - Nesting needs no layout change.
  - Because the tag doubles as the operator byte, a tree costs no more bytes than a flat list.
- **Canonical form.** Every value has exactly one encoding, so encoders are byte-identical:
  - context attributes are sorted ascending by name bytes;
  - `salt_len` is 0 when the salt equals the flag key;
  - segment `included` and `excluded` keys are sorted ascending by bytes (§9.4).
- **Bounded recursion.** A decoder enforces its level's maximum group depth before recursing (1 at level 1). A deeper tree is `malformed_flag`.
- **Strict decoding.** A decoder:
  - checks every length against the bytes remaining;
  - never reads past `def_len` or `context_len`;
  - rejects unknown enum values, out-of-range indexes and trailing bytes;
  - rejects reserved bits that are set, and fields marked "must be 0" that are not;
  - rejects float64 values that are NaN or ±Inf;
  - rejects lists that break the canonical form: unsorted or duplicate context attribute names, or `included` / `excluded` entries.

  A definition that fails these checks is reported as `malformed_flag`. A context the server receives that fails them gets `Nack` 111. Any other message the server receives that fails them gets `Error` 115 (101 for the Handshake). A client that receives such a message closes the connection.

### 9.2 Message items

A flag's or segment's key and version travel in the message item envelope
(§8.5), not inside the definition. The environment is implied by the
credential.

### 9.3 Flag definition

```
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
  [group]                        // root node; must be a group
  [serve]

node: first byte is the node code
  group (0x80 all, 0x81 any):
    [u8 node_code]
    [u8 child_count]
    repeated [node]              // level 1: every child is a leaf
  leaf (0x00–0x7F):
    [condition]

condition:
  [u8 op]                        // node code
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

```
[u8 schema_version]              // 1
[u8 eval_level]                  // 1
[u16 included_count]
repeated: [u16 len][context key] // sorted ascending by bytes
[u16 excluded_count]
repeated: [u16 len][context key] // sorted ascending by bytes
[u8 rule_count]
repeated rule:
  [u8 id_len][id]
  [group]                        // as in §9.3; op 0x08 (inSegment) is invalid
```

`included` and `excluded` are sorted, so an SDK can binary-search them in
place.

### 9.5 Context

```
[u16 key_len][key]
[u8 attr_count]
repeated attribute:              // sorted ascending by name bytes
  [u8 name_len][name]
  [u8 kind]                      // value kind; bit7 set ⇒ list
  if scalar: [value]
  if list:   [u16 count] repeated [value]   // homogeneous; kind = kind & 0x7F
```

### 9.6 Worked example: the §6.1 flag

The flag in §6.1 encodes to **125 bytes**. The same content as minified JSON,
without the fields the envelope carries (`key`, `environmentId`, `version`) and the human-only fields (rule `description`, segment `name`), is 589 bytes. For comparison:

- the §6.2 segment is 64 bytes, against 190 as JSON;
- the §6.6 context is 58 bytes, against 92 as JSON.

```
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
01 03 6b 65 79                            serve rollout, bucketBy "key"
02 00 00 00 c3 50 01 00 00 c3 50          [0]→50000, [1]→50000
```

---

## 10. Frame examples

Every example below is a complete frame: the length prefix followed by the
payload.

**Handshake.** Version 0.1; `CAP_DEFINITIONS` with `EVAL_LEVEL` 1
(capabilities `0x0101`); server-default credential check interval; token
`tok-ABC`; no client id.

```
00 00 00 12                 length 18
01                          Handshake
00 01                       version 0.1
01 01                       capabilities: EVAL_LEVEL=1, CAP_DEFINITIONS
00 00                       credential_check_interval_s = server default
00 07 74 6f 6b 2d 41 42 43  token "tok-ABC"
00 00                       no client id
```

**HandshakeAck.** Session id `00..0f`; definitions granted; heartbeat every 15
seconds; credential check every 30 seconds.

```
00 00 00 18  02  00 01
00 01 02 03 04 05 06 07 08 09 0a 0b 0c 0d 0e 0f
01           server_flags: CAP_DEFINITIONS granted
00 0f        heartbeat_interval_s = 15
00 1e        credential_check_interval_s = 30
```

**Ack(SESSION_READY).** Healthy; the token is valid for 3,600 seconds.

```
00 00 00 0b  30  00  00 00 00 00  00  00 00 0e 10
```

**Ack(CHECK) and Ack(CHECK_OK).** The client sends `ref_id` 7. The server
replies degraded, with 1,200 seconds left on the token.

```
00 00 00 0b  30  01  00 00 00 07  00  00 00 00 00
00 00 00 0b  30  02  00 00 00 07  01  00 00 04 b0
```

**Reauthenticate and Ack(REAUTH_OK).** New token `tok-XYZ`, valid for 3,600
seconds.

```
00 00 00 0a  03  00 07 74 6f 6b 2d 58 59 5a
00 00 00 0b  30  03  00 00 00 00  00  00 00 0e 10
```

**Query.** `query_id` 7; context `{key:"u1"}`; key `f.a`.

```
00 00 00 15  10  00 00 00 07
00 00 00 05  00 02 75 31 00          context: key "u1", 0 attributes
00 01  00 03 66 2e 61                1 key: "f.a"
```

**QueryResponse.** `f.a` is boolean `true` via `fallthrough` at version 5;
`f.b` was not found. A single frame (`MORE` clear).

```
00 00 00 31  11  00 00 00 07  00  00 02
00 03 66 2e 61  02 00  00  01  04 74 72 75 65  00  00 00 00 00 00 00 00 05
00 03 66 2e 62  03 00  ff      00              00  00 00 00 00 00 00 00 00
```

**Subscribe.** Definition mode; `subscription_id` 1; criterion `checkout.*`.

```
00 00 00 18  20  00 00 00 01  01  00 00 00 00  00 01  00 0a 63 68 65 63 6b 6f 75 74 2e 2a
```

**Update.** Value mode; `subscription_id` 2; sequence 1; a single-frame snapshot
with one upsert.

```
00 00 00 28  22  00 00 00 02  00 00 00 00 00 00 00 01  03  00 01
00  00 03 66 2e 61  00 00 00 00 00 00 00 05
02 00 00  01  04 74 72 75 65  00
```

**DefinitionUpdate.** `subscription_id` 1; a single-frame snapshot carrying the
§6.2 segment and then the §6.1 flag. The payload is 258 bytes.

```
00 00 01 02  23  00 00 00 01  00 00 00 00 00 00 00 01  03  00 02
00 01  00 04 62 65 74 61  00 00 00 00 00 00 04 0e  00 00 00 40  <64 bytes, §9.4>
00 00  00 11 63 68 65 63 6b 6f 75 74 2e 6e 65 77 2d 66 6c 6f 77
       00 00 00 00 00 00 04 12  00 00 00 7d  <125 bytes, §9.6>
```

**Unsubscribe.** `subscription_id` 1.

```
00 00 00 05  21  00 00 00 01
```

**Nack.** Subscription 1 rejected with `ErrInvalidCriteria` (112).

```
00 00 00 1a  31  01  00 00 00 01  00 70  00 10 69 6e 76 61 6c 69 64 20 63 72 69 74 65 72 69 61
```

**Heartbeat.**

```
00 00 00 01  40
```

**Error.** `ErrAuthenticationFailed` (105).

```
00 00 00 1a  50  00 69  00 15 61 75 74 68 65 6e 74 69 63 61 74 69 6f 6e 20 66 61 69 6c 65 64
```

---

## 11. Conformance

A language-neutral vector set under `docs/fqdp-vectors/` holds four kinds of
case:

- **Evaluation cases.** Each case is a flag or segment plus one or more contexts, given both as JSON and as expected binary hex, together with the expected `{variationId, reason, ruleId, errorKind}`. Together the cases cover:
  - every operator;
  - `negate` with missing attributes;
  - `negate` on `inSegment` (matches non-members);
  - `bucketBy` on a number attribute of `-0` (bucketed as `"0"`);
  - `bucketBy` on a number attribute of 2^60 (unusable, bucket 0);
  - list attributes, including an empty list with `negate` (counts as missing);
  - segment precedence;
  - `all` and `any`;
  - rule order;
  - the kill switch;
  - the bucketing vectors (§6.5).
- **Malformed binary cases.** Each of these must decode as `malformed_flag`:
  - truncated lengths;
  - a bad index;
  - an unknown op;
  - trailing bytes;
  - a nested group at level 1;
  - a NaN or ±Inf condition value;
  - an unsorted segment `included` list;
  - a set reserved bit in `flag_bits`.
- **Frame cases.** The frames in §10, plus a level-2 flag offered to a level-1 session, which must arrive as `item_kind 2`.
- **Lifecycle cases.** Each of these must produce the result shown:

  | Case | Result |
  | --- | --- |
  | Request before `SESSION_READY` | 109 |
  | Handshake with a credential without `flags:read` | 105 |
  | Second Handshake, before or after `SESSION_READY` | 121 |
  | Client sends an Update (S→C type) | 100 |
  | Client sends `Ack(CHECK_OK)` | 115 |
  | `query_id` not higher than the previous one | `Nack` 122 |
  | Reauthenticate with another credential's token | 118 |
  | Token expiry without Reauthenticate | 116 |
  | Revocation found by the server-side check | 106 |
  | Revocation found by `Ack(CHECK)` | 106 |
  | Unknown `ack_kind` | 115 |
  | Heartbeat before `SESSION_READY` | accepted, no error |
  | Reauthenticate with a token without `flags:read` | 105 |
  | Subscribe that matches nothing | one `SNAPSHOT\|SNAPSHOT_END` frame, `change_count = 0` |
  | Reauthenticate that drops `flags:definitions:read` | `Nack` 110 for every definition subscription |
  | `flags:definitions:read` removed from the credential in the store | `Nack` 110 for every definition subscription |
  | `flags:read` removed from the credential in the store | 106 |

The Go server (evaluator and codec) and every SDK run the same vectors. That is
the guarantee that both modes give identical answers and that every encoder is
byte-compatible.

---

## 12. Security considerations

- **Tokens.** The Handshake token is a short-lived bearer token: the service token from the HTTP API's OAuth2 client-credentials endpoint. Human login tokens are not accepted (`Error` 105), because they can span several environments.
  - Scopes decide what the session may do: `flags:read` for value mode, and `flags:definitions:read` as well for definition mode.
  - The credential's environment decides which flags the session sees.
- **Token lifetime.** A session outlives its token only through Reauthenticate (§4.3). An expired token gets `Error` 116.
- **Revocation and scope removal.** Both take effect through the two-sided check (§4.4): no later than `credential_check_interval` after the change, even on a push-only session. A client can shorten that interval in the Handshake but never lengthen it past the server's maximum. Revocation, or losing `flags:read`, gets `Error` 106 and the connection is closed. Losing `flags:definitions:read` ends definition subscriptions with `Nack` 110.
- **Definition exposure.** Definition mode exposes every rule and every segment member list in the environment. Do not give `flags:definitions:read` to less-trusted clients.
- **TLS.** Strongly recommended in production. Servers may present certificates from a public CA, a private CA, or **self-signed** certificates. Clients and SDKs must support both:
  - **Custom trust roots:** a configurable CA bundle, used instead of or alongside the system roots.
  - **Certificate pinning:** trusting a specific server certificate or public key by its SHA-256 fingerprint.

  Turning verification off entirely may be offered for local development only, and SDKs must log a warning when it is used. SDKs must log the same warning when they connect without TLS, since the token then travels in clear text.
- **Abuse limits.**
  - Rate-limit new and unauthenticated connections per IP, and requests per session. The response is `Error` 107.
  - Enforce the handshake timeout (`Error` 120).
  - Cap active subscriptions per session (`Nack` 123).
- **Decoders.** All decoders follow the strict-decoding and bounded-recursion rules in §9.1.

---

## 13. Implementation notes

These notes are not normative.

- **Durability.** Flag and segment state lives in the Raft-replicated store; the protocol only delivers it. Versions are Raft log indexes.
- **Shared package.** One Go package (proposed: `backend/internal/flageval`) holds the model, write-time validation (§6), evaluation (§7) and the codec (§9). The HTTP API validates with it, and the FQDP server evaluates and encodes with it.
- **Encode once.** The server encodes each flag and segment once per write, and caches the result by `(environment, key, version)`, so fan-out to subscribers is a plain copy.
- **HTTP API.**
  - Flags are read and written as the §6.1 JSON document.
  - A full-definition `PUT` carries the `version` it was based on and gets `409` if that version is stale.
  - Segments get their own CRUD endpoints and permissions.
  - The credential scope catalog gains `flags:definitions:read`.
- **Audit.** Audit before/after values are whole definitions, which gives diffs and reverts a natural unit.
- **Store migration.** The model changes break the store's command and snapshot format. Under the pre-v1 stance, the Raft data directory is wiped on upgrade.
- **Session registry and health.** The server keeps each session's lifecycle state and last check results (§4). Healthy, expiring and total session counts feed the server's own health reporting, such as a cluster status endpoint and the admin UI's cluster health page.
- **Revocation push.** Applying a credential change in the store, including a scope change, triggers the server-side credential check. A periodic sweep covers anything the trigger misses.
- **Readiness.** `SESSION_READY` is the hook for node readiness. A node that has not caught up with the cluster leader withholds it and eventually sends `Error` 117. A node that is draining ends its sessions with `Error` 124.
- **Debugging.** During development, a decoding tool (for example `fqdpdump`) built on the reference codec takes the place of a text encoding.
- **Current code.** `backend/internal/fqdp/message_types.go` still defines an Admin message type (`0xF0`). It should be removed when the protocol is implemented.

---

## 14. Not in 0.1

- **Application-originated writes** over FQDP, and change-attribution metadata (actor, comment, origin).
- **Flag prerequisites** and dependencies between flags.
- **Rule-language extensions**, which arrive as later evaluator levels:
  - nested condition groups and the `not` group;
  - regex, semver and date operators.
- **Larger targeting models:** multi-kind contexts, and segments beyond the §2.2 limits ("big segments").
- **Version-based delta resume** on reconnect.
- **A text/JSON encoding.** Use the decoding tool instead (§13).
- **An Admin channel.** The earlier `0xF0` type is removed. Operational actions go through the HTTP API.
