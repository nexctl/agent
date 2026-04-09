package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kardianos/service"
	"github.com/nexctl/agent/internal/app"
	"github.com/nexctl/agent/internal/config"
	"github.com/nexctl/agent/internal/store"
)

// noopProgram 仅用于 install/uninstall 等控制命令注册服务；进程由 SCM/systemd 拉起时改用 agentProgram。
type noopProgram struct{}

func (p *noopProgram) Start(s service.Service) error { return nil }
func (p *noopProgram) Stop(s service.Service) error { return nil }

// agentProgram 在系统服务模式下运行 Agent（Windows 须通过 kardianos svc.Run 向 SCM 报告 Running，否则超时 1053）。
type agentProgram struct {
	absConfig string
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func newAgentProgram(absConfig string) *agentProgram {
	return &agentProgram{absConfig: absConfig}
}

func (p *agentProgram) Start(s service.Service) error {
	logger, _ := s.Logger(nil)
	p.wg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go func() {
		defer p.wg.Done()
		cfg, err := config.LoadAgent(p.absConfig)
		if err != nil {
			if logger != nil {
				_ = logger.Errorf("load config: %v", err)
			}
			return
		}
		agent, err := app.NewAgent(cfg)
		if err != nil {
			if logger != nil {
				_ = logger.Errorf("create agent: %v", err)
			}
			return
		}
		if err := agent.Run(ctx); err != nil {
			if errors.Is(err, app.ErrRestartAfterUpdate) {
				os.Exit(1)
			}
			if ctx.Err() == nil && logger != nil {
				_ = logger.Errorf("run agent: %v", err)
			}
		}
	}()
	return nil
}

func (p *agentProgram) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(25 * time.Second):
	}
	return nil
}

// runAsService 由 systemd / Windows SCM 拉起时调用，必须走 service.Run，不得直接跑 main 里的 agent.Run。
func runAsService() int {
	fs := flag.NewFlagSet("nexctl-agent", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "configs/agent.example.yaml", "agent config path")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return 1
	}
	absConfig, err := filepath.Abs(*configPath)
	if err != nil {
		return 1
	}
	prg := newAgentProgram(absConfig)
	cfg := buildServiceConfig(absConfig)
	svc, err := service.New(prg, cfg)
	if err != nil {
		return 1
	}
	if err := svc.Run(); err != nil {
		return 1
	}
	return 0
}

func printServiceUsage() {
	fmt.Fprintf(os.Stderr, `用法: %s service <子命令> [-config <配置文件>]

子命令:
  install     安装为系统服务（开机自启，需管理员/root）
  uninstall   卸载服务
  start       启动服务
  stop        停止服务
  restart     重启服务
  status      查看运行状态

示例:
  %s service install -config /etc/nexctl/agent.yaml
  %s service install -config /etc/nexctl/agent.yaml -keep-credential
  %s service status -config /etc/nexctl/agent.yaml

说明:
  - 系统服务固定名为 nexctl（Linux: systemd 单元；Windows: 服务名）。
  - install 默认会先删除 credential.json（覆盖重装）；若需保留已有凭证，请加 -keep-credential。
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

const fixedServiceName = "nexctl"

func buildServiceConfig(absConfig string) *service.Config {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	exeDir := filepath.Dir(exe)

	// Linux systemd: Restart=；Windows: OnFailure（见 kardianos/service 文档）
	kv := service.KeyValue{
		"Restart":             "always",
		"OnFailure":           "restart",
		"OnFailureDelayDuration": "5s",
	}

	return &service.Config{
		Name:             fixedServiceName,
		DisplayName:      "NexCtl",
		Description:      "NexCtl monitoring agent",
		Arguments:        []string{"-config", absConfig},
		WorkingDirectory: exeDir,
		Option:           kv,
	}
}

func runServiceCLI(args []string) int {
	if len(args) < 1 {
		printServiceUsage()
		return 2
	}
	action := args[0]
	fs := flag.NewFlagSet("service", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "configs/agent.example.yaml", "配置文件路径")
	keepCred := fs.Bool("keep-credential", false, "仅 install：保留已有 credential.json（默认会删除以实现覆盖重装）")
	if err := fs.Parse(args[1:]); err != nil {
		log.Printf("参数错误: %v", err)
		printServiceUsage()
		return 2
	}
	if fs.NArg() != 0 {
		log.Printf("未知参数: %v", fs.Args())
		printServiceUsage()
		return 2
	}

	absConfig, err := filepath.Abs(*configPath)
	if err != nil {
		log.Printf("解析配置路径: %v", err)
		return 1
	}

	cfg := buildServiceConfig(absConfig)
	prg := &noopProgram{}
	svc, err := service.New(prg, cfg)
	if err != nil {
		log.Printf("创建服务对象失败: %v", err)
		return 1
	}

	switch action {
	case "help", "-h", "--help":
		printServiceUsage()
		return 0
	case "install":
		if !*keepCred {
			agentCfg, err := config.LoadAgent(absConfig)
			if err != nil {
				log.Printf("加载配置: %v", err)
				return 1
			}
			credPath := store.CredentialPath(agentCfg.CredentialDir)
			if err := store.RemoveCredentialFile(agentCfg.CredentialDir); err != nil {
				log.Printf("删除凭证: %v", err)
				return 1
			}
			fmt.Println("已删除旧凭证（重装覆盖）:", credPath)
		}
		fmt.Println("init system:", service.Platform())
		if err := svc.Install(); err != nil {
			log.Printf("install: %v", err)
			return 1
		}
		fmt.Println("已安装服务:", cfg.Name)
		fmt.Println("随后请使用本机服务管理命令启动（如 Linux: systemctl start；Windows: sc start 或服务管理器；macOS: launchctl）。")
		return 0
	case "uninstall":
		if err := svc.Uninstall(); err != nil {
			log.Printf("uninstall: %v", err)
			return 1
		}
		fmt.Println("已卸载服务:", cfg.Name)
		return 0
	case "start":
		if err := svc.Start(); err != nil {
			log.Printf("start: %v", err)
			return 1
		}
		fmt.Println("已启动:", cfg.Name)
		return 0
	case "stop":
		if err := svc.Stop(); err != nil {
			log.Printf("stop: %v", err)
			return 1
		}
		fmt.Println("已停止:", cfg.Name)
		return 0
	case "restart":
		if err := svc.Restart(); err != nil {
			log.Printf("restart: %v", err)
			return 1
		}
		fmt.Println("已重启:", cfg.Name)
		return 0
	case "status":
		st, err := svc.Status()
		if err != nil {
			log.Printf("status: %v", err)
			return 1
		}
		switch st {
		case service.StatusRunning:
			fmt.Println("running")
		case service.StatusStopped:
			fmt.Println("stopped")
		default:
			fmt.Println("unknown")
		}
		return 0
	default:
		log.Printf("未知子命令: %s", action)
		printServiceUsage()
		return 2
	}
}
