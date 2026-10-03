# ADR 0005: Gateway API with k3s's Traefik

- **Status:** accepted
- **Date:** 2026-10-03

## Context

Traffic entered through two `Ingress` objects (services, observability) and the Let's Encrypt issuer used the Ingress http01 solver. The Ingress API is feature-frozen; Gateway API is its successor, splits the cluster-owned Gateway from app-owned routes, and is portable across controllers. Every env runs k3s, which bundles Traefik 3.5 and the Gateway API v1.3 standard CRDs, with Traefik's Gateway provider off.

## Options

| Option | For | Against |
|--------|-----|---------|
| **Traefik's Gateway provider** | Already running in k3s; one `HelmChartConfig` turns it on; no extra pods on a 16 GB machine | Traefik binds listeners to its entrypoints by port (8000, 8443), so listener ports are controller-specific values |
| Envoy Gateway | Standalone, Envoy-based, independent of the k8s distro | One more controller plus an Envoy proxy per Gateway; Traefik would still run for nothing unless disabled in k3s |
| Keep Ingress | Nothing to change | Frozen API, annotations for anything beyond host/path |

## Decision

- The charts render only standard Gateway API objects: the services chart creates the `dsn` Gateway (`route.gateway.className`, listener ports in values) and an HTTPRoute for `route.host`; the observability chart adds one HTTPRoute per UI
- `route.gateway.create: false` plus `route.parentRefs` attaches the routes to a Gateway owned by the platform instead
- k3s's Traefik runs the Gateway (`platform-k3s/traefik.yaml` in the gitops repo, an Argo CD app in every env); its default Gateway stays off
- TLS: an HTTPS listener with the `cert-manager.io/cluster-issuer` annotation and HTTP redirecting to it; the issuer solves http01 through the `dsn` Gateway

## Consequences

- The app route needs a hostname (`localhost` locally): Traefik ranks a hostname-less `/api` prefix above a `Host(prometheus.localhost)` + `/` match, so a catch-all app route captured `/api/...` on the UI hosts. Locally the app answers on `localhost:8081`, not `127.0.0.1:8081`
- The OIDC issuer the gateway checks comes from `publicUrl`, no longer from the route host
- Moving to another controller is a values change: class name and listener ports
