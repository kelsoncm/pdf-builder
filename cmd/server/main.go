package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kelsoncm/pdf-builder/internal/auth"
	"github.com/kelsoncm/pdf-builder/internal/config"
	"github.com/kelsoncm/pdf-builder/internal/engine"
	"github.com/kelsoncm/pdf-builder/internal/fetcher"
	"github.com/kelsoncm/pdf-builder/internal/handler"
)

func main() {
	// --health-check mode: perform a single HTTP GET to /health and exit.
	// Used by the Docker HEALTHCHECK instruction (no shell required).
	if len(os.Args) > 1 && os.Args[1] == "--health-check" {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://localhost:8080/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Bootstrap logger (plain text until we have config).
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal("loading configuration", zap.Error(err))
	}

	// Set Gin mode based on environment.
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Health check — no auth required.
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Build services.
	tokenMap := cfg.TokenMap()
	pdfEng := engine.NewPDFEngine(cfg.Wkhtmltopdf.BinaryPath, cfg.Wkhtmltopdf.TimeoutSeconds)
	urlFetcher := fetcher.New(30 * time.Second)
	generateHandler := handler.NewGenerateHandler(pdfEng, urlFetcher, logger)

	// Authenticated routes.
	protected := router.Group("/")
	protected.Use(auth.Middleware(tokenMap, logger))
	protected.POST("/generate", generateHandler.Handle)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Graceful shutdown.
	go func() {
		logger.Info("starting PDF service", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}
}
