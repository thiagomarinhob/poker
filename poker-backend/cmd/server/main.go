package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/thiagomarinho/poker-backend/internal/admin"
	"github.com/thiagomarinho/poker-backend/internal/auditlog"
	"github.com/thiagomarinho/poker-backend/internal/auth"
	"github.com/thiagomarinho/poker-backend/internal/cache"
	"github.com/thiagomarinho/poker-backend/internal/config"
	"github.com/thiagomarinho/poker-backend/internal/db"
	"github.com/thiagomarinho/poker-backend/internal/pokerdb"
	"github.com/thiagomarinho/poker-backend/internal/table"
	"github.com/thiagomarinho/poker-backend/internal/user"
	"github.com/thiagomarinho/poker-backend/internal/ws"
)

func main() {
	// Console writer for startup messages before config is loaded.
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}

	if cfg.IsProd() {
		gin.SetMode(gin.ReleaseMode)
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.Kitchen})
	}
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer pool.Close()
	log.Info().Msg("database connected")

	redisClient, err := cache.Connect(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("redis connection failed")
	}
	defer redisClient.Close()
	log.Info().Msg("redis connected")

	queries := user.New(pool)
	authSvc := auth.NewService(queries, redisClient, cfg.JWTSecret)
	authHandler := auth.NewHandler(authSvc)

	tableReg := table.NewRegistry(ctx)
	hub := ws.NewHub(tableReg)
	tableReg.SetClearPumpHook(func(id uuid.UUID) { hub.ClearPump(id) })
	wsHandler := ws.NewHandler(hub, cfg.JWTSecret)
	tableHandler := table.NewHandler(tableReg, hub.PumpRoom)

	adminHandler := admin.New(
		queries,
		pokerdb.New(pool),
		auditlog.New(pool),
		tableReg,
		hub,
		hub.PumpRoom,
	)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	authHandler.RegisterRoutes(api)
	wsHandler.RegisterRoutes(api)

	player := api.Group("", auth.AuthRequired(cfg.JWTSecret))
	tableHandler.RegisterPlayer(player)

	adminGroup := api.Group("/admin", auth.AuthRequired(cfg.JWTSecret), auth.AdminOnly())
	adminHandler.RegisterRoutes(adminGroup)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  0,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Info().Str("port", cfg.Port).Str("env", cfg.Env).Msg("server started")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("shutdown signal received")

	tableReg.Shutdown()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed, forcing exit")
	}
	log.Info().Msg("server stopped")
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", time.Since(start)).
			Msg("request")
	}
}
