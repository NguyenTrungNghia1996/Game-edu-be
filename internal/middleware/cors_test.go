package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestCORSCapacitorAndMobileOrigins(t *testing.T) {
	app := fiber.New()
	app.Use(NewCORS("https://game.yourdomain.com,capacitor://localhost,https://localhost,http://localhost:3000"))
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	allowedOrigins := []string{
		"capacitor://localhost",
		"https://localhost",
		"http://localhost:3000",
		"https://game.yourdomain.com",
	}

	for _, origin := range allowedOrigins {
		req := httptest.NewRequest(http.MethodOptions, "/health", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Authorization, X-Profile-ID, Content-Type")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed for origin %s: %v", origin, err)
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("origin %s: Access-Control-Allow-Origin = %q, want %q", origin, got, origin)
		}
	}

	// Disallowed origin should not get Allow-Origin echoed back
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed for disallowed origin: %v", err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("disallowed origin got Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestCORSWildcardAllowsAllOrigins(t *testing.T) {
	app := fiber.New()
	app.Use(NewCORS("*"))
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	for _, origin := range []string{"https://any-domain.com", "capacitor://localhost", "http://localhost:5173"} {
		req := httptest.NewRequest(http.MethodOptions, "/health", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Authorization, X-Profile-ID, Content-Type")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed for origin %s: %v", origin, err)
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("origin %s: Access-Control-Allow-Origin = %q, want %q", origin, got, "*")
		}
	}
}
