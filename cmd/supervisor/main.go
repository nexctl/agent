package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"github.com/nexctl/agent/internal/app"
	"github.com/nexctl/agent/internal/config"
)

// main starts supervisor.
func main() {
	configPath := flag.String("config", "configs/supervisor.example.yaml", "supervisor config path")
	flag.Parse()

	cfg, err := config.LoadSupervisor(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	supervisor, err := app.NewSupervisor(cfg)
	if err != nil {
		log.Fatalf("create supervisor: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := supervisor.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("run supervisor: %v", err)
	}
}
