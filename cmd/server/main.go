package main

import (
	"log"
	"net/http"

	"bubblewhite-backend/config"
	"bubblewhite-backend/middlewares"
	"bubblewhite-backend/routes"
	"bubblewhite-backend/seed"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()

	db := config.ConnectDatabase(cfg)
	config.AutoMigrate(db)

	config.ConnectR2(cfg)

	seed.Run(db)

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.Use(middlewares.CORSMiddleware(cfg))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Swagger UI + raw OpenAPI spec (see docs/).
	r.Static("/docs", "./docs")

	container := routes.Build(db)
	routes.RegisterRoutes(r, container)

	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	log.Printf("Bubble White API listening on :%s (env=%s)", port, cfg.AppEnv)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
