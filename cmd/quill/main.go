package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/smalik2811/quill/internal/quill"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := quill.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
