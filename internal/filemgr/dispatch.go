package filemgr

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// 与控制面 ws.FileDispatchPayload / FileReportPayload JSON 字段保持一致。
const (
	MaxReadBytes   = 4 << 20
	MaxListEntries = 1000
)

type DispatchPayload struct {
	Op         string `json:"op"`
	Path       string `json:"path"`
	PathTo     string `json:"path_to,omitempty"`
	ContentB64 string `json:"content_b64,omitempty"`
	MaxBytes   int    `json:"max_bytes,omitempty"`
	Recursive  bool   `json:"recursive,omitempty"`
}

type FileEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

type ReportPayload struct {
	OK         bool        `json:"ok"`
	Error      string      `json:"error,omitempty"`
	Entries    []FileEntry `json:"entries,omitempty"`
	ContentB64 string      `json:"content_b64,omitempty"`
	Size       int64       `json:"size,omitempty"`
	ModTime    string      `json:"mod_time,omitempty"`
	IsDir      bool        `json:"is_dir,omitempty"`
}

// Executor 在 Agent 本地执行文件操作，路径必须落在配置的 roots 之下。
type Executor struct {
	roots []string
}

// NewExecutor roots 为空时使用 OS 默认根（Unix: /，Windows: C:\）。
func NewExecutor(roots []string) *Executor {
	return &Executor{roots: normalizeRoots(roots)}
}

func DefaultRoots() []string {
	if runtime.GOOS == "windows" {
		return []string{`C:\`}
	}
	return []string{"/"}
}

func normalizeRoots(in []string) []string {
	if len(in) == 0 {
		return DefaultRoots()
	}
	out := make([]string, 0, len(in))
	for _, r := range in {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		abs, err := filepath.Abs(r)
		if err != nil {
			continue
		}
		out = append(out, filepath.Clean(abs))
	}
	if len(out) == 0 {
		return DefaultRoots()
	}
	return out
}

func underRoot(path, root string) bool {
	if root == "" {
		return false
	}
	ap, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	r, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	if ap == r {
		return true
	}
	sep := string(os.PathSeparator)
	return strings.HasPrefix(ap, r+sep)
}

func (e *Executor) resolvePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errMsg("empty path")
	}
	abs, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return "", err
	}
	for _, root := range e.roots {
		if underRoot(abs, root) {
			return abs, nil
		}
	}
	return "", errMsg("path not allowed by file_manager_roots")
}

func errMsg(s string) error { return &pathError{s} }

type pathError struct{ s string }

func (e *pathError) Error() string { return e.s }

// Execute 执行控制面下发的单次文件操作。
func (e *Executor) Execute(p DispatchPayload) ReportPayload {
	op := strings.ToLower(strings.TrimSpace(p.Op))
	switch op {
	case "list":
		return e.opList(p.Path)
	case "stat":
		return e.opStat(p.Path)
	case "read":
		return e.opRead(p.Path, p.MaxBytes)
	case "write":
		return e.opWrite(p.Path, p.ContentB64)
	case "mkdir":
		return e.opMkdir(p.Path)
	case "remove":
		return e.opRemove(p.Path, p.Recursive)
	case "rename":
		return e.opRename(p.Path, p.PathTo)
	default:
		return ReportPayload{OK: false, Error: "unknown op: " + op}
	}
}

func (e *Executor) opList(raw string) ReportPayload {
	dir, err := e.resolvePath(raw)
	if err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	if len(entries) > MaxListEntries {
		entries = entries[:MaxListEntries]
	}
	out := make([]FileEntry, 0, len(entries))
	for _, de := range entries {
		info, err := de.Info()
		if err != nil {
			continue
		}
		out = append(out, FileEntry{
			Name:    de.Name(),
			IsDir:   de.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return ReportPayload{OK: true, Entries: out}
}

func (e *Executor) opStat(raw string) ReportPayload {
	path, err := e.resolvePath(raw)
	if err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	info, err := os.Stat(path)
	if err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	return ReportPayload{
		OK:      true,
		Size:    info.Size(),
		ModTime: info.ModTime().UTC().Format(time.RFC3339),
		IsDir:   info.IsDir(),
	}
}

func (e *Executor) opRead(raw string, maxBytes int) ReportPayload {
	path, err := e.resolvePath(raw)
	if err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	if maxBytes <= 0 || maxBytes > MaxReadBytes {
		maxBytes = MaxReadBytes
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	if len(b) > maxBytes {
		return ReportPayload{OK: false, Error: "file too large for read (max_bytes)"}
	}
	return ReportPayload{OK: true, ContentB64: base64.StdEncoding.EncodeToString(b), Size: int64(len(b))}
}

func (e *Executor) opWrite(raw, contentB64 string) ReportPayload {
	path, err := e.resolvePath(raw)
	if err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	rawBytes, err := base64.StdEncoding.DecodeString(contentB64)
	if err != nil {
		return ReportPayload{OK: false, Error: "invalid content_b64: " + err.Error()}
	}
	if err := os.WriteFile(path, rawBytes, 0o644); err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	return ReportPayload{OK: true, Size: int64(len(rawBytes))}
}

func (e *Executor) opMkdir(raw string) ReportPayload {
	path, err := e.resolvePath(raw)
	if err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	return ReportPayload{OK: true}
}

func (e *Executor) opRemove(raw string, recursive bool) ReportPayload {
	path, err := e.resolvePath(raw)
	if err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	if recursive {
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}
	if err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	return ReportPayload{OK: true}
}

func (e *Executor) opRename(fromRaw, toRaw string) ReportPayload {
	from, err := e.resolvePath(fromRaw)
	if err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	to, err := e.resolvePath(toRaw)
	if err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	if err := os.Rename(from, to); err != nil {
		return ReportPayload{OK: false, Error: err.Error()}
	}
	return ReportPayload{OK: true}
}

// RootDirs 返回规范化后的允许根目录（展示用）。
func (e *Executor) RootDirs() []string {
	out := make([]string, len(e.roots))
	copy(out, e.roots)
	return out
}
