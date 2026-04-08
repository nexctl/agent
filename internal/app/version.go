package app

import (
	"strings"

	"github.com/blang/semver"
)

// Version 由 GoReleaser 在发布构建时通过 -ldflags 注入；本地开发默认为 dev。
var Version = "0.0.0-dev"

// ParseBuildVersion 解析 Version（支持可选的 v 前缀），无法解析时返回错误。
func ParseBuildVersion() (semver.Version, error) {
	s := strings.TrimSpace(strings.TrimPrefix(Version, "v"))
	return semver.Parse(s)
}
