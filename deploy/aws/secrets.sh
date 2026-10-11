#!/usr/bin/env bash
# the node's secrets as SSM SecureString parameters (aws/ssm key), so any machine with the account's
# credentials has them. Replaces nothing unless asked: a value that differs on both sides stops it.
#   secrets.sh push   create the parameters that are missing, from local files
#   secrets.sh pull   write the files that are missing, from the parameters
#   FORCE=1           replace what differs instead (push: SSM from the files, pull: the files from SSM).
#                     Never for the unseal key once OpenBao has data: only the old key opens it
#   SECRETS           local dir for the OpenBao files (default ~/.config/dsn-aws)
set -euo pipefail
cd "$(dirname "$0")"

SECRETS=${SECRETS:-$HOME/.config/dsn-aws}
export AWS_REGION=${AWS_REGION:-us-east-1}
mkdir -p "$SECRETS" && chmod 700 "$SECRETS"
umask 077

# parameter, local file, encoding (the unseal key is binary)
PARAMS=(
  "/dsn/ssh-key dsn.pem text"
  "/dsn/tfvars terraform.tfvars text"
  "/dsn/prod-secrets $SECRETS/prod-secrets.env text"
  "/dsn/openbao-unseal-key $SECRETS/openbao-unseal.key base64"
)

# reads the file in its parameter form, on stdout
encoded() { if [ "$2" = base64 ]; then base64 <"$1" | tr -d '\n'; else cat "$1"; fi; }

# prints the parameter's value, or nothing (exit 1) when it doesn't exist
remote() {
  aws ssm get-parameter --name "$1" --with-decryption --output json 2>/dev/null |
    python3 -c 'import json,sys; sys.stdout.write(json.load(sys.stdin)["Parameter"]["Value"])' 2>/dev/null
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

push() {
  local name file enc
  for p in "${PARAMS[@]}"; do
    read -r name file enc <<<"$p"
    [ -f "$file" ] || { echo "skip  $name: no $file"; continue; }
    encoded "$file" "$enc" >"$tmp/local"
    if remote "$name" >"$tmp/remote"; then
      cmp -s "$tmp/local" "$tmp/remote" && echo "same  $name" && continue
      [ "${FORCE:-}" = 1 ] || { echo "differs $name: SSM and $file disagree, nothing changed (FORCE=1 replaces SSM)" >&2; exit 1; }
      # file:// keeps the value out of the process list
      aws ssm put-parameter --name "$name" --type SecureString --value "file://$tmp/local" --overwrite >/dev/null
      echo "replace $name"
      continue
    fi
    aws ssm put-parameter --name "$name" --type SecureString --value "file://$tmp/local" \
      --tags Key=Project,Value=dsn >/dev/null
    echo "push  $name"
  done
}

pull() {
  local name file enc
  for p in "${PARAMS[@]}"; do
    read -r name file enc <<<"$p"
    remote "$name" >"$tmp/remote" || { echo "skip  $name: not in SSM"; continue; }
    if [ "$enc" = base64 ]; then base64 -d <"$tmp/remote" >"$tmp/value"; else cp "$tmp/remote" "$tmp/value"; fi
    if [ -f "$file" ]; then
      cmp -s "$tmp/value" "$file" && echo "same  $name" && continue
      [ "${FORCE:-}" = 1 ] || { echo "differs $name: SSM and $file disagree, nothing changed (FORCE=1 replaces the file)" >&2; exit 1; }
    fi
    cp "$tmp/value" "$file" && chmod 600 "$file"
    echo "pull  $name -> $file"
  done
}

case ${1:-} in
  push) push ;;
  pull) pull ;;
  *) echo "usage: $0 push|pull" >&2 && exit 2 ;;
esac
