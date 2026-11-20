package main

import (
	"log"

	"edu-game-be/internal/config"
	"edu-game-be/internal/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.NewPostgres(cfg)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}

	if err := database.RunSQLMigrations(db, cfg.MigrationsDir); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	redisClient, err := database.NewRedis(cfg)
	if err != nil {
		log.Printf("warning: redis unavailable (%v), continuing seed without cache invalidation", err)
	} else {
		defer func() { _ = redisClient.Close() }()
	}

	if err := database.Seed(db, redisClient, cfg.S3PublicBaseURL); err != nil {
		log.Fatalf("seed failed: %v", err)
	}

	log.Println("Seed completed successfully! (5 worlds, 50 stages, 18 reward items)")
	log.Println("Demo Admin:  admin@edugame.local  / Admin123!")
	log.Println("Demo Parent: parent@edugame.local / Parent123! (Profile: Bé Bo)")
}
