package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"edu-game-be/internal/config"
	"edu-game-be/internal/database"
	"edu-game-be/internal/handler"
	"edu-game-be/internal/middleware"
	"edu-game-be/internal/repository"
	"edu-game-be/internal/service"
	"edu-game-be/internal/storage"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := database.NewPostgres(cfg)
	if err != nil {
		log.Fatalf("failed to connect postgres: %v", err)
	}

	if cfg.AutoMigrate {
		if err := database.RunSQLMigrations(db, cfg.MigrationsDir); err != nil {
			log.Fatalf("failed to run migrations: %v", err)
		}
		log.Println("database migrations applied")
	}

	redisClient, err := database.NewRedis(cfg)
	if err != nil {
		log.Fatalf("failed to connect redis: %v", err)
	}
	defer func() { _ = redisClient.Close() }()

	if cfg.AutoSeed {
		if err := database.Seed(db, redisClient, cfg.S3PublicBaseURL); err != nil {
			log.Fatalf("failed to auto-seed database: %v", err)
		}
		log.Println("database auto-seed completed")
	}

	r2Storage, err := storage.NewR2Storage(
		cfg.S3Endpoint,
		cfg.S3Region,
		cfg.S3AccessKeyID,
		cfg.S3SecretAccessKey,
		cfg.S3Bucket,
		cfg.S3PublicBaseURL,
	)
	if err != nil {
		log.Fatalf("failed to init R2 storage: %v", err)
	}

	// Repositories
	authRepo := repository.NewAuthRepository(db)
	worldRepo := repository.NewWorldRepository(db)
	stageRepo := repository.NewStageRepository(db)
	progressRepo := repository.NewProgressRepository(db)

	// Services
	authService := service.NewAuthService(authRepo, worldRepo, stageRepo, progressRepo, cfg)
	gameplayService := service.NewGameplayService(worldRepo, stageRepo, progressRepo, redisClient)
	inventoryService := service.NewInventoryService(progressRepo)
	adminService := service.NewAdminService(worldRepo, stageRepo, progressRepo, r2Storage, redisClient)

	// Handlers
	healthHandler := handler.NewHealthHandler(db, redisClient)
	authHandler := handler.NewAuthHandler(authService)
	profileHandler := handler.NewProfileHandler(authService)
	gameplayHandler := handler.NewGameplayHandler(gameplayService)
	inventoryHandler := handler.NewInventoryHandler(inventoryService)
	adminHandler := handler.NewAdminHandler(adminService)

	app := fiber.New(fiber.Config{
		AppName:      "EduGame Backend API",
		BodyLimit:    10 * 1024 * 1024, // 10MB JSON limit (large assets go directly to R2)
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(middleware.NewCORS(cfg.CORSAllowedOrigins))

	// Health check (Postgres + Redis)
	app.Get("/health", healthHandler.Health)

	api := app.Group("/api/v1")

	// Auth routes (rate limited)
	authLimiter := limiter.New(limiter.Config{
		Max:        30,
		Expiration: 1 * time.Minute,
	})
	authGroup := api.Group("/auth", authLimiter)
	authGroup.Post("/register", authHandler.Register)
	authGroup.Post("/login", authHandler.Login)
	authGroup.Post("/refresh", authHandler.Refresh)

	requireAuth := middleware.RequireAuth(cfg.JWTSecret, authRepo)
	requireProfile := middleware.RequireProfile(authRepo)
	requireAdmin := middleware.RequireAdmin()

	// Profiles (JWT required)
	profilesGroup := api.Group("/profiles", requireAuth)
	profilesGroup.Get("", profileHandler.List)
	profilesGroup.Post("", profileHandler.Create)

	// Gameplay & Inventory (JWT + X-Profile-ID required)
	worldsGroup := api.Group("/worlds", requireAuth, requireProfile)
	worldsGroup.Get("", gameplayHandler.ListWorlds)
	worldsGroup.Get("/:worldId/map", gameplayHandler.GetWorldMap)

	stagesGroup := api.Group("/stages", requireAuth, requireProfile)
	stagesGroup.Post("/:stageId/start", gameplayHandler.StartStage)
	stagesGroup.Post("/:stageId/submit", gameplayHandler.SubmitAnswer)

	inventoryGroup := api.Group("/inventory", requireAuth, requireProfile)
	inventoryGroup.Get("", inventoryHandler.List)
	inventoryGroup.Patch("/:itemId/equip", inventoryHandler.Equip)

	// Admin CMS routes (JWT + role == admin; does NOT run RequireProfile)
	adminGroup := api.Group("/admin", requireAuth, requireAdmin)
	adminGroup.Post("/upload-url", adminHandler.GenerateUploadURL)

	adminGroup.Get("/worlds", adminHandler.ListWorlds)
	adminGroup.Get("/worlds/:id", adminHandler.GetWorld)
	adminGroup.Post("/worlds", adminHandler.CreateWorld)
	adminGroup.Put("/worlds/:id", adminHandler.UpdateWorld)
	adminGroup.Delete("/worlds/:id", adminHandler.DeleteWorld)

	adminGroup.Get("/stages", adminHandler.ListStages)
	adminGroup.Get("/stages/:id", adminHandler.GetStage)
	adminGroup.Post("/stages", adminHandler.CreateStage)
	adminGroup.Put("/stages/:id", adminHandler.UpdateStage)
	adminGroup.Delete("/stages/:id", adminHandler.DeleteStage)

	adminGroup.Get("/challenges", adminHandler.ListChallenges)
	adminGroup.Get("/challenges/:id", adminHandler.GetChallenge)
	adminGroup.Post("/challenges", adminHandler.CreateChallenge)
	adminGroup.Put("/challenges/:id", adminHandler.UpdateChallenge)
	adminGroup.Delete("/challenges/:id", adminHandler.DeleteChallenge)

	adminGroup.Get("/items", adminHandler.ListItems)
	adminGroup.Get("/items/:id", adminHandler.GetItem)
	adminGroup.Post("/items", adminHandler.CreateItem)
	adminGroup.Put("/items/:id", adminHandler.UpdateItem)
	adminGroup.Delete("/items/:id", adminHandler.DeleteItem)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down server...")
		_ = app.ShutdownWithTimeout(10 * time.Second)
	}()

	addr := ":" + cfg.AppPort
	log.Printf("EduGame API listening on %s (env=%s)", addr, cfg.AppEnv)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
