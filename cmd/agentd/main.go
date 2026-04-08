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

// main starts agentd.
func main() {
	configPath := flag.String("config", "configs/agent.example.yaml", "agent config path")
	flag.Parse()

	cfg, err := config.LoadAgent(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	agentd, err := app.NewAgentd(cfg)
	if err != nil {
		log.Fatalf("create agentd: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := agentd.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("run agentd: %v", err)
	}
}
