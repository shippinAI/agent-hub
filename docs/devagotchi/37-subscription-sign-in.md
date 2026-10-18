# 37. Subscription sign-in: ChatGPT/Codex and Grok

[Index](../devagotchi-spec.md) · Decision (Jesse, 2026-10-18): users may sign in with their Grok and ChatGPT/Codex
subscriptions in both editions; this is allowed use. This file covers the official flows and how the hub and the
Shippin console use them. Provider rows: [34](34-provider-schema.md); secrets: [36](36-vaultix.md). Read 2026-10-18.

## Two ways to connect a subscription
| Method | Who holds the credential | Used for |
|---|---|---|
| **A. Official client login** | The vendor's client (Codex, Grok Build) in its own home, on the user's device or in their hosted workspace | Agent seats driven through the client ([29](29-provider-matrix.md)) |
| **B. Our app as an OAuth client** | Vaultix, user-personal scope ([36](36-vaultix.md)) | Direct API calls from the hub or console with the user's plan (`openai_api` adapter) |
Both can exist for one user. The seat's provider row says which `auth_kinds` it accepts.

## ChatGPT / Codex
**A. Codex app-server login.** `account/login/start` takes `chatgpt` (browser; returns `authUrl`),
`chatgptDeviceCode` (returns `verificationUrl` and `userCode`, then `account/login/completed` and
`account/updated` notifications), or `apiKey` ([protocol](https://github.com/openai/codex/blob/53b50197/codex-rs/app-server-protocol/src/protocol/v2/account.rs),
[device code PR](https://github.com/openai/codex/pull/15525)). `chatgptAuthTokens` is marked internal, do not use
(same file). `account/read` returns email and plan type ([app-server](https://learn.chatgpt.com/docs/app-server)).
- Local: the hub calls `account/login/start {type: chatgpt}`; the UI opens `authUrl` in the user's browser.
- Hosted workspace: use `chatgptDeviceCode`; the console shows the code and link in a modal; the user approves
  on any device. No loopback callback needs to reach the workspace.

**B. Sign in with ChatGPT (SIWC) with plan usage.**
- Identity scopes give account id, name, email, picture; plan usage is a separate scope
  `chatgpt.tokens.use.direct`, only for Plus and Pro, with a per-app weekly cap the user sets in ChatGPT
  ([quickstart](https://developers.openai.com/siwc/quickstart), [errors](https://developers.openai.com/siwc/token-sharing-open-source/errors-and-recovery)).
- Open-source flow: first sign-in uses `client_id=dynamic_agent_client`, `agent_name_hint` (our app name), and a
  persisted `ext_agent_host_id` per host; PKCE, state and nonce per attempt; save the issued `client_id` and
  reuse it; no client secret ([sign-in](https://developers.openai.com/siwc/token-sharing-open-source/sign-in),
  [overview](https://developers.openai.com/siwc/token-sharing-open-source)).
- Inference: `POST https://api.openai.com/v1/responses` with the bearer token, `store: false`, `stream: true`
  ([models](https://developers.openai.com/siwc/token-sharing-open-source/models-and-inference)). Preview limits: no
  hosted tools (file search, Code Interpreter, hosted MCP), no `previous_response_id` over HTTP, several request
  fields rejected ([limitations](https://developers.openai.com/siwc/token-sharing-open-source/preview-limitations)).
  Our tools run locally, so the hub sends function tools only.
- Codex app-server can run on a SIWC token: pass `ACCESS_TOKEN` to the child process
  ([codex-app-server](https://developers.openai.com/siwc/token-sharing-open-source/codex-app-server)).
- Remote hosts: complete OAuth where the browser is, then move the credential to the VM; the VM keeps its own
  host id and owns refreshes ([self-hosted VMs](https://developers.openai.com/siwc/token-sharing-open-source/self-hosted-vms)).
- The OpenAI docs cover open-source and locally hosted apps; paid or remotely hosted apps use an interest form
  ([overview](https://developers.openai.com/siwc/token-sharing-open-source)). Action: Shippin submits the form for
  shippin.cloud. Until it is approved, hosted users connect with method A (device code), which works today.
- Errors to handle: 429 `subscription_sharing_usage_limit_exceeded` (pause plan calls, link to ChatGPT Usage,
  do not guess a reset), 403 `..._user_not_eligible` (explain, no retry), 503 (backoff) ([errors](https://developers.openai.com/siwc/token-sharing-open-source/errors-and-recovery)).
- Console sign-in: SIWC identity scopes can also act as a "Continue with ChatGPT" login, wired as a generic OIDC
  IdP in Zitadel ([Zitadel](https://zitadel.com/docs/guides/integrate/identity-providers/generic-oidc)).

## Grok (xAI)
**A. Grok Build login.** `grok login`, or `grok login --device-auth` for headless or remote machines
([CLI](https://docs.x.ai/build/cli/reference)). Same pattern as Codex: local opens the browser; hosted workspace uses
device auth in a console modal. `grok inspect --json` and `grok models` confirm the session.

**B. xAI OAuth for our app.** Tools such as Kilo Code, OpenClaw and Hermes sign users in at `accounts.x.ai`
with browser PKCE (loopback `127.0.0.1:56121`) or device code, for SuperGrok and X Premium+ accounts, then call
`https://api.x.ai/v1` ([Kilo](https://kilo.ai/docs/ai-providers/xai), [OpenClaw](https://docs.openclaw.ai/providers/xai),
[Hermes](https://hermes-agent.nousresearch.com/docs/guides/xai-grok-oauth)). Facts to design for:
- Those tools use xAI's shared Grok client, so consent may say "Grok Build"
  ([OpenClaw](https://docs.openclaw.ai/providers/xai)); one notes xAI accepts loopback OAuth only from allowlisted
  clients ([pi](https://pi.dev/packages/pi-xai-supergrok)). We register **our own** client with xAI and do not
  reuse Grok's client id. Until it is registered, method A covers Grok seats.
- xAI rotates the refresh token on every use, so two processes refreshing one credential log each other out
  ([Kilo](https://kilo.ai/docs/ai-providers/xai)). Vaultix holds one credential with **one refresh owner**
  (a hub job with a row lock); consumers get short-lived access tokens from it.
- Some tiers get HTTP 403 on the OAuth API surface; the row then offers the API-key path
  ([Hermes](https://hermes-agent.nousresearch.com/docs/guides/xai-grok-oauth)).

## Who does what
| Edition | ChatGPT/Codex | Grok |
|---|---|---|
| Local | A: app-server browser login; B: SIWC open-source flow (host id per machine) | A: `grok login`; B once our client is registered |
| Hosted | A: app-server device code inside the user's workspace; B after the interest form | A: `grok login --device-auth` in the workspace; B once registered |
The hub never reads a client's own credential files (method A); method B tokens live only in Vaultix.

## Data
`subscription_connections(id, tenant_id, user_id, provider_id, method client_login|oauth, runner user_device|
hosted_workspace|hub, machine_id null, host_id text null (SIWC ext_agent_host_id), oauth_client_ref null,
token_ref null (Vaultix), scopes text[], plan_type null, account_label, status connected|needs_reauth|
limited|ineligible, last_check_at, version)`. Status comes from `account/read`, `grok inspect`, or token refresh
results; `limited` is set by the 429 above and feeds quota snapshots ([33](33-cost-data-and-ui.md)).

## UI (A2UI modal, React Aria)
"Connect" on a provider row opens a `Modal`: method choice (`RadioGroup`), then either "Continue with ChatGPT"
(OpenAI branding per its guidelines) / "Sign in with Grok", or the **device-code** panel: code in large
monospace with Copy, link, `ProgressBar` while polling, cancel (`account/login/cancel`). Success shows account,
plan and a per-app cap reminder with a link to ChatGPT Usage. The console lists connections per user;
admins see status, never tokens.
