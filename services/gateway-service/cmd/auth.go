package main

import (
	"context"
	"log"
	"log/slog"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// tokenVerifier checks a bearer token's signature, issuer, audience and expiry.
type tokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (*oidc.IDToken, error)
}

// newVerifier trusts tokens signed with the keys at OIDC_JWKS_URL (the realm's
// cluster-internal address) that name OIDC_AUDIENCE. OIDC_ISSUER, the public
// address browsers use, is checked when set; without a known public host it is
// left out and the keys are the only proof of origin.
func newVerifier(ctx context.Context) tokenVerifier {
	jwks := envOrDefault("OIDC_JWKS_URL", "")
	if jwks == "" {
		log.Fatal("OIDC_JWKS_URL is required")
	}
	issuer := strings.TrimSuffix(envOrDefault("OIDC_ISSUER", ""), "/")
	if issuer == "" {
		slog.WarnContext(ctx, "OIDC_ISSUER not set, token issuer is not checked")
	}
	keys := oidc.NewRemoteKeySet(ctx, jwks)
	return oidc.NewVerifier(issuer, keys, &oidc.Config{
		ClientID:        envOrDefault("OIDC_AUDIENCE", "dsn-api"),
		SkipIssuerCheck: issuer == "",
	})
}

type identity struct {
	ID       string
	Username string
}

const identityKey = "identity"

// requireUser rejects requests without a valid bearer token and stores the caller's
// identity for handlers; the token, not the body, says who is acting.
func requireUser(v tokenVerifier) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw, ok := strings.CutPrefix(c.Get(fiber.HeaderAuthorization), "Bearer ")
		if !ok || raw == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing bearer token"})
		}
		token, err := v.Verify(c.UserContext(), raw)
		if err != nil {
			slog.InfoContext(c.UserContext(), "rejected token", "error", err)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
		}
		var claims struct {
			Username string `json:"preferred_username"`
		}
		if err := token.Claims(&claims); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token claims"})
		}

		trace.SpanFromContext(c.UserContext()).SetAttributes(attribute.String("enduser.id", token.Subject))
		c.Locals(identityKey, identity{ID: token.Subject, Username: claims.Username})
		return c.Next()
	}
}

// caller is the identity requireUser stored; only call it behind requireUser.
func caller(c *fiber.Ctx) identity {
	id, _ := c.Locals(identityKey).(identity)
	return id
}
