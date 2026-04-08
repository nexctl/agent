// 自更新逻辑参考 nezhahq agent 的 doSelfUpdate（GitHub Release + 临时 stat 文件防并发）。
package app

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/blang/semver"
	selfupd "github.com/creativeprojects/go-selfupdate"
	"github.com/nexctl/agent/internal/config"
	"go.uber.org/zap"
)

const binaryName = "nexctl-agent"

// doSelfUpdate 检查并应用 GitHub Release 更新。返回 true 表示应退出进程以便重启到新版本。
// useLocalVersion 为 true 时，会用子进程 -v 与编译期 Version 交叉校验（与 nezhahq 一致）。
func doSelfUpdate(ctx context.Context, logger *zap.Logger, cfg config.AgentConfig, useLocalVersion bool) bool {
	vFromMeta, err := ParseBuildVersion()
	if err != nil {
		logger.Debug("self-update skip: invalid semver in Version", zap.Error(err))
		return false
	}

	v := vFromMeta
	if useLocalVersion {
		exe, err := selfupd.ExecutablePath()
		if err != nil {
			logger.Warn("self-update: executable path", zap.Error(err))
			return false
		}
		cmd := exec.CommandContext(ctx, exe, "-v")
		out, err := cmd.Output()
		if err != nil {
			logger.Warn("self-update: read executable version", zap.Error(err))
			return false
		}
		parts := strings.Fields(strings.TrimSpace(string(out)))
		if len(parts) == 0 {
			logger.Warn("self-update: empty -v output")
			return false
		}
		vStr := strings.TrimPrefix(strings.TrimSpace(parts[len(parts)-1]), "v")
		vRun, err := semver.Parse(vStr)
		if err != nil {
			logger.Warn("self-update: parse -v semver", zap.Error(err), zap.String("raw", vStr))
			return false
		}
		if !vFromMeta.EQ(vRun) {
			logger.Warn("executable version differs from build metadata, exiting to re-check update",
				zap.String("build", vFromMeta.String()), zap.String("running", vRun.String()))
			return true
		}
		v = vRun
	}

	exePath, err := selfupd.ExecutablePath()
	if err != nil {
		logger.Warn("self-update: executable path", zap.Error(err))
		return false
	}

	sum := md5.Sum([]byte(exePath))
	statName := fmt.Sprintf("agent-%s.stat", hex.EncodeToString(sum[:])[:7])
	tmpDir := filepath.Join(os.TempDir(), binaryName)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		logger.Warn("self-update: temp dir", zap.Error(err))
		return false
	}
	statFile := filepath.Join(tmpDir, statName)

	if _, err := os.Stat(statFile); err == nil {
		logger.Info("self-update: waiting for another process to finish", zap.String("stat", statFile))
		if waitErr := waitStatFileGone(ctx, statFile, 2*time.Minute); waitErr != nil {
			if errors.Is(waitErr, context.DeadlineExceeded) || errors.Is(waitErr, context.Canceled) {
				_ = os.Remove(statFile)
			}
			logger.Warn("self-update: stat file wait", zap.Error(waitErr))
			return false
		}
		return true
	} else if !errors.Is(err, os.ErrNotExist) {
		logger.Warn("self-update: stat file", zap.Error(err))
		return false
	}

	stat, err := os.OpenFile(statFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		logger.Warn("self-update: create stat file", zap.Error(err))
		return false
	}
	_ = stat.Close()
	defer func() { _ = os.Remove(statFile) }()

	repo := selfupd.ParseSlug(cfg.GithubRepo)
	updater, err := selfupd.NewUpdater(selfupd.Config{})
	if err != nil {
		logger.Warn("self-update: create updater", zap.Error(err))
		return false
	}

	logger.Info("self-update: checking", zap.String("current", v.String()), zap.String("repo", cfg.GithubRepo))
	rel, err := updater.UpdateSelf(ctx, v.String(), repo)
	if err != nil {
		logger.Warn("self-update: update failed", zap.Error(err))
		return false
	}

	if rel == nil {
		return false
	}
	latest, err := semver.Parse(rel.Version())
	if err != nil {
		return false
	}
	if latest.GT(v) {
		logger.Info("self-update: updated, exiting for restart", zap.String("to", latest.String()))
		return true
	}
	return false
}

func waitStatFileGone(ctx context.Context, path string, maxWait time.Duration) error {
	deadline := time.NewTimer(maxWait)
	defer deadline.Stop()
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return context.DeadlineExceeded
		case <-tick.C:
			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				return nil
			}
		}
	}
}
