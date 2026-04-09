package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nexctl/agent/internal/config"
	"github.com/nexctl/agent/internal/store"
)

func printResetCredentialUsage() {
	fmt.Fprintf(os.Stderr, `用法: %s reset-credential [-config <配置文件>]

删除 credential_dir 下的 credential.json。若使用 agent.yaml 中的 agent_id/agent_secret/node_key，则不受影响。
不通过 service 安装时，可用本命令手动清凭证；service install 默认已会删除凭证（除非加 -keep-credential）。

`, os.Args[0])
}

func runResetCredentialCLI(args []string) int {
	fs := flag.NewFlagSet("reset-credential", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "configs/agent.example.yaml", "配置文件路径")
	if err := fs.Parse(args); err != nil {
		log.Printf("参数错误: %v", err)
		printResetCredentialUsage()
		return 2
	}
	if fs.NArg() != 0 {
		printResetCredentialUsage()
		return 2
	}

	cfg, err := config.LoadAgent(*configPath)
	if err != nil {
		log.Printf("加载配置: %v", err)
		return 1
	}
	path := store.CredentialPath(cfg.CredentialDir)
	if err := store.RemoveCredentialFile(cfg.CredentialDir); err != nil {
		log.Printf("删除凭证: %v", err)
		return 1
	}
	fmt.Println("已删除凭证文件（若原本不存在则无需操作）:", path)
	return 0
}
