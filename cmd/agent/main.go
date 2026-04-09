package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kardianos/service"
	"github.com/nexctl/agent/internal/app"
	"github.com/nexctl/agent/internal/config"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "service" {
		os.Exit(runServiceCLI(os.Args[2:]))
	}
	if len(os.Args) >= 2 && os.Args[1] == "reset-credential" {
		os.Exit(runResetCredentialCLI(os.Args[2:]))
	}

	// Windows：由服务管理器启动时必须走 kardianos service.Run（svc.Run），向 SCM 报告 Running；
	// Linux：由 systemd 启动时 Interactive() 为 false，同样须走 service.Run。
	if !service.Interactive() {
		os.Exit(runAsService())
	}

	configPath := flag.String("config", "configs/agent.example.yaml", "agent config path")
	showVersion := flag.Bool("version", false, "print version and exit")
	shortV := flag.Bool("v", false, "print version and exit")
	flag.Parse()

	if *showVersion || *shortV {
		fmt.Println(app.Version)
		return
	}

	cfg, err := config.LoadAgent(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	agent, err := app.NewAgent(cfg)
	if err != nil {
		log.Fatalf("create agent: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := agent.Run(ctx); err != nil {
		if errors.Is(err, app.ErrRestartAfterUpdate) {
			os.Exit(1)
		}
		if ctx.Err() == nil {
			log.Fatalf("run agent: %v", err)
		}
	}
}
