package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"ptcgl-omarchy/internal/app"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(app.RunCLI(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
