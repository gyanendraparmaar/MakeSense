package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gyanendraparmaar/makesense/backend/internal/config"
	"github.com/gyanendraparmaar/makesense/backend/internal/llm"
	"github.com/gyanendraparmaar/makesense/backend/internal/server"
	"github.com/gyanendraparmaar/makesense/backend/internal/storage"
)

func main() {
	cfg := config.Load()

	store, err := storage.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	gem := llm.NewGeminiClient(cfg.GeminiAPIKey, cfg.GeminiModel)
	pipeline := llm.NewPipeline(gem)

	srv := server.New(cfg, store, pipeline)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout so SSE streams can stay open.
	}

	// Graceful shutdown.
	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("shutdown: draining connections…")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	log.Printf("makesense listening on :%s (model=%s, db=%s)", cfg.Port, cfg.GeminiModel, cfg.DBPath)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
	<-idleConnsClosed
	log.Println("bye")
}
