#!/usr/bin/env bash
# Where the scripts point: the compose stack, or the local k3d cluster with CLUSTER=1
# (make demo CLUSTER=1). Any URL can still be overridden on its own.

if [[ "${CLUSTER:-}" == 1 ]]; then
  : "${GATEWAY_URL:=https://localhost:8443}"
  : "${KEYCLOAK_URL:=https://localhost:8443/auth}"
  : "${PROMETHEUS_URL:=https://prometheus.localhost:8443}"
  : "${GRAFANA_URL:=https://grafana.localhost:8443}"
  : "${JAEGER_URL:=https://jaeger.localhost:8443}"
  : "${REDPANDA_CONSOLE_URL:=https://redpanda.localhost:8443}"
  : "${MINIO_CONSOLE_URL:=https://minio.localhost:8443}"
  # the cluster's certificates come from the machine's local CA (make cluster-up)
  [[ -f "$HOME/.config/dsn/local-ca.crt" ]] && export CURL_CA_BUNDLE="$HOME/.config/dsn/local-ca.crt"
  # in_infra <service> <cmd...>: runs a command in an infra container
  in_infra() { local svc=$1; shift; kubectl -n dsn exec "sts/$svc" -- "$@"; }
else
  : "${GATEWAY_URL:=http://localhost:8080}"
  : "${KEYCLOAK_URL:=http://localhost:8180/auth}"
  : "${PROMETHEUS_URL:=http://localhost:9090}"
  : "${GRAFANA_URL:=http://localhost:3000}"
  : "${JAEGER_URL:=http://localhost:16686}"
  : "${REDPANDA_CONSOLE_URL:=http://localhost:8888}"
  : "${MINIO_CONSOLE_URL:=http://localhost:9001}"
  in_infra() { local svc=$1; shift; docker exec "infrastructure-$svc-1" "$@"; }
fi
