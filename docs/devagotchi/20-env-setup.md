# 20. Turnkey environment setup (WSL, Docker, Podman) driven by data

[Index](../devagotchi-spec.md) · Extends guides ([12](12-guides.md)) and Docker setup ([13](13-docker-setup.md)).
Sources read 2026-10-18. **Inference** marks our design.

## What this touches
The chosen enrolled machine only: Windows optional features and the WSL package, one WSL distro, Docker Desktop or
Engine or Podman and its VM. Elevated steps stay with the operator (13). The hub stores the plan and the run log.

## Facts the guides rely on

| Need | Fact | Source |
|---|---|---|
| WSL without a distro | `wsl --install --no-distribution` | [WSL basic commands](https://learn.microsoft.com/en-us/windows/wsl/basic-commands) |
| Store-free install | `wsl --install --web-download` | same |
| Distro choice | `wsl --list --online`, then `wsl --install -d <Distro>` | same, [install](https://learn.microsoft.com/en-us/windows/wsl/install) |
| Admin + restart | `wsl --install` runs in an administrator PowerShell and may need a restart | [install](https://learn.microsoft.com/en-us/windows/wsl/install) |
| Docker Desktop, unattended, per-user | `"Docker Desktop Installer.exe" install --user --quiet --accept-license --backend=wsl-2`; installs to `%LOCALAPPDATA%\Programs\DockerDesktop` without admin | [Windows install](https://docs.docker.com/desktop/setup/install/windows-install/) |
| Docker Desktop licence | Paid plan for orgs >250 employees or >$10M revenue | [licence](https://docs.docker.com/subscription/desktop-license/) |
| Podman on macOS/Windows | `podman machine init`, `podman machine start`; WSL is the default Windows provider | [machine init](https://docs.podman.io/en/stable/markdown/podman-machine-init.1.html), [Windows tutorial](https://github.com/containers/podman/blob/main/docs/tutorials/podman-for-windows.md) |
| Rootless Docker | needs `uidmap` and ≥65,536 subordinate IDs | [rootless](https://docs.docker.com/engine/security/rootless/) |
| Desired state as data (precedent) | WinGet Configuration: declarative YAML, `winget configure test` checks, `configure` applies only differences | [WinGet configuration](https://learn.microsoft.com/en-us/windows/package-manager/configuration/), [configure command](https://learn.microsoft.com/en-us/windows/package-manager/winget/configure) |

Note: `--accept-license` accepts Docker's terms on the operator's behalf. Our rule (13) is that the AI never accepts
a licence, so that flag is only used when the operator ticks the licence `confirm` step themselves (**inference**).

## Model: every setup item is test → plan → apply → verify
A setup **component** is a data row, shaped like a WinGet/DSC resource (**inference**: same idea, our own format):

```json
{ "schema": 1, "id": "wsl-distro", "applies_to": {"os": ["windows"]},
  "depends_on": ["wsl-platform"],
  "test":   {"exec": ["wsl", "--list", "--quiet"], "expect": {"stdout_contains": "${distro}"}},
  "apply":  {"actor": "operator", "elevated": true, "text_ref": "wsl.install_distro",
             "command": ["wsl", "--install", "-d", "${distro}", "--web-download"]},
  "verify": {"exec": ["wsl", "-d", "${distro}", "--", "uname", "-a"], "expect": {"exit": 0}},
  "params": {"distro": {"type": "select", "source": "exec:wsl --list --online", "default": "Ubuntu"}},
  "restart": "maybe", "risks": ["vm_ram_disk"] }
```

- **Idempotent by design**: `apply` runs only when `test` fails; re-running a profile is safe and only fills gaps.
- **Detection is read-only**: `test` steps run under the read-only `exec` grant with argv prefixes (no shell).
- **Actor** is `agent` (bridge does it under a grant), `operator` (elevated, licence, restart, UI clicks), or
  `either` (operator chooses). `elevated: true` always forces `operator`.
- **Restart-aware**: a step with `restart` stores a resume marker in the guide run; after the machine reconnects,
  the run continues at the next `test`.
- **Params** come from the UI (Select, TextField) and may be filled from a detection command; never free text into
  argv without the param's type check.

## Profiles (data)
A **profile** is an ordered list of components with parameters, e.g. `windows-docker-desktop` = `virtualization-on`
→ `wsl-platform` → `wsl-distro(Ubuntu)` → `docker-desktop(wsl-2)` → `docker-verify`; `mac-podman` =
`podman-cli(brew)` → `podman-machine` → `podman-verify`; `linux-rootless-docker` = `uidmap` (operator) →
`subuid` check → `dockerd-rootless-setup` → `docker-verify`. Profiles are seeded presets, edited in the Guide editor
(07), versioned and signed like guides (D21).

## UI (React Aria Components)
- **Setup planner**: pick machine → profile (Select) → the planner runs every `test` and shows a `GridList` of
  components with state `ok | missing | needs operator | blocked by licence | unknown`.
- **Plan review**: a `Dialog` lists what will change, who does each step, risks, and restarts; **Start** creates one
  access request for all agent steps (one grant, `task` duration).
- **Run**: the existing Guide runner panel (07) drives it; operator steps show copy buttons and per-OS text.
- **Drift check**: "Check again" re-runs every `test` without applying, for a later audit.

## Failure handling
- Each `apply` has a timeout from data and reports honestly (no silent cap, see [25](25-dc-lessons.md)).
- Known failure signatures (data: regex on stderr → hint id), e.g. virtualization off, WSL kernel missing,
  Docker Desktop needing WSL ≥2.1.5 ([13](13-docker-setup.md)).
- After `max_retries` the run pauses and offers **Report a problem** (12).
