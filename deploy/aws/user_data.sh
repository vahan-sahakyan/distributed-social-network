#!/bin/bash
# first boot only; k3s and everything on it come from bootstrap.sh
set -euxo pipefail
exec > >(tee -a /var/log/user-data.log) 2>&1

# ~25 pods each watching files; the AL2023 default of 128 inotify instances runs out
cat > /etc/sysctl.d/99-dsn.conf <<'SYSCTL'
fs.inotify.max_user_instances = 1024
fs.inotify.max_user_watches = 524288
SYSCTL
sysctl --system

dnf install -y -q openssl tar
echo "user-data done"
