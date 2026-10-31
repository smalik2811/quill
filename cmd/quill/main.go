package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/smalik2811/quill/internal/quill"
)

func main() {
	// Create a context cancelled on termination signals (SIGINT, SIGTERM, SIGHUP)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	// Channel to catch a second signal for forced immediate exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigChan)

	// Run quill in a background goroutine so we can coordinate graceful shutdown
	errChan := make(chan error, 1)
	go func() {
		errChan <- quill.Run(ctx)
	}()

	select {
	case err := <-errChan:
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Fatalf("quill exited with error: %v", err)
		}
	case <-ctx.Done():
		log.Println("Received termination signal, shutting down gracefully...")

		// If user sends a second signal while shutting down, terminate immediately
		go func() {
			select {
			case sig := <-sigChan:
				log.Printf("Received second signal (%v), terminating immediately.", sig)
				os.Exit(1)
			case <-errChan:
			}
		}()

		const shutdownTimeout = 5 * time.Second
		select {
		case err := <-errChan:
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Fatalf("quill exited with error during shutdown: %v", err)
			}
			log.Println("quill shut down gracefully.")
		case <-time.After(shutdownTimeout):
			log.Printf("Graceful shutdown timed out after %v, terminating forcefully.", shutdownTimeout)
			os.Exit(1)
		}
	}
}
