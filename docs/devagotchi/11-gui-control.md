# 11. Desktop GUI control (in scope from v0.1)
[Index](../devagotchi-spec.md) · Screen capture, mouse and keyboard, and window/app control on Windows, macOS, and Linux (X11 and Wayland), run by the GUI helper ([02](02-bridge.md#gui-helper-desktop-control-in-scope-for-v01)) under `gui.*` grants (D7, resolved: in scope from the start). Researched 2026-10-17; sources inline. **Inference** = our design choice; **unverified** = not confirmed from a primary source.

## What this touches
Only the enrolled machine's interactive desktop session of the operator who installed the helper: its displays, its input queue, its accessibility tree, and its windows. It does not touch other users' sessions, elevated/secure desktops, or the exec sandbox user. Nothing is installed until the operator runs the `enable-gui-<os>` guide.

## Per-OS APIs and permissions
| OS | Capture | Input | Tree / window / app | OS permission gate |
|---|---|---|---|---|
| macOS | ScreenCaptureKit ([docs](https://developer.apple.com/documentation/screencapturekit)); check with `CGPreflightScreenCaptureAccess`, ask with `CGRequestScreenCaptureAccess` ([docs](https://developer.apple.com/documentation/coregraphics/cgpreflightscreencaptureaccess%28%29)) | `CGEvent` posting, which needs Accessibility trust | AX API (`AXUIElement`), trust checked with `AXIsProcessTrustedWithOptions` ([docs](https://developer.apple.com/documentation/applicationservices/1459186-axisprocesstrustedwithoptions)); `NSWorkspace` to launch/quit apps | TCC: **Screen Recording** and **Accessibility**, approved by the operator in System Settings → Privacy & Security |
| Windows | Windows.Graphics.Capture: system picker or programmatic item; a yellow border shows while capturing, and borderless capture needs user consent ([capture](https://learn.microsoft.com/en-us/windows/uwp/audio-video-camera/screen-capture), [IsBorderRequired](https://learn.microsoft.com/en-us/uwp/api/windows.graphics.capture.graphicscapturesession.isborderrequired)) | `SendInput`, subject to UIPI: only into equal or lower integrity processes, and a block is not reported in the return value ([SendInput](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-sendinput)) | UI Automation for the element tree and patterns ([UIA](https://learn.microsoft.com/en-us/windows/win32/winauto/entry-uiauto-win32)); Win32 window APIs; prior art Windows-MCP uses UIA tree + screenshot ([repo](https://github.com/CursorTouch/Windows-MCP)) | No consent prompt for same-integrity apps. Higher-integrity UI needs `uiAccess` (signed, installed in Program Files) ([UIAccess](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-securityoverview), [policy](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-10/security/threat-protection/security-policy-settings/user-account-control-only-elevate-uiaccess-applications-that-are-installed-in-secure-locations)); **we never request it** (01 invariant 8) |
| Linux Wayland | xdg-desktop-portal **ScreenCast** (PipeWire streams) ([docs](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.ScreenCast.html)) | Portal **RemoteDesktop**, with `ConnectToEIS` (libei) recommended over the D-Bus Notify methods ([docs](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.RemoteDesktop.html), [libei](https://gitlab.freedesktop.org/libinput/libei)) | AT-SPI2 over D-Bus for the tree ([AT-SPI2](https://www.freedesktop.org/wiki/Accessibility/AT-SPI2/)); no generic portal for moving other apps' windows (**unverified**: compositor-specific) | Portal dialog asks the operator. `persist_mode=2` returns a single-use `restore_token` to skip the dialog next time |
| Linux X11 | `XGetImage`/MIT-SHM | XTEST extension ([spec](https://www.x.org/releases/X11R7.7/doc/xextproto/xtest.html)) | AT-SPI2; EWMH window hints | **None**: any X client can capture and inject. Our grant is the only gate (T22) |

### Platform facts that shape the design
- **macOS Screen Recording needs a relaunch** after first grant before capture works in that process; Accessibility takes effect live (community scaffold, [README](https://github.com/xueshiqiao/macos-app-scaffold/blob/main/skills/macos-app-scaffold-new/templates/screen-recording/README.md); **unverified** by Apple docs). The guide has a "Relaunch helper" step and `gui_ready` shows `needs_relaunch`.
- **macOS 15+ re-asks** the operator periodically to keep Screen Recording (reported monthly; [TidBITS](https://tidbits.com/2024/09/23/how-to-avoid-sequoias-repetitive-screen-recording-permissions-prompts/)). We do not suppress it (**inference**: it is a useful second reminder).
- **MDM cannot silently grant Screen Recording**; a PPPC profile can only let a standard user approve it (vendor docs: [Addigy](https://support.addigy.com/hc/en-us/articles/4403549601043-Privacy-Preferences-Policy-Control-PPPC-for-Standard-Users), [Breeze](https://docs.breezermm.com/agents/macos-permissions/)). Managing Accessibility via PPPC is reported deprecated in macOS 26.2 ([wiki](https://github.com/gilburns/Intune-PPPC-Utility/wiki/Service-Types); **unverified**). So the guide always has the operator approve both in System Settings.
- **macOS responsible process**: a helper launched by launchd may fail TCC checks on macOS 26 (report: [RustDesk #15364](https://github.com/rustdesk/rustdesk/discussions/15364); **unverified**). The helper ships as a signed `.app` login item and phase 4 tests both launch paths.
- **Windows service isolation**: services run in Session 0 and cannot use the desktop ([Interactive Services](https://learn.microsoft.com/en-us/windows/win32/services/interactive-services)), hence the per-user helper.
- **Wayland restore tokens** are single-use; each `Start` returns a new one to store ([RemoteDesktop](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.RemoteDesktop.html)). wlroots' portal backend implements ScreenCast and Screenshot only ([xdg-desktop-portal-wlr](https://github.com/emersion/xdg-desktop-portal-wlr)), so on such compositors input is unavailable and `gui_ready.input = unsupported`.

## Grant scopes
| Capability | Allows | Selector (data, validated) | Default risk |
|---|---|---|---|
| `gui.observe` | Screenshots and the accessibility tree | `monitors` (ids or `all`), `windows` (app id / exe / bundle id list, or `any`), `max_edge` px, `tree_depth` | High: "sees everything on the chosen screens, including passwords and messages" |
| `gui.input` | Mouse and keyboard | `apps` allowlist (required; `any` only via unsandboxed policy), `actions` (`click`, `type`, `key`, `scroll`, `drag`), `deny_keys` (e.g. password fields when the tree marks them secure), `max_actions_per_minute` | High: "acts as you in those apps; it is not sandboxed" |
| `gui.window` | List, focus, move, resize, minimise windows | `apps` allowlist | Medium |
| `gui.app` | Launch and quit apps | `apps` allowlist with argv for each | High: "a launched app can open others" |
Preset `gui-assist`: `gui.observe` (one monitor, any window) + `gui.input` and `gui.window` for the apps the agent names, duration `task`. **Inference**: `gui.input` without `gui.observe` is refused, since blind input is riskier.

## Ops
`gui_session_start` takes a lease (fence, `gui_session_ttl_seconds`); one active session per machine. Every `gui_input` action runs these checks in order, failing closed (**inference**):
1. Hub: live grants for the capability; fence current; rate under `gui_max_actions_per_minute`.
2. Helper: the desktop is unlocked and not a secure desktop (else `gui_secure_desktop`); the foreground window's process matches the `apps` selector (else `gui_target_not_allowed`); the target element is not a secure text field when `deny_keys` covers it.
3. Helper: performs the action, then re-reads focus/tree to confirm it landed (UIPI drops input silently on Windows).
Coordinates: screenshots are scaled to `max_edge` and the helper maps coordinates back, as Anthropic's tool expects ([computer use](https://platform.claude.com/docs/en/agents-and-tools/tool-use/computer-use-tool)).

## Operator visibility and stop
- While a GUI session is live the helper shows an always-on-top overlay "Devagotchi agent is controlling: <apps>" with **Stop**, plus a stop hotkey (`gui_stop_hotkey`). Stop ends the session and pauses GUI grants on that machine until the operator resumes in the UI (**inference**; on Windows the capture border also shows).
- Physical input by the operator pauses agent input for 3 s (**inference**: avoids fighting over the pointer).
- Consequential actions (send, buy, delete, accept terms, change settings) are steps the agent must hand to the operator or confirm through a card, per Anthropic's precautions ([computer use](https://platform.claude.com/docs/en/agents-and-tools/tool-use/computer-use-tool)).

## Audit
- Events per action: `gui.action` with action kind, target app id and window title fingerprint, element role, coordinates bucketed to 10 px, result, and `grant_id`. **Typed text is never logged**; only its length and whether the field was marked secure.
- Per screenshot: `gui.screenshot` with `sha256`, size, monitor, and the `gui_artifacts` id. Session start/end/stop events carry who and why.

## Screenshot retention (D15, proposed default `none`)
| Setting | Stored | Use |
|---|---|---|
| `none` (default) | Hash, size, time only | The image goes to the calling agent and is gone. Audit proves *that* a capture happened, not what it showed |
| `thumbnail_redacted` | 320 px thumbnail, secure fields and detected text blurred, encrypted, expires after `gui_retention_days` | Operator review in the GUI session screen |
| `full_encrypted` | Full image encrypted with the tenant key, expires after `gui_retention_days`, viewable by humans with `gui.view` only | Debugging and incident review |
**Inference**: `none` matches the "never print secrets" rule because a screenshot can contain any secret on screen (T20); keeping images trades that for reviewability, which is why it is the operator's choice.
