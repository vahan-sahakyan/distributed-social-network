#!/usr/bin/env bash
# k3s, OpenBao's secrets, Argo CD and the prod root app on the node terraform made; safe to rerun.
# run after terraform apply: deploy/aws/bootstrap.sh
#   SECRETS     offline dir for the unseal key and prod values (default ~/.config/dsn-aws), made once
#   GITOPS_REF  gitops branch the bootstrap files come from (default main)
set -euo pipefail
cd "$(dirname "$0")"

K3S_VERSION=${K3S_VERSION:-v1.33.6+k3s1}
ARGOCD_VERSION=${ARGOCD_VERSION:-v3.5.3}
GITOPS_RAW=https://raw.githubusercontent.com/vahan-sahakyan/distributed-social-network-gitops/${GITOPS_REF:-main}
SECRETS=${SECRETS:-$HOME/.config/dsn-aws}

IP=$(terraform output -raw public_ip)
mkdir -p "$SECRETS" && chmod 700 "$SECRETS"
on_node() {
  ssh -i dsn.pem -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile="$SECRETS/known_hosts" \
    -o ConnectTimeout=10 ec2-user@"$IP" "$@"
}

echo "==> waiting for ssh on $IP"
for _ in $(seq 1 60); do on_node true 2>/dev/null && break; sleep 5; done
on_node 'cloud-init status --wait >/dev/null; tail -1 /var/log/user-data.log'

echo "==> offline secrets in $SECRETS"
# without the unseal key OpenBao's data can't be unsealed: keep it, never in git
[ -f "$SECRETS/openbao-unseal.key" ] || (umask 077 && openssl rand -out "$SECRETS/openbao-unseal.key" 32)
if [ ! -f "$SECRETS/prod-secrets.env" ]; then
  # the keys of the public local seed; user names kept, every password random
  (
    umask 077
    curl -fsSL "$GITOPS_RAW/envs/local/openbao-seed.env" | grep -E '^[A-Z_]+=' | while IFS='=' read -r key value; do
      case $key in *_USER | KEYCLOAK_ADMIN) echo "$key=$value" ;; *) echo "$key=$(openssl rand -hex 24)" ;; esac
    done >"$SECRETS/prod-secrets.env"
  )
fi

echo "==> k3s $K3S_VERSION"
on_node K3S_VERSION="$K3S_VERSION" bash -s <<'REMOTE'
set -euo pipefail
if ! k3s --version 2>/dev/null | grep -qF "${K3S_VERSION}"; then
  # pinned: the channel lookup (update.k3s.io) isn't reliably reachable from EC2. Traefik and
  # servicelb stay on: no cloud controller runs here, so servicelb binds Traefik to host 80/443
  curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="$K3S_VERSION" INSTALL_K3S_SKIP_SELINUX_RPM=true sh -
fi
# the API answers before the node has registered, and wait --all needs one to exist
for _ in $(seq 1 60); do sudo k3s kubectl get nodes -o name 2>/dev/null | grep -q . && break; sleep 5; done
sudo k3s kubectl wait node --all --for=condition=Ready --timeout=300s
REMOTE

echo "==> dsn-tls: self-signed certificate for the node's current IP, renewed at boot"
on_node 'sudo tee /usr/local/bin/dsn-tls >/dev/null' <<'SCRIPT'
#!/bin/bash
# writes dsn/dsn-tls for the node's public IP unless it already covers it; the IP changes on stop/start
set -euo pipefail
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
token=$(curl -sf -X PUT http://169.254.169.254/latest/api/token -H 'X-aws-ec2-metadata-token-ttl-seconds: 60')
ip=$(curl -sf -H "X-aws-ec2-metadata-token: $token" http://169.254.169.254/latest/meta-data/public-ipv4)
# Argo CD creates the namespace on its first sync
until kubectl get namespace dsn >/dev/null 2>&1; do sleep 10; done
# browsers send no SNI to an IP, so this must be Traefik's default certificate too
kubectl apply --server-side --field-manager=dsn-tls -f - <<'YAML'
apiVersion: traefik.io/v1alpha1
kind: TLSStore
metadata:
  name: default
  namespace: dsn
spec:
  defaultCertificate:
    secretName: dsn-tls
YAML
[ "$(kubectl -n dsn get secret dsn-tls -o jsonpath='{.metadata.annotations.dsn/public-ip}' 2>/dev/null)" = "$ip" ] && exit 0
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
openssl req -x509 -nodes -newkey rsa:2048 -days 825 -keyout "$tmp/tls.key" -out "$tmp/tls.crt" \
  -subj "/CN=$ip" -addext "subjectAltName=IP:$ip"
kubectl -n dsn create secret tls dsn-tls --cert="$tmp/tls.crt" --key="$tmp/tls.key" --dry-run=client -o yaml \
  | kubectl annotate --local -f - dsn/public-ip="$ip" -o yaml \
  | kubectl apply --server-side --field-manager=dsn-tls --force-conflicts -f -
echo "dsn-tls for $ip"
SCRIPT
on_node 'sudo tee /etc/systemd/system/dsn-tls.service >/dev/null' <<'UNIT'
[Unit]
Description=Self-signed dsn-tls certificate for the current public IP
After=k3s.service
Wants=k3s.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/dsn-tls
TimeoutStartSec=1h

[Install]
WantedBy=multi-user.target
UNIT
on_node 'sudo chmod 755 /usr/local/bin/dsn-tls && sudo systemctl daemon-reload && sudo systemctl enable --now --no-block dsn-tls'

echo "==> OpenBao unseal key and first-start values"
on_node 'sudo k3s kubectl create namespace openbao --dry-run=client -o yaml | sudo k3s kubectl apply -f - >/dev/null'
# only on a fresh cluster: the unseal key must never change under existing data
if ! on_node 'sudo k3s kubectl -n openbao get secret openbao-unseal >/dev/null 2>&1'; then
  on_node 'sudo k3s kubectl -n openbao create secret generic openbao-unseal --from-file=key=/dev/stdin' <"$SECRETS/openbao-unseal.key"
  on_node 'sudo k3s kubectl -n openbao create secret generic openbao-seed --from-env-file=/dev/stdin' <"$SECRETS/prod-secrets.env"
fi

echo "==> Argo CD $ARGOCD_VERSION and the root app"
on_node ARGOCD_VERSION="$ARGOCD_VERSION" GITOPS_RAW="$GITOPS_RAW" bash -s <<'REMOTE'
set -euo pipefail
k() { sudo k3s kubectl "$@"; }
k create namespace argocd --dry-run=client -o yaml | k apply -f - >/dev/null
k apply -n argocd --server-side --force-conflicts -f "https://raw.githubusercontent.com/argoproj/argo-cd/$ARGOCD_VERSION/manifests/install.yaml" >/dev/null
k wait --for=condition=Established crd/applications.argoproj.io --timeout=60s
k apply --server-side --field-manager=dsn-bootstrap -f "$GITOPS_RAW/bootstrap/argocd-config.yaml"
k -n argocd rollout restart statefulset/argocd-application-controller
k -n argocd rollout status deploy/argocd-repo-server --timeout=600s
k apply -f "$GITOPS_RAW/bootstrap/root.yaml"
REMOTE

echo "==> kubeconfig (through an ssh tunnel, the API isn't public)"
(umask 077 && on_node 'sudo cat /etc/rancher/k3s/k3s.yaml' | sed 's/127.0.0.1:6443/127.0.0.1:16443/; s/: default$/: dsn-aws/' >kubeconfig)

echo "==> waiting for OpenBao's first start, then dropping its seed"
on_node bash -s <<'REMOTE'
set -euo pipefail
k() { sudo k3s kubectl "$@"; }
k -n openbao get secret openbao-seed >/dev/null 2>&1 || exit 0
for _ in $(seq 1 120); do k -n openbao get pod openbao-0 >/dev/null 2>&1 && break; sleep 10; done
k -n openbao wait pod/openbao-0 --for=condition=Ready --timeout=1200s
k -n openbao delete secret openbao-seed
REMOTE

cat <<EOF

Argo CD is syncing; the app comes up on https://$IP (self-signed certificate) in ~15 minutes.
  kubectl:  ssh -i dsn.pem -fN -L 16443:127.0.0.1:6443 ec2-user@$IP && KUBECONFIG=kubeconfig kubectl get applications -n argocd
  Argo UI:  KUBECONFIG=kubeconfig kubectl -n argocd port-forward svc/argocd-server 9443:443
Offline, keep safe: $SECRETS/openbao-unseal.key, $SECRETS/prod-secrets.env
EOF
