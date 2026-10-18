# 13. Docker setup (guided, per OS)
[Index](../devagotchi-spec.md) · Guided install and setup of a container runtime on an enrolled machine, driven by the `docker-setup` and `podman-setup` guides ([12](12-guides.md)) and done through the bridge under grants. Facts are from vendor docs read 2026-10-17 and change often, so every number below lives in guide data, not code. **Inference** marks our choices.

## What this touches
The chosen machine only: the container runtime package or app, its VM/WSL backend, and the operator's shell config if they choose `DOCKER_HOST`. Agent steps run under the `docker-setup` grant preset; anything elevated, any installer UI with a licence, and any OS restart is an **operator step**. Nothing touches other machines or the hub host.

## Runtime presets (data rows, the guide branches on them)
| Preset | OS | What it is | Key facts (source) |
|---|---|---|---|
| `docker-desktop-windows` | Windows | Docker Desktop on the **WSL 2 backend** | Per-user install is recommended and needs no admin; needs WSL 2.1.5+; Windows 10 22H2 (19045) or Windows 11 23H2 (22631)+; virtualization on ([install](https://docs.docker.com/desktop/setup/install/windows-install/), [WSL backend](https://docs.docker.com/desktop/features/wsl/)). WSL itself is installed with `wsl --install` from an **administrator** PowerShell, then a restart ([WSL](https://learn.microsoft.com/en-us/windows/wsl/install)) |
| `docker-desktop-macos` | macOS | Docker Desktop (Apple silicon or Intel build) | Supported on the current and two previous macOS majors; ≥4 GB RAM; Rosetta 2 optional for some amd64 tools ([install](https://docs.docker.com/desktop/setup/install/mac-install/)) |
| `docker-engine-linux` | Linux | Docker Engine from Docker's apt/dnf repository | Install via the official repository; published ports bypass ufw/firewalld rules ([Ubuntu](https://docs.docker.com/engine/install/ubuntu/)); the `docker` group "grants root-level privileges" ([post-install](https://docs.docker.com/engine/install/linux-postinstall/)) |
| `docker-rootless-linux` | Linux | Engine in rootless mode | Daemon and containers run as the user; needs `uidmap` and ≥65,536 subordinate UIDs/GIDs ([rootless](https://docs.docker.com/engine/security/rootless/)) |
| `docker-desktop-linux` | Linux | Docker Desktop for Linux (a VM) | Needs KVM virtualization support ([install](https://docs.docker.com/desktop/setup/install/linux/)) |
| `podman` | all | Podman; on macOS/Windows a **Podman machine** VM | Rootless by default; `podman machine init` / `start`; on Windows the default provider is WSL, Hyper-V optional; listens for Docker API clients ([install](https://podman.io/docs/installation), [machine init](https://docs.podman.io/en/stable/markdown/podman-machine-init.1.html), [Windows](https://github.com/containers/podman/blob/main/docs/tutorials/podman-for-windows.md)); `DOCKER_HOST` points Docker tools at it ([Podman Desktop](https://podman-desktop.io/docs/migrating-from-docker/using-the-docker_host-environment-variable)) |

**Licence**: commercial use of Docker Desktop in organisations with more than 250 employees **or** more than $10 million annual revenue needs a paid subscription ([licence](https://docs.docker.com/subscription/desktop-license/)). The guide asks this as a `confirm` step and the operator answers; the AI never decides it. Docker Engine and Podman have no such step.

## Default choice (inference)
- Windows: `docker-desktop-windows` (WSL 2), with `podman` offered when the licence answer is "needs a paid plan" or the operator prefers it.
- macOS: `docker-desktop-macos`, with `podman` as the alternative on the same terms.
- Linux: `docker-rootless-linux` first (avoids root-equivalent access, T24), then `docker-engine-linux` if the operator needs rootful features, then `podman`.
The defaults are data (`docker_setup_default_preset` per OS) and editable.

## Guide outline (`docker-setup`, branches per OS)
1. `explain`: what a container runtime is for here, what it can access, and the risks of the chosen preset (rows: "root-equivalent via docker group", "ports bypass firewall", "VM uses RAM and disk").
2. `check`: is a runtime already present (`docker version`, `podman version` under a read-only `exec` grant; argv prefix only, no shell)? If yes, jump to step 8.
3. `branch`: OS, arch, and operator choice of preset (default above), plus the licence `confirm` for Docker Desktop.
4. `request_access`: preset `docker-setup` for this machine: `exec` with argv prefixes for the runtime and its installer only (`docker`, `podman`, `wsl --status`, `winget`/`brew` install of the named package if the operator chose a package manager), `net.egress` to the vendor hosts listed in the guide (data), `fs.read` of the download dir. Duration `task`. No `unsandboxed`.
5. Install, per OS:
   - Windows: `check` `wsl --status`; if WSL is missing, `operator_action`: "Open PowerShell as administrator, run `wsl --install`, restart." After restart, `check` again. Then `operator_action` to run the Docker Desktop installer (per-user mode) and accept its terms, or `agent_action` `winget install` if the operator chose winget and the grant allows it. On first start the operator confirms Docker's agreement screen (`confirm`; with a `gui.*` grant the AI may point at it on screen but does not click Accept).
   - macOS: `operator_action` to open the installer (or `agent_action` `brew install --cask` if chosen and granted), drag to Applications, start it, and approve its privileged helper prompt. Podman: `agent_action` `podman machine init` then `podman machine start`.
   - Linux Engine: `operator_action` with the exact repository + `apt`/`dnf` commands from the guide data, run with `sudo` by the operator in their own terminal (the bridge never holds sudo). Rootless: `operator_action` for `uidmap` (needs sudo), then `agent_action` `dockerd-rootless-setuptool.sh install` as the user.
6. `operator_action` (optional): set `DOCKER_HOST` for Podman, or add the user to `docker`, each shown with its risk row and skipped by default.
7. Firewall note (Linux, Engine): `explain` the ufw/firewalld bypass and suggest publishing ports on `127.0.0.1`.
8. `verify`: `docker run --rm hello-world` (or `podman run --rm quay.io/podman/hello`) under the grant; record versions in the run log.
9. `explain`: what was installed, how to stop or uninstall it, and that the setup grant ends with the task.

## Why the bridge does not install as admin
The bridge runs as the operator's user (or the low-privilege sandbox user for exec) and never elevates (02). Elevated steps stay with the operator so a UAC or sudo prompt is always a human decision, matching T21. **Inference**: with the unsandboxed policy enabled the AI can do more steps itself, but still not elevated ones.

## WSL and Docker
On Windows, Docker Desktop's WSL 2 backend runs its own distro. If the operator also enrolled a WSL distro as a machine (D2), the guide can enable Docker Desktop's WSL integration for it; that distro's bridge then uses the same daemon. **Inference**: we do not install a second Docker Engine inside an enrolled WSL distro by default, to avoid two daemons.
