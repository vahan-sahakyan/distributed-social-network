package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/gofiber/fiber/v2"
)

const testIssuer = "http://keycloak.test/auth/realms/dsn"

func signer(t *testing.T) (*rsa.PrivateKey, tokenVerifier) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keys := &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}
	return key, oidc.NewVerifier(testIssuer, keys, &oidc.Config{ClientID: "dsn-api"})
}

func sign(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	s, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(claims)
	jws, err := s.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jws.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func validClaims() map[string]any {
	return map[string]any{
		"iss":                testIssuer,
		"aud":                []string{"dsn-api", "account"},
		"sub":                "user-1",
		"preferred_username": "alice",
		"exp":                time.Now().Add(time.Minute).Unix(),
		"iat":                time.Now().Unix(),
	}
}

func TestRequireUser(t *testing.T) {
	key, verifier := signer(t)
	otherKey, _ := signer(t)
	app := fiber.New()
	app.Get("/me", requireUser(verifier), func(c *fiber.Ctx) error {
		return c.JSON(caller(c))
	})

	with := func(mutate func(map[string]any)) string {
		claims := validClaims()
		mutate(claims)
		return "Bearer " + sign(t, key, claims)
	}
	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"valid", with(func(map[string]any) {}), 200},
		{"no header", "", 401},
		{"not bearer", "Basic abc", 401},
		{"garbage", "Bearer not.a.jwt", 401},
		{"expired", with(func(c map[string]any) { c["exp"] = time.Now().Add(-time.Minute).Unix() }), 401},
		{"other issuer", with(func(c map[string]any) { c["iss"] = "http://evil.test/realms/dsn" }), 401},
		{"other audience", with(func(c map[string]any) { c["aud"] = []string{"account"} }), 401},
		{"other signer", "Bearer " + sign(t, otherKey, validClaims()), 401},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/me", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != tt.want {
				t.Fatalf("status %d, want %d: %s", resp.StatusCode, tt.want, body)
			}
			if tt.want == 200 {
				var id identity
				_ = json.Unmarshal(body, &id)
				if id.ID != "user-1" || id.Username != "alice" {
					t.Errorf("identity = %+v, want user-1/alice", id)
				}
			}
		})
	}
}

func TestRequireUserWithoutIssuerStillChecksKeysAndAudience(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keys := &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}
	verifier := oidc.NewVerifier("", keys, &oidc.Config{ClientID: "dsn-api", SkipIssuerCheck: true})
	otherKey, _ := signer(t)
	app := fiber.New()
	app.Get("/me", requireUser(verifier), func(c *fiber.Ctx) error { return c.SendStatus(200) })

	anyIssuer := validClaims()
	anyIssuer["iss"] = "http://whatever.test/auth/realms/dsn"
	noAudience := validClaims()
	noAudience["aud"] = []string{"account"}
	for name, tc := range map[string]struct {
		token string
		want  int
	}{
		"any issuer":   {sign(t, key, anyIssuer), 200},
		"other signer": {sign(t, otherKey, validClaims()), 401},
		"no audience":  {sign(t, key, noAudience), 401},
	} {
		req := httptest.NewRequest("GET", "/me", nil)
		req.Header.Set("Authorization", "Bearer "+tc.token)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != tc.want {
			t.Errorf("%s: status %d, want %d", name, resp.StatusCode, tc.want)
		}
	}
}
