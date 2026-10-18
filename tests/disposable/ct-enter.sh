#!/usr/bin/env bash
# Run a command inside a disposable Debian rootfs on the box only: private mount, pid, uts and ipc namespaces,
# a private /dev, /run and /tmp, and pivot_root into ROOT (container storage needs a real mount at /). Nothing is mounted into the box's own namespace; deleting ROOT removes it all.
# usage: ct-enter.sh ROOT cmd...    (needs sudo on the box; touches only ROOT)
set -euo pipefail
R=$(cd "$1" && pwd); shift
exec sudo unshare --mount --pid --uts --ipc --fork --propagation private bash -c '
set -e; R=$0
mount --bind "$R" "$R"
mount -t proc proc "$R/proc"; mount --rbind /sys "$R/sys"; mount --make-rslave "$R/sys"
mount -t tmpfs -o mode=755 tmpfs "$R/dev"
for d in null zero full random urandom tty; do touch "$R/dev/$d"; mount --bind "/dev/$d" "$R/dev/$d"; done
mkdir -p "$R/dev/pts" "$R/dev/shm" "$R/dev/net"; mount -t devpts devpts "$R/dev/pts" -o newinstance,ptmxmode=0666
ln -sf pts/ptmx "$R/dev/ptmx"; mount -t tmpfs tmpfs "$R/dev/shm"
if [ -e /dev/net/tun ]; then touch "$R/dev/net/tun"; mount --bind /dev/net/tun "$R/dev/net/tun"; fi
if [ -e /dev/fuse ]; then touch "$R/dev/fuse"; mount --bind /dev/fuse "$R/dev/fuse"; fi
mount -t tmpfs tmpfs "$R/run"; mount -t tmpfs -o mode=1777 tmpfs "$R/tmp"
hostname disposable
cd "$R"; mkdir -p .oldroot; pivot_root . .oldroot; umount -l /.oldroot; rmdir /.oldroot 2>/dev/null || true
exec chroot / "$@"' "$R" "$@"
