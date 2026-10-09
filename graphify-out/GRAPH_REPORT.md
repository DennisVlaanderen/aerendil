# Graph Report - aerendil  (2026-10-09)

## Corpus Check
- 171 files · ~66,541 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 14 file(s) not represented in the graph (top: (none) 9, .css 2, .mdc 1)

## Summary
- 1582 nodes · 3999 edges · 58 communities (46 shown, 12 thin omitted)
- Extraction: 87% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 506 edges (avg confidence: 0.85)
- Token cost: 184,419 input · 0 output

## Community Hubs (Navigation)
- English UI Messages
- Dutch UI Messages
- API Handler Tests
- API Handlers & Audit Middleware
- API Response & Error Types
- Architecture Docs & FQDP Spec
- UI Hooks & Theme
- Auth Service & Users
- UI Server: Groups & Users
- Store Tests
- UI Dev Dependencies
- UI Runtime Dependencies
- UI Server Auth & Env
- UI Shared Client Libs
- Layout Components & Toasts
- Sidebar Navigation
- Dashboard CRUD Pages
- Credentials Dashboard Page
- Client Credentials Auth Tests
- Header & Locale Switcher
- UI Credentials Server Module
- Confirm Modal
- UI Errors & Flags Server
- UI Permissions & Environments
- Groups Dashboard Page
- Audit Log & Flash
- UI API Client & Users Page
- Server Bootstrap & Admin Seed
- CI/CD & Security Workflows
- Application Credential Store
- Flag Detail Page
- UI TypeScript Config
- FQDP Server
- Flag Store
- Raft FSM Core
- Audit Store
- Permission Resolution Tests
- Environment Store
- Group Store
- Brand Logo Assets
- UI npm Scripts
- Server Config & Main Tests
- ESLint Config
- Inlang i18n Settings
- Test Snapshot Sink
- Component Browser Tests
- Issue Templates: Stories
- Security Policy
- FSM Snapshot
- Prettier Config
- FSM Command Apply
- Copilot Instructions
- App Type Declarations
- Root Layout
- Bug Report Template
- Stale PR Workflow
- Go Module

## God Nodes (most connected - your core abstractions)
1. `newTestMux()` - 116 edges
2. `tokenFor()` - 68 edges
3. `seedEnvironmentForTest()` - 48 edges
4. `hasPermission()` - 46 edges
5. `tokenForWithEnvironments()` - 44 edges
6. `getAuthToken()` - 44 edges
7. `getSession()` - 36 edges
8. `NewID()` - 35 edges
9. `newTestStore()` - 35 edges
10. `@sveltejs/kit` - 34 edges

## Surprising Connections (you probably didn't know these)
- `FQDP security considerations (scoped tokens, TLS, rate limits)` --semantically_similar_to--> `Application credentials / OAuth2 client-credentials service tokens`  [INFERRED] [semantically similar]
  docs/fqdp.md → AGENTS.md
- `Durable store + replication, idempotent versioned updates` --conceptually_related_to--> `Raft-replicated Store (backend/internal/store)`  [INFERRED]
  docs/fqdp.md → AGENTS.md
- `Query / QueryResponse` --conceptually_related_to--> `Raft FSM command envelope + snapshotDoc`  [INFERRED]
  docs/fqdp.md → AGENTS.md
- `Handshake / HandshakeAck token authentication` --conceptually_related_to--> `auth Service (HS256 JWT + bcrypt)`  [INFERRED]
  docs/fqdp.md → AGENTS.md
- `FQDP server stub (backend/internal/fqdp)` --references--> `Error message (u16 code + message)`  [EXTRACTED]
  AGENTS.md → docs/fqdp.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Required status checks gating Dependabot auto-merge** — _github_workflows_go_go_workflow, _github_workflows_node_js_node_js_ci_workflow, _github_workflows_codeql_codeql_workflow, _github_workflows_docker_docker_workflow, _github_workflows_dependabot_auto_merge_dependabot_auto_merge_workflow [EXTRACTED 1.00]
- **Layered security scanning (SAST, dependency CVEs, image CVEs)** — _github_workflows_codeql_codeql_workflow, _golangci_gosec, _github_workflows_go_govulncheck, _github_workflows_docker_trivy_scan [INFERRED 0.85]
- **GitHub issue templates** — _github_issue_template_bug_report_bug_report_template, _github_issue_template_feature_request_feature_request_template, _github_issue_template_user_story_user_story_template [EXTRACTED 1.00]
- **main.go wires Store, FQDP listener and HTTP API over one flag store** — agents_raft_store, agents_fqdp_server_stub, agents_http_api [EXTRACTED 1.00]
- **UI request path: client -> BFF -> lib/server bridge -> backend API (permission-gated)** — agents_bff_proxy, agents_haspermission, agents_server_bridge, agents_http_api, agents_requirepermission_withaudit [EXTRACTED 1.00]
- **FQDP subscription delivery with ordering, ack and backfill** — docs_fqdp_subscribe_update, docs_fqdp_ack_ordering, docs_fqdp_reconnect_backfill, docs_fqdp_heartbeat [INFERRED 0.85]
- **Aerendil Brand Asset Family (logos, marks, favicon)** — ui_static_aerendil_logo_image, ui_static_aerendil_logo_dark_image, ui_static_aerendil_mark_image, ui_static_aerendil_mark_dark_image, ui_static_favicon_image, docs_aerendil_logo_image, docs_aerendil_logo_dark_image [INFERRED 0.95]

## Communities (58 total, 12 thin omitted)

### Community 0 - "English UI Messages"
Cohesion: 0.01
Nodes (211): application_credentials_create_button, application_credentials_create_environment_label, application_credentials_create_name_label, application_credentials_create_no_environment, application_credentials_create_scopes_label, application_credentials_create_submit, application_credentials_delete_button, application_credentials_delete_confirm_description (+203 more)

### Community 1 - "Dutch UI Messages"
Cohesion: 0.01
Nodes (211): application_credentials_create_button, application_credentials_create_environment_label, application_credentials_create_name_label, application_credentials_create_no_environment, application_credentials_create_scopes_label, application_credentials_create_submit, application_credentials_delete_button, application_credentials_delete_confirm_description (+203 more)

### Community 2 - "API Handler Tests"
Cohesion: 0.05
Nodes (139): TestMeHandlerRequiresAuthentication(), TestMeHandlerResolvesEnvironmentNamesWithoutEnvironmentsReadPermission(), TestMeHandlerReturnsResolvedPrincipal(), createApplicationCredentialForTest(), TestApplicationCredentialsAuditRedactsClientSecret(), TestApplicationCredentialsFullCRUD(), TestApplicationCredentialsGetByID(), TestApplicationCredentialsGetByIDRejectsInaccessibleEnvironment() (+131 more)

### Community 3 - "API Handlers & Audit Middleware"
Cohesion: 0.06
Nodes (93): apiHandlerFunc, auditConfig, auditRecorder, environmentResponse, groupResponse, resolvedPrincipal, userResponse, created() (+85 more)

### Community 4 - "API Response & Error Types"
Cohesion: 0.16
Nodes (14): apiError, apiResponse, applicationCredentialResponse, applicationCredentialSecretResponse, oauthErrorResponse, principalContextKey, isProductionEnvironment(), jwtSecretFromEnvironment() (+6 more)

### Community 5 - "Architecture Docs & FQDP Spec"
Cohesion: 0.06
Nodes (28): Seeded immutable Admin group + bootstrap admin user, Aerendil (distributed feature flagging toolkit), aerendil.auth httpOnly cookie session (getSession), auth Service (HS256 JWT + bcrypt), FQDP server stub (backend/internal/fqdp), hasPermission(session, perm) shared UI gate, HTTP API (backend/internal/api, stdlib ServeMux), Application credentials / OAuth2 client-credentials service tokens (+20 more)

### Community 6 - "UI Hooks & Theme"
Cohesion: 0.06
Nodes (8): handle, formatTimestamp(), Locale, Override, theme, ThemeStore, { data }, expandedIds

### Community 7 - "Auth Service & Users"
Cohesion: 0.10
Nodes (9): EnvironmentSet, PermissionSet, Service, Service, User, fsm, User, isActiveAdmin() (+1 more)

### Community 8 - "UI Server: Groups & Users"
Cohesion: 0.16
Nodes (22): hasPermission(), getAuthToken(), ALL_PERMISSIONS, createGroup(), deleteGroup(), GroupResult, listGroups(), updateGroup() (+14 more)

### Community 9 - "Store Tests"
Cohesion: 0.13
Nodes (31): TestStoreDeleteApplicationCredential(), TestStoreDeleteEnvironmentRejectsWhenCredentialsStillReferenceIt(), TestStoreListApplicationCredentialsReturnsStableOrder(), TestStoreSetAndGetApplicationCredential(), newTestStore(), seedEnvironment(), TestStoreAuditsAppendAndList(), TestStoreDeleteEnvironmentAllowsNonLastEnvironment() (+23 more)

### Community 10 - "UI Dev Dependencies"
Cohesion: 0.06
Nodes (32): devDependencies, eslint, eslint-config-prettier, @eslint/js, eslint-plugin-svelte, @fontsource/ibm-plex-mono, @fontsource/sora, globals (+24 more)

### Community 11 - "UI Runtime Dependencies"
Cohesion: 0.07
Nodes (28): @fontsource/ibm-plex-mono, @fontsource/sora, @iconify-json/lucide, @iconify-json/lucide-lab, @iconify/tailwind4, @inlang/paraglide-js, playwright, prettier (+20 more)

### Community 12 - "UI Server Auth & Env"
Cohesion: 0.13
Nodes (17): @sveltejs/kit, variables, clearAuthCookie(), getSelectedEnvironmentId(), login(), parseSession(), Session, setAuthCookie() (+9 more)

### Community 13 - "UI Shared Client Libs"
Cohesion: 0.14
Nodes (12): localizedResolve(), resolveUntyped, createError, { data }, handleCreate(), isCreating, confirmDelete(), goToLogin() (+4 more)

### Community 14 - "Layout Components & Toasts"
Cohesion: 0.10
Nodes (10): choose(), container, {
		environments,
		selectedEnvironmentId
	}, open, selected, toast, ToastStore, { data, children } (+2 more)

### Community 15 - "Sidebar Navigation"
Cohesion: 0.09
Nodes (17): applicationSettingsContainer, applicationSettingsDefaultPath, applicationSettingsOpen, canCreateFlags, canSeeApplicationCredentials, canSeeApplicationSettings, canSeeAuditLog, canSeeEnvironments (+9 more)

### Community 16 - "Dashboard CRUD Pages"
Cohesion: 0.13
Nodes (20): apiRequest(), resolveErrorMessage(), confirmDelete(), confirmRotate(), handleCreate(), handleUpdate(), handleCreate(), handleUpdate() (+12 more)

### Community 17 - "Credentials Dashboard Page"
Cohesion: 0.09
Nodes (15): ApplicationCredentialSummary, copied, createError, { data }, deleteModal, isCreating, pendingDeleteName, pendingRotateName (+7 more)

### Community 18 - "Client Credentials Auth Tests"
Cohesion: 0.20
Nodes (21): seedCredential(), TestAuthenticateClientCredentialsFailsForInactiveCredential(), TestAuthenticateClientCredentialsFailsForUnknownClientID(), TestAuthenticateClientCredentialsFailsForWrongSecret(), TestAuthenticateClientCredentialsSucceeds(), TestAuthenticateTokenRejectsServiceTokenAfterCredentialDeactivation(), TestAuthenticateTokenStillResolvesHumanTokens(), TestGenerateServiceTokenRoundTripsThroughAuthenticateToken() (+13 more)

### Community 19 - "Header & Locale Switcher"
Cohesion: 0.10
Nodes (8): title, {
		username,
		environments,
		selectedEnvironmentId
	}, { compact = false }, container, localeMeta, open, openUpward, getInitials()

### Community 20 - "UI Credentials Server Module"
Cohesion: 0.20
Nodes (15): ErrorCode, ApplicationCredentialResult, ApplicationCredentialSecretResult, createApplicationCredential(), deleteApplicationCredential(), listApplicationCredentials(), rotateApplicationCredential(), toSecretResult() (+7 more)

### Community 21 - "Confirm Modal"
Cohesion: 0.11
Nodes (13): dialogEl, handleBackdropClick(), Props, requestCancel(), {
		title,
		description,
		confirmLabel,
		cancelLabel,
		variant = 'default',
		onconfirm,
		oncancel
	}, canUpdate, { data }, deleteError (+5 more)

### Community 22 - "UI Errors & Flags Server"
Cohesion: 0.18
Nodes (10): vitest, errorMessages, createFlags(), deleteFlag(), FlagResult, FlagsResult, updateFlag(), DELETE() (+2 more)

### Community 23 - "UI Permissions & Environments"
Cohesion: 0.19
Nodes (11): EnvironmentAware, PermissionAware, createEnvironment(), deleteEnvironment(), EnvironmentResult, listEnvironments(), updateEnvironment(), DELETE() (+3 more)

### Community 24 - "Groups Dashboard Page"
Cohesion: 0.11
Nodes (10): svelte, GroupSummary, confirmDelete(), createError, { data }, deleteModal, isCreating, pendingDeleteGroupName (+2 more)

### Community 25 - "Audit Log & Flash"
Cohesion: 0.16
Nodes (10): AuditEntry, AuditLogFilter, listAuditLog(), FlashReason, readFlash(), setFlash(), load(), load() (+2 more)

### Community 26 - "UI API Client & Users Page"
Cohesion: 0.12
Nodes (9): ApiResult, UserSummary, canCreate, canUpdate, createError, { data }, isCreating, pendingToggleUser (+1 more)

### Community 27 - "Server Bootstrap & Admin Seed"
Cohesion: 0.17
Nodes (12): main(), run(), storeConfigFromEnvironment(), hasOtherActiveAdmin(), SeedAdminGroupAndUser(), seedAdminGroupAndUserOnce(), AdminConfig, TestSeedAdminGroupAndUserIsIdempotent() (+4 more)

### Community 28 - "CI/CD & Security Workflows"
Cohesion: 0.19
Nodes (12): CodeQL Config (security-extended queries), Dependabot Config, CodeQL Workflow, Dependabot Auto-Merge Workflow, DEPENDABOT_COMPAT_PAT secret, Docker Workflow (build-only image validation), Trivy Image Scan (report-only SARIF), Go Workflow (build, test, lint, govulncheck) (+4 more)

### Community 29 - "Application Credential Store"
Cohesion: 0.19
Nodes (3): ApplicationCredential, fsm, ApplicationCredentialRepository

### Community 30 - "Flag Detail Page"
Cohesion: 0.15
Nodes (12): canDelete, canUpdate, confirmDelete(), { data }, deleteError, deleteModal, flagUrl(), handleSave() (+4 more)

### Community 31 - "UI TypeScript Config"
Cohesion: 0.13
Nodes (14): $app/tsconfig, compilerOptions, allowJs, checkJs, esModuleInterop, forceConsistentCasingInFileNames, moduleResolution, resolveJsonModule (+6 more)

### Community 32 - "FQDP Server"
Cohesion: 0.22
Nodes (6): handleHandshake(), readFrame(), sendError(), serveConnection(), StartTCPServer(), FQDPMessageType

### Community 33 - "Flag Store"
Cohesion: 0.22
Nodes (3): fsm, Flag, FlagRepository

### Community 35 - "Audit Store"
Cohesion: 0.29
Nodes (4): AuditEntry, fsm, AuditFilter, AuditRepository

### Community 36 - "Permission Resolution Tests"
Cohesion: 0.31
Nodes (10): TestResolveAdminGroupBypasses(), TestResolveFailsForDeactivatedUser(), TestResolveFailsForUnknownUser(), TestResolveUnionsEnvironmentIDsAcrossGroups(), TestResolveUnionsPermissionsAcrossGroups(), TestResolveUserWithNoEnvironmentGrantsHasNoEnvironmentAccess(), TestResolveUserWithNoGroupsHasNoPermissions(), NewService() (+2 more)

### Community 37 - "Environment Store"
Cohesion: 0.31
Nodes (3): Environment, fsm, EnvironmentRepository

### Community 38 - "Group Store"
Cohesion: 0.31
Nodes (3): fsm, Group, GroupRepository

### Community 39 - "Brand Logo Assets"
Cohesion: 0.25
Nodes (10): Aerendil Logo (Dark, docs), Aerendil Logo (Light, docs), Aerendil Wordmark, Aerendil Logo (Dark, UI static), Aerendil Logo (Light, UI static), Aerendil Mark (Dark), Aerendil Mark (Light), Pill Emblem with Dot and Four-Point Star (+2 more)

### Community 40 - "UI npm Scripts"
Cohesion: 0.18
Nodes (11): scripts, build, check, check:watch, dev, format, lint, prepare (+3 more)

### Community 41 - "Server Config & Main Tests"
Cohesion: 0.28
Nodes (7): adminConfigFromEnvironment(), isProductionEnvironment(), newAPIServer(), TestAdminConfigFromEnvironmentFallsBackInDevelopment(), TestAdminConfigFromEnvironmentUsesConfiguredCredentials(), TestIsProductionEnvironment(), TestNewAPIServerTimeouts()

### Community 42 - "ESLint Config"
Cohesion: 0.22
Nodes (7): eslint, eslint-config-prettier, @eslint/js, eslint-plugin-svelte, globals, typescript-eslint, gitignorePath

### Community 43 - "Inlang i18n Settings"
Cohesion: 0.29
Nodes (6): baseLocale, locales, modules, plugin.inlang.messageFormat, pathPattern, $schema

### Community 45 - "Component Browser Tests"
Cohesion: 0.40
Nodes (3): vitest-browser-svelte, baseProps, baseProps

### Community 46 - "Issue Templates: Stories"
Cohesion: 0.50
Nodes (4): Feature Request Issue Template, Acceptance Criteria, Definition of Done, User Story Issue Template

### Community 47 - "Security Policy"
Cohesion: 0.50
Nodes (4): CI workflows (go.yml, node.js.yml, codeql.yml, docker.yml), SECURITY.md (Security Policy), Automated scanning (CodeQL, govulncheck, Trivy), GitHub private vulnerability reporting

## Knowledge Gaps
- **38 isolated node(s):** `aerendil`, `@fontsource/ibm-plex-mono`, `@fontsource/sora`, `@iconify-json/lucide`, `@iconify-json/lucide-lab` (+33 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 771 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **12 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `vitest` connect `UI Errors & Flags Server` to `UI Server: Groups & Users`, `UI Runtime Dependencies`, `UI Server Auth & Env`, `Component Browser Tests`, `UI Credentials Server Module`, `UI Permissions & Environments`, `UI API Client & Users Page`?**
  _High betweenness centrality (0.035) - this node is a cross-community bridge._
- **Are the 100 inferred relationships involving `newTestMux()` (e.g. with `TestMeHandlerRequiresAuthentication()` and `TestMeHandlerResolvesEnvironmentNamesWithoutEnvironmentsReadPermission()`) actually correct?**
  _`newTestMux()` has 100 INFERRED edges - model-reasoned connections that need verification._
- **What connects `aerendil`, `@fontsource/ibm-plex-mono`, `@fontsource/sora` to the rest of the system?**
  _38 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `English UI Messages` be split into smaller, more focused modules?**
  _Cohesion score 0.009433962264150943 - nodes in this community are weakly interconnected._
- **Why does `resolveErrorMessage()` connect `Dashboard CRUD Pages` to `UI Shared Client Libs`, `Layout Components & Toasts`, `Credentials Dashboard Page`, `Confirm Modal`, `UI Errors & Flags Server`, `Groups Dashboard Page`, `UI API Client & Users Page`, `Flag Detail Page`?**
  _High betweenness centrality (0.032) - this node is a cross-community bridge._
- **Are the 60 inferred relationships involving `tokenFor()` (e.g. with `TestMeHandlerReturnsResolvedPrincipal()` and `TestApplicationCredentialsAuditRedactsClientSecret()`) actually correct?**
  _`tokenFor()` has 60 INFERRED edges - model-reasoned connections that need verification._
- **Should `Dutch UI Messages` be split into smaller, more focused modules?**
  _Cohesion score 0.009433962264150943 - nodes in this community are weakly interconnected._