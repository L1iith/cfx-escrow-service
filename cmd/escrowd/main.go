package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/L1iith/cfx-escrow-service/internal/api"
	"github.com/L1iith/cfx-escrow-service/internal/auth"
	"github.com/L1iith/cfx-escrow-service/internal/config"
	"github.com/L1iith/cfx-escrow-service/internal/runner"
	"github.com/L1iith/cfx-escrow-service/internal/store"
	"github.com/L1iith/cfx-escrow-service/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	jobStore, err := store.Open(cfg.DataDirectory)
	if err != nil {
		slog.Error("store failed", "error", err)
		os.Exit(1)
	}
	root, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	manager := worker.NewManager(jobStore, runner.New(cfg), cfg.JobTimeout)
	manager.Start(root)
	defer manager.Stop()

	verifier := auth.NewVerifier(cfg.APISecret, 5*time.Minute, cfg.MaxBodyBytes)
	handler := api.New(jobStore, manager, cfg.MaxBodyBytes).Handler(verifier.Middleware)
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-root.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	slog.Info("service started", "address", cfg.ListenAddress)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
