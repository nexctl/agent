package main

import (
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kardianos/service"
)

// noopProgram 仅用于 install/uninstall 等控制命令注册服务；实际运行由 systemd/launchd 等直接执行
// `nexctl-agent -config <path>`，不会调用 Start/Stop。
type noopProgram struct{}

func (p *noopProgram) Start(s service.Service) error { return nil }
func (p *noopProgram) Stop(s service.Service) error  { return nil }

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
  %s service status -config /etc/nexctl/agent.yaml

说明: 不同配置文件会注册为不同服务名（与 nezhahq agent 类似，避免多实例冲突）。
`, os.Args[0], os.Args[0], os.Args[0])
}

func serviceNameForConfig(absConfig string) string {
	defaultAbs, err := filepath.Abs("configs/agent.example.yaml")
	if err == nil && filepath.Clean(absConfig) == filepath.Clean(defaultAbs) {
		return "nexctl-agent"
	}
	sum := md5.Sum([]byte(filepath.Clean(absConfig)))
	return "nexctl-agent-" + hex.EncodeToString(sum[:])[:7]
}

func buildServiceConfig(absConfig string) *service.Config {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	exeDir := filepath.Dir(exe)
	name := serviceNameForConfig(absConfig)

	// Linux systemd: Restart=；Windows: OnFailure（见 kardianos/service 文档）
	kv := service.KeyValue{
		"Restart":             "always",
		"OnFailure":           "restart",
		"OnFailureDelayDuration": "5s",
	}

	return &service.Config{
		Name:             name,
		DisplayName:      "NexCtl Agent",
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
