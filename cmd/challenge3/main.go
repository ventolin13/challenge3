// Package main serves the coin transfer API.
//
// @title       Coin transfers
// @version     1.0
// @description Переводы монет между игровыми аккаунтами.
// @host        localhost:3000
// @BasePath    /
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/ventolin13/challenge3/internal/config"
	"github.com/ventolin13/challenge3/internal/handler"
	"github.com/ventolin13/challenge3/internal/repositories"
	"github.com/ventolin13/challenge3/internal/services"

	_ "github.com/ventolin13/challenge3/internal/swagdocs"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 5 * time.Second
	listenErrBuffer   = 1
)

func main() {
	if err := run(); err != nil {
		log.Printf("%v", err)
		os.Exit(1)
	}
}

func run() error {
	log.Printf("service main has been started")
	cfg := config.Load()
	if os.Getenv("DATABASE_URL") == "" {
		log.Printf("DATABASE_URL is empty, using 127.0.0.1:5432/challenge")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := repositories.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("can't start postgres pool: %w", err)
	}
	log.Printf("postgres pool has been started")
	defer func() {
		db.Close()
		log.Printf("postgres pool has been closed")
		if errors.Is(ctx.Err(), context.Canceled) {
			log.Printf("service main is gracefully shutdown.")
		}
	}()

	accounts := repositories.NewAccounts(db)
	if cfg.SeedAccounts {
		log.Printf("seed accounts has been started")
		if err = accounts.EnsureSeed(ctx); err != nil {
			return fmt.Errorf("seed accounts failed: %w", err)
		}
		log.Printf("seed accounts has been closed")
	}
	api := handler.NewTransfers(services.NewTransfers(db, accounts, repositories.NewTransfers(db)))

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.POST("/api/v1/transfers", api.PostTransfer)
	engine.GET("/api/v1/accounts/:id", api.GetAccount)
	engine.GET("/api/v1/transfers/:idempotency_key", api.GetTransfer)
	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	server := &http.Server{
		Addr:                         cfg.HTTPAddr,
		Handler:                      engine,
		DisableGeneralOptionsHandler: false,
		TLSConfig:                    nil,
		ReadTimeout:                  0,
		ReadHeaderTimeout:            readHeaderTimeout,
		WriteTimeout:                 0,
		IdleTimeout:                  0,
		MaxHeaderBytes:               0,
		TLSNextProto:                 nil,
		ConnState:                    nil,
		ErrorLog:                     nil,
		BaseContext:                  nil,
		ConnContext:                  nil,
		HTTP2:                        nil,
		Protocols:                    nil,
	}
	listenErr := make(chan error, listenErrBuffer)
	go func() {
		log.Printf("http server has been started on %s", cfg.HTTPAddr)
		listenErr <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Printf("http server is shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		shutdownErr := server.Shutdown(shutdownCtx)
		cancel()
		if shutdownErr != nil {
			log.Printf("can't close http server: %v", shutdownErr)
		} else {
			log.Printf("http server has been closed")
		}
		if err = <-listenErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server failed: %w", err)
		}
	case err = <-listenErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server failed: %w", err)
		}
	}
	return nil
}
