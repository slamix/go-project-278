package main

import (
	"link-shortener/internal/config"
	"link-shortener/internal/db"
	"link-shortener/internal/handler"
	"link-shortener/internal/service"
	"log"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	dbPool, err := db.NewPostgresPool(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	queries := db.New(dbPool)
	linkService := service.NewLinkService(queries)
	linkVisitService := service.NewLinkVisitService(queries)

	router := handler.NewRouter(linkService, linkVisitService, handler.RouterOptions{
		SentryDSN:    cfg.SentryDSN,
		ShortURLBase: cfg.ShortURLBase,
	})
	if err := router.Run(cfg.ServerAddress()); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
