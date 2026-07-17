#!/usr/bin/env bash
# Rewrites the COPY pkg/ block in each service Dockerfile based on actual Go dependencies.
# Usage: ./scripts/gen-dockerfiles.sh
#        or via: make dockerfiles
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MODULE_PREFIX="github.com/vahan-sahakyan/distributed-social-network/pkg/"

for svc_dir in "$REPO_ROOT"/services/*/; do
  svc=$(basename "$svc_dir")
  dockerfile="$svc_dir/Dockerfile"

  if [[ ! -f "$dockerfile" ]]; then
    continue
  fi

  # collect pkg/* import paths used by this service, sorted
  pkg_deps=$(
    cd "$svc_dir" && go list -deps ./... 2>/dev/null \
      | grep "^${MODULE_PREFIX}" \
      | sed "s|${MODULE_PREFIX}|pkg/|" \
      | sort -u
  )

  if [[ -z "$pkg_deps" ]]; then
    echo "  $svc: no pkg/ deps found, skipping"
    continue
  fi

  # build replacement COPY block
  new_block="COPY pkg/go.mod pkg/go.sum ./pkg/"
  while IFS= read -r dep; do
    new_block+=$'\n'"COPY ${dep}/ ./${dep}/"
  done <<< "$pkg_deps"

  # rewrite Dockerfile: keep lines before the pkg block, inject new block, keep
  # lines from "COPY services/" onward
  {
    awk '/^COPY pkg\//{exit} {print}' "$dockerfile"
    printf '%s\n' "$new_block"
    awk '/^COPY services\//{found=1} found{print}' "$dockerfile"
  } > "${dockerfile}.tmp" && mv "${dockerfile}.tmp" "$dockerfile"

  echo "  updated $svc"
done

echo "done."
