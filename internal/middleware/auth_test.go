package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"edu-game-be/internal/domain"
	"edu-game-be/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func makeTestJWT(t *testing.T, secret, role string) string {
	t.Helper()
	now := time.Now().UTC()
	claims := service.TokenClaims{
		UserID:    uuid.NewString(),
		Role:      role,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signed
}

func TestAdminRoutesDoNotRequireProfileHeader(t *testing.T) {
	const secret = "test_secret_key_for_route_isolation_32_chars"

	app := fiber.New()
	api := app.Group("/api/v1")

	requireAuth := RequireAuth(secret, nil)
	requireProfile := RequireProfile(nil)
	requireAdmin := RequireAdmin()

	profilesGroup := api.Group("/profiles", requireAuth)
	profilesGroup.Get("", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	worldsGroup := api.Group("/worlds", requireAuth, requireProfile)
	worldsGroup.Get("", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	stagesGroup := api.Group("/stages", requireAuth, requireProfile)
	stagesGroup.Post("/:stageId/start", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	inventoryGroup := api.Group("/inventory", requireAuth, requireProfile)
	inventoryGroup.Get("", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	adminGroup := api.Group("/admin", requireAuth, requireAdmin)
	adminGroup.Get("/worlds", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	adminToken := makeTestJWT(t, secret, domain.RoleAdmin)
	parentToken := makeTestJWT(t, secret, domain.RoleParent)

	// 1. Admin route without X-Profile-ID must succeed for admin
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/worlds", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("admin request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /api/v1/admin/worlds as admin without X-Profile-ID = %d, want 200", resp.StatusCode)
	}

	// 2. Admin route without X-Profile-ID as parent must return 403 (RequireAdmin), NOT 400 (RequireProfile)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/worlds", nil)
	req.Header.Set("Authorization", "Bearer "+parentToken)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("parent->admin request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("GET /api/v1/admin/worlds as parent = %d, want 403", resp.StatusCode)
	}

	// 3. Profiles route without X-Profile-ID must succeed
	req = httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	req.Header.Set("Authorization", "Bearer "+parentToken)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("profiles request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /api/v1/profiles without X-Profile-ID = %d, want 200", resp.StatusCode)
	}

	// 4. Gameplay & Inventory routes without X-Profile-ID must return 400
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/worlds"},
		{http.MethodPost, "/api/v1/stages/" + uuid.NewString() + "/start"},
		{http.MethodGet, "/api/v1/inventory"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("Authorization", "Bearer "+parentToken)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s %s failed: %v", tc.method, tc.path, err)
		}
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("%s %s without X-Profile-ID = %d, want 400", tc.method, tc.path, resp.StatusCode)
		}
	}
}
