package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func NewCORS(allowedOrigins string) fiber.Handler {
	origins := strings.TrimSpace(allowedOrigins)
	if origins == "" {
		origins = "*"
	}

	// GoFiber v2 panics if AllowOrigins is "*" while AllowCredentials is true.
	// Since this API authenticates via Bearer JWT headers (not cookies), AllowCredentials
	// is disabled automatically when wildcard "*" is configured.
	allowCredentials := origins != "*"

	return cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Profile-ID",
		ExposeHeaders:    "Content-Length",
		AllowCredentials: allowCredentials,
		MaxAge:           86400,
	})
}
