# REST API Standards for Open-Source Projects

This document defines practical conventions for designing, implementing,
documenting, and maintaining REST-style HTTP APIs in this project. It is
intended to be a shared baseline for contributors. If an existing
project convention conflicts with this guide, document the exception and
apply it consistently.

## 1. Design principles

- **Consistency:** Similar resources should behave and be named
    similarly.
- **Resource-oriented design:** Model the API around resources, not
    action names.
- **HTTP semantics:** Use HTTP methods and status codes according to
    their intended meaning.
- **Predictability:** Keep request formats, response envelopes,
    errors, and pagination consistent.
- **Compatibility:** Avoid breaking existing clients without a
    deliberate versioning and migration plan.
- **Security by default:** Authenticate and authorize appropriately,
    validate input, and avoid leaking sensitive data.
- **Documentation:** Every public endpoint must be documented with
    examples and its error behavior.

## 2. URL and resource naming

### 2.1 Use nouns for resources

Use plural nouns for collection endpoints and identify individual
resources with path parameters.

Good:

``` http
GET    /api/v1/users
GET    /api/v1/users/{userId}
POST   /api/v1/users
PATCH  /api/v1/users/{userId}
DELETE /api/v1/users/{userId}
```

Avoid action-oriented paths such as `/getUsers`, `/createUser`, or
`/deleteUser`. Use a nested path only when the child resource is
meaningfully scoped by its parent, for example
`/api/v1/users/{userId}/keys`.

### 2.2 Naming conventions

- Use lowercase path segments.
- Use hyphens for multi-word path segments, such as
    `/api/v1/access-tokens`.
- Use stable, descriptive resource names.
- Use path parameters to identify resources and query parameters to
    filter, sort, search, or paginate collections.
- Avoid trailing slashes unless the project deliberately requires
    them.
- Do not expose database table names, internal class names, or
    implementation details in public URLs.
- Treat path and query parameter names as case-sensitive; use one
    convention consistently.

### 2.3 API prefix and version

Use a consistent prefix, such as `/api/v1`. Version the public API when
a change is incompatible with existing clients. Do not introduce a new
version for routine additive changes that preserve existing behavior.

## 3. HTTP methods

Use methods according to their standard semantics.

| Method   | Intended use                                            | Typical request body | Safe | Idempotent                     |
| -------- | ------------------------------------------------------- | -------------------- | ---- | ------------------------------ |
| `GET`    | Retrieve a resource or collection                       | No                   | Yes  | Yes                            |
| `HEAD`   | Retrieve headers without the response body              | No                   | Yes  | Yes                            |
| `POST`   | Create a resource or perform a non-idempotent operation | Usually              | No   | No, by default                 |
| `PUT`    | Replace a resource at a known URI                       | Usually              | No   | Yes                            |
| `PATCH`  | Partially modify a resource                             | Usually              | No   | Depends on the patch semantics |
| `DELETE` | Remove a resource                                       | Usually no           | No   | Yes                            |

A method being idempotent means that repeating the same request has the
same intended effect on server state; it does not require identical
responses. Do not use `GET` to change state. Do not use `PUT` for
partial updates unless the API explicitly defines that behavior.

For retry-sensitive operations, consider an idempotency-key mechanism
and document its scope, retention period, and duplicate-request
behavior.

## 4. Request conventions

### 4.1 Headers

Use standard headers where applicable:

- `Accept` to indicate acceptable response media types.
- `Content-Type` to describe the request body.
- `Authorization` for credentials, commonly using `Bearer` tokens.
- `If-Match` and `ETag` for conditional updates where concurrency
    control is needed.
- `Idempotency-Key` for supported operations where clients may retry
    requests.

Use `application/json` for JSON request and response bodies unless the
endpoint documents another media type. Return
`415 Unsupported Media Type` for unsupported request content types when
appropriate, and `406 Not Acceptable` when the API cannot provide an
acceptable representation.  
Customs header must follow the [RFC-6648 Standard](https://www.rfc-editor.org/info/rfc6648/).

### 4.2 JSON conventions

- Use valid JSON and a consistent field-naming convention; this guide
    recommends `camelCase`.
- Represent dates and times as ISO 8601 / RFC 3339 strings with an
    explicit timezone, preferably UTC, for example
    `2026-10-09T11:30:00Z`.
- Use `null` only when it has a defined meaning. Distinguish a missing
    field from an explicitly null field.
- Use JSON numbers for numeric values, booleans for true/false values,
    arrays for ordered lists, and objects for structured data.
- Do not return secrets, password hashes, internal stack traces, or
    fields the caller is not authorized to see.
- Document numeric precision and units where they may be ambiguous.

### 4.3 Validation

Validate all untrusted input on the server, including path parameters,
query parameters, headers, and request bodies. Enforce required fields,
types, allowed values, length limits, and business rules. Return a
structured `400 Bad Request` or `422 Unprocessable Content` response
according to the project's documented convention; use one consistently.

## 5. Responses and status codes

Use meaningful HTTP status codes rather than returning `200 OK` for
every outcome.

| Status                       | Use                                                                                     |
| ---------------------------- | --------------------------------------------------------------------------------------- |
| `200 OK`                     | Successful request with a response body                                                 |
| `201 Created`                | Resource created; include `Location` when a new resource URI is available               |
| `202 Accepted`               | Request accepted for asynchronous processing                                            |
| `204 No Content`             | Successful request with no response body                                                |
| `304 Not Modified`           | Conditional request indicates the cached representation is still current                |
| `400 Bad Request`            | Malformed request or invalid request syntax                                             |
| `401 Unauthorized`           | Missing or invalid authentication credentials                                           |
| `403 Forbidden`              | Authenticated caller is not permitted to perform the action                             |
| `404 Not Found`              | Resource does not exist or is intentionally undisclosed                                 |
| `405 Method Not Allowed`     | Method is not supported for the resource; include `Allow` when applicable               |
| `409 Conflict`               | Request conflicts with the current resource state                                       |
| `412 Precondition Failed`    | A request precondition, such as `If-Match`, was not met                                 |
| `413 Content Too Large`      | Request content exceeds the permitted limit                                             |
| `415 Unsupported Media Type` | Request media type is unsupported                                                       |
| `422 Unprocessable Content`  | Content is syntactically valid but fails semantic validation, if adopted by the project |
| `429 Too Many Requests`      | Rate limit exceeded; include `Retry-After` when useful                                  |
| `500 Internal Server Error`  | Unexpected server-side failure                                                          |
| `502 Bad Gateway`            | Gateway or proxy received an invalid upstream response                                  |
| `503 Service Unavailable`    | Service is temporarily unable to handle the request                                     |
| `504 Gateway Timeout`        | Gateway or proxy did not receive a timely upstream response                             |

Choose `400` or `422` for validation failures according to the
documented project convention. Do not expose internal exception details
in production responses.

## 6. Response body conventions

Use a consistent response structure. A simple approach is to return the
resource representation directly for single-resource responses and an
object containing collection metadata for list responses. Avoid wrapping
every response in a generic `data` object unless the project has a clear
reason to do so.

Example single resource:

``` json
{
  "id": "usr_123",
  "displayName": "Alex Example",
  "createdAt": "2026-10-09T11:30:00Z"
}
```

Example collection:

``` json
{
  "items": [
    {
      "id": "usr_123",
      "displayName": "Alex Example"
    }
  ],
  "page": {
    "limit": 20,
    "nextCursor": "eyJvZmZzZXQiOjIwfQ"
  }
}
```

These are example conventions, not mandatory JSON shapes for every
endpoint. Pick a shape and use it consistently.

## 7. Error format

Use a predictable, machine-readable error structure. Consider adopting
the standard `application/problem+json` format described by RFC 9457
(Problem Details for HTTP APIs).

Example:

``` json
{
  "type": "https://example.org/problems/validation-error",
  "title": "Request validation failed",
  "status": 422,
  "detail": "One or more fields are invalid.",
  "instance": "/api/v1/users",
  "errors": [
    {
      "field": "email",
      "code": "invalid_format",
      "message": "Must be a valid email address."
    }
  ],
  "requestId": "req_abc123"
}
```

- The `status` value must agree with the HTTP status code.
- Keep error `code` values stable so clients can handle them
    programmatically.
- Make messages useful but do not disclose secrets, personal data,
    stack traces, SQL, or internal topology.
- Include a request or correlation ID when available.
- Document which error codes each endpoint can return.
- Do not use a successful HTTP status for an unsuccessful operation.

## 8. Filtering, sorting, searching, and pagination

### 8.1 Filtering

Use query parameters to filter collections, with documented names and
allowed values.

``` http
GET /api/v1/orders?status=pending&customerId=cus_123
```

Define how repeated filters behave, whether unknown filters are
rejected, and how empty values are interpreted.

### 8.2 Sorting

Use a documented parameter such as `sort`. Specify allowed fields and
sort direction.

``` http
GET /api/v1/users?sort=-createdAt,displayName
```

In this example, a leading `-` means descending order. If this syntax is
adopted, document it and reject unsupported sort fields rather than
silently guessing.

### 8.3 Searching

Use a documented parameter such as `q` for free-text search. Define
matching behavior, case sensitivity, and which fields are searchable.
Apply authorization rules to search results just as for ordinary reads.

### 8.4 Pagination

Paginate collections that can grow large. Cursor-based pagination is
generally preferable for large or frequently changing datasets;
limit/offset pagination may be suitable for small or stable datasets.

``` http
GET /api/v1/events?limit=50&cursor=opaque-token
```

- Set a default page size and a maximum permitted page size.
- Return a stable continuation token or next link when more results
    exist.
- Do not expose sensitive database internals in cursors.
- Define ordering so results are predictable.
- Document whether totals are returned and whether they are exact.
- Validate page sizes and cursor values.

## 9. Authentication, authorization, and security

- Use HTTPS for all non-local traffic.
- Authenticate callers using the project's chosen mechanism, such as
    OAuth 2.0 / OpenID Connect or scoped bearer tokens, where
    appropriate.
- Authorize every request at the resource and action level.
    Authentication alone does not grant access.
- Apply least privilege and deny access by default.
- Validate input and encode output appropriately; use parameterized
    database queries.
- Configure CORS narrowly when browser clients require it. CORS is not
    an authentication mechanism.
- Apply rate limits and request-size limits.
- Avoid putting credentials, access tokens, or sensitive personal data
    in URLs.
- Store secrets securely and never commit real credentials to source
    control.
- Redact credentials and sensitive data from logs.
- Use generic authentication failure responses where detailed errors
    could enable account enumeration.
- Protect state-changing requests against relevant threats for the
    chosen authentication model, including CSRF where cookie-based
    authentication is used.
- Establish a policy for dependency updates and security vulnerability
    reports.

## 10. Caching and concurrency

Use caching where it improves performance without violating freshness or
privacy requirements.

- Use `Cache-Control` to define whether and how responses may be
    cached.
- Use `ETag` and conditional requests such as `If-None-Match` for
    cache validation where useful.
- Use `ETag` with `If-Match` or another explicit concurrency mechanism
    to prevent lost updates when needed.
- Do not mark personalized or sensitive responses as publicly
    cacheable unless the design explicitly makes that safe.
- Document freshness, invalidation, and concurrency behavior for
    relevant endpoints.

## 11. Compatibility and versioning

Treat the following as potentially breaking changes:

- Removing or renaming a field, endpoint, or enum value.
- Changing a field's type or meaning.
- Making an optional request field required.
- Changing authorization requirements or response semantics in a way
    that breaks clients.
- Changing pagination or ordering behavior incompatibly.

Prefer additive changes when possible. Clients should generally tolerate
unknown response fields, and servers should avoid rejecting harmless
unknown fields unless strict validation is required. Document
deprecations, provide a migration path, and announce removal dates. Use
the project's chosen versioning policy consistently.

## 12. Rate limiting and asynchronous work

- Publish rate-limit behavior where clients need to plan around it.
- Return `429 Too Many Requests` when a client exceeds a limit and use
    `Retry-After` when possible.
- For long-running operations, consider returning `202 Accepted` with
    a status resource or operation URL.
- Document polling intervals, terminal states, failure reporting, and
    cancellation behavior for asynchronous jobs.
- Make retries safe where possible; do not assume clients will send a
    request only once.

## 13. Documentation requirements

Document every public endpoint with:

- Purpose and HTTP method.
- Path parameters and query parameters, including types, defaults,
    constraints, and examples.
- Authentication and authorization requirements.
- Request headers and content types.
- Request and response schemas with examples.
- Success and error status codes.
- Pagination, filtering, sorting, and rate-limit behavior where
    applicable.
- Side effects, idempotency, and retry guidance where relevant.
- Deprecation or compatibility notes.

Use OpenAPI to describe the API when practical, and validate the
specification in CI. Keep examples aligned with actual behavior.

## 14. Testing and quality gates

For each endpoint, test at least:

- Expected success cases.
- Missing, malformed, and boundary-value input.
- Authentication and authorization failures.
- Resource-not-found behavior.
- Correct status codes and response schemas.
- Pagination and filtering behavior.
- Duplicate or retried requests where relevant.
- Rate-limit and concurrency behavior where applicable.
- Protection against unauthorized access to another user's resources.
- Backward compatibility for supported clients.

Automated checks should run in CI. Useful checks include formatting and
linting, unit and integration tests, OpenAPI validation, and
security/dependency scanning.

## 15. Recommended endpoint checklist

Before merging a new or changed endpoint, confirm:

- [ ] The URL is resource-oriented and follows the project's naming
    convention.
- [ ] The HTTP method and status codes match the operation's
    semantics.
- [ ] Request and response schemas are documented and consistent.
- [ ] Input is validated on the server.
- [ ] Authentication and authorization are enforced.
- [ ] Errors use the standard error format.
- [ ] Pagination and limits are considered for collection endpoints.
- [ ] Sensitive values are excluded from responses and logs.
- [ ] Tests cover success, failure, permissions, and edge cases.
- [ ] OpenAPI documentation and examples are updated.
- [ ] Compatibility and deprecation implications are considered.

## 16. Standards and references

Use the current versions of these standards and specifications as the
authoritative references:

- **HTTP Semantics:** [RFC
    9110](https://www.rfc-editor.org/rfc/rfc9110)
- **HTTP Caching:** [RFC 9111](https://www.rfc-editor.org/rfc/rfc9111)
- **Problem Details for HTTP APIs:** [RFC
    9457](https://www.rfc-editor.org/rfc/rfc9457)
- **JSON:** [RFC 8259](https://www.rfc-editor.org/rfc/rfc8259)
- **URI Generic Syntax:** [RFC
    3986](https://www.rfc-editor.org/rfc/rfc3986)
- **Unstandardized parameter naming** [RFC 6648](https://www.rfc-editor.org/info/rfc6648/)
- **OpenAPI Specification:** [openapis.org](https://www.openapis.org/)
- **OWASP API Security Project:**
    [owasp.org/www-project-api-security](https://owasp.org/www-project-api-security/)

This guide is a project convention, not a claim that every
recommendation is mandated by an RFC. Standards define protocol
behavior; this document chooses consistent defaults for this codebase.
