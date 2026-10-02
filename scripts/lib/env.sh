#!/usr/bin/env bash
# Where the scripts point: the compose stack, or the local k3d cluster with CLUSTER=1
# (make demo CLUSTER=1). Any URL can still be overridden on its own.

if [[ "${CLUSTER:-}" == 1 ]]; then
  : "${GATEWAY_URL:=http://localhost:8081}"
  : "${KEYCLOAK_URL:=http://localhost:8081/auth}"
  : "${PROMETHEUS_URL:=http://prometheus.localhost:8081}"
  : "${GRAFANA_URL:=http://grafana.localhost:8081}"
  : "${JAEGER_URL:=http://jaeger.localhost:8081}"
  : "${REDPANDA_CONSOLE_URL:=http://redpanda.localhost:8081}"
  : "${MINIO_CONSOLE_URL:=http://minio.localhost:8081}"
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
