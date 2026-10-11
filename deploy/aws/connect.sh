#!/usr/bin/env bash
# a machine's way in, after terraform init (README): the secrets from SSM, the node's current IP,
# kubeconfig and an ssh tunnel for kubectl. Safe to rerun, e.g. after the node's IP changed
set -euo pipefail
cd "$(dirname "$0")"

SECRETS=${SECRETS:-$HOME/.config/dsn-aws}
export AWS_REGION=${AWS_REGION:-us-east-1}

./secrets.sh pull

# the state only knows the IP from the last apply; it changes on every stop/start
ID=$(terraform output -raw instance_id)
IP=$(aws ec2 describe-instances --instance-ids "$ID" --query 'Reservations[0].Instances[0].PublicIpAddress' --output text)
if [ "$IP" = None ]; then
  echo "the node is stopped: aws ec2 start-instances --instance-ids $ID, then rerun" >&2
  exit 1
fi

# port 22 is open to one address only (ssh_cidr in terraform.tfvars)
ME=$(curl -fsS https://checkip.amazonaws.com)
if ! grep -q "\"$ME/32\"" terraform.tfvars; then
  echo "ssh is open to $(grep ssh_cidr terraform.tfvars | cut -d'"' -f2), this machine is $ME/32:" >&2
  echo "  set ssh_cidr = \"$ME/32\" in terraform.tfvars, terraform apply, FORCE=1 ./secrets.sh push" >&2
  exit 1
fi

ssh_opts=(-i dsn.pem -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile="$SECRETS/known_hosts" -o ConnectTimeout=10)
(umask 077 && ssh "${ssh_opts[@]}" ec2-user@"$IP" 'sudo cat /etc/rancher/k3s/k3s.yaml' |
  sed 's/127.0.0.1:6443/127.0.0.1:16443/; s/: default$/: dsn-aws/' >kubeconfig)
pkill -f 'ssh .*-L 16443:127.0.0.1:6443' 2>/dev/null || true
ssh "${ssh_opts[@]}" -o ExitOnForwardFailure=yes -fN -L 16443:127.0.0.1:6443 ec2-user@"$IP"

cat <<EOF
connected to $IP (app: https://$IP)
  export KUBECONFIG=$PWD/kubeconfig
  ssh -i $PWD/dsn.pem ec2-user@$IP
EOF
