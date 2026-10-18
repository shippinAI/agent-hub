# Devagotchi bridge bundle: @ID@

**Not installed. Installing this on @ID@ needs Jesse's explicit approval.**

## What it does
One command on the host: `@ONE@`
1. Picks the binary for the CPU (amd64 or arm64) and checks it against `bin/SHA256SUMS`.
2. Asks for the **enrollment code**, hidden. It is never a flag and never echoed or logged.
3. Enrolls with the hub in `host.env` (`HUB_URL` must be the hub's tailnet URL; loopback is refused).
4. Installs the background service (systemd user unit on Linux, DevagotchiBridge service on Windows)
   and starts it. The bridge only dials out to the hub; it opens no listening port.

## Enrollment-code flow
1. Jesse approves this host.
2. In the hub UI: Machines > Add machine > id `@ID@`. The code is shown once (masked, with a copy button)
   and is single-use and short-lived.
3. On @ID@, run the one command above and paste the code at the prompt.
4. The hub shows `@ID@` Online after its first heartbeat. Grants are still per agent, per call,
   approved by a human, and every call passes the policy gate (fails closed).

## Undo
Linux: `systemctl --user disable --now devagotchi-bridge; rm ~/.local/bin/devagotchi-bridge`, then revoke
the machine in the hub. Windows: `sc.exe stop DevagotchiBridge; sc.exe delete DevagotchiBridge`, then
remove `%ProgramFiles%\Devagotchi` and `%ProgramData%\Devagotchi`, and revoke the machine in the hub.
