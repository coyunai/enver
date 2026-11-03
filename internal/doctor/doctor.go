package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/enver-dev/enver/internal/platform"
	"github.com/enver-dev/enver/internal/venv"
	"github.com/enver-dev/enver/internal/vfox"
)

type Status string

const (
	OK   Status = "ok"
	Warn Status = "warn"
	Fail Status = "error"
	Info Status = "info"
)

// Check is one diagnostic result.
type Check struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
	Hint   string `json:"hint,omitempty"`
}

// Report is the result of a doctor run.
type Report struct {
	CheckedAt time.Time      `json:"checkedAt"`
	Checks    []Check        `json:"checks"`
	Summary   map[string]int `json:"summary"`
}

// Options are the inputs available to the diagnostics.
type Options struct {
	Vfox    *vfox.Manager
	VfoxErr error
	Venvs   []venv.Record
}

// Run executes all diagnostics.
func Run(opts Options) Report {
	r := Report{CheckedAt: time.Now(), Summary: map[string]int{}}
	add := func(c Check) {
		r.Checks = append(r.Checks, c)
		r.Summary[string(c.Status)]++
	}

	if opts.VfoxErr != nil || opts.Vfox == nil {
		add(Check{
			ID: "vfox", Title: "vfox 可用性", Status: Fail,
			Detail: opts.VfoxErr.Error(),
			Hint:   "安装 vfox 后重试：https://vfox.dev",
		})
		return r
	}
	m := opts.Vfox
	if m.Version == "" {
		add(Check{ID: "vfox", Title: "vfox 可用性", Status: Fail, Detail: "未检测到 vfox", Hint: "https://vfox.dev"})
		return r
	}
	if vfox.VersionAtLeast(m.Version, vfox.MinVersion) {
		add(Check{ID: "vfox", Title: "vfox 版本", Status: OK, Detail: "v" + m.Version})
	} else {
		add(Check{
			ID: "vfox", Title: "vfox 版本", Status: Fail,
			Detail: fmt.Sprintf("v%s 低于最低要求 v%s", m.Version, vfox.MinVersion),
			Hint:   "运行 vfox upgrade",
		})
	}

	if writable(m.Home) {
		add(Check{ID: "home", Title: "vfox 配置目录", Status: OK, Detail: m.Home})
	} else {
		add(Check{ID: "home", Title: "vfox 配置目录", Status: Fail, Detail: m.Home + " 不可写", Hint: "检查目录权限或设置 VFOX_HOME"})
	}

	root := m.SDKRoot()
	if writable(root) {
		add(Check{ID: "sdkroot", Title: "SDK 存储目录", Status: OK, Detail: root})
	} else {
		add(Check{ID: "sdkroot", Title: "SDK 存储目录", Status: Fail, Detail: root + " 不可写", Hint: "在设置中修改存储路径"})
	}
	if free, err := platform.DiskFree(root); err == nil {
		status := OK
		if free < 2<<30 {
			status = Warn
		}
		add(Check{ID: "disk", Title: "磁盘剩余空间", Status: status, Detail: humanBytes(free)})
	}

	for _, bin := range []string{"node", "python", "java", "go", "uv"} {
		path, err := exec.LookPath(bin)
		if err != nil {
			if bin == "uv" {
				add(Check{
					ID: "uv", Title: "uv", Status: Warn,
					Detail: "未在 PATH 中找到 uv",
					Hint:   "安装 uv 后可获得更快的 venv 创建与依赖同步；当前会回退到 python -m venv",
				})
			}
			continue
		}
		abs, _ := filepath.Abs(path)
		if bin == "uv" {
			add(Check{ID: "uv", Title: "uv", Status: OK, Detail: abs})
			continue
		}
		if isManaged(abs, m) {
			add(Check{ID: "path-" + bin, Title: "PATH · " + bin, Status: OK, Detail: abs})
		} else {
			add(Check{
				ID: "path-" + bin, Title: "PATH · " + bin, Status: Warn,
				Detail: "当前解析到 vfox 之外的解释器：" + abs,
				Hint:   "如需让 enver 完全接管，请检查系统 PATH 或卸载其他版本管理器",
			})
		}
	}

	broken := 0
	entries, _ := os.ReadDir(filepath.Join(m.Home, "sdks"))
	for _, e := range entries {
		link := filepath.Join(m.Home, "sdks", e.Name())
		target, err := os.Readlink(link)
		if err != nil {
			continue
		}
		if _, statErr := os.Stat(target); statErr != nil {
			broken++
		}
	}
	if broken > 0 {
		add(Check{
			ID: "symlinks", Title: "版本软链接", Status: Warn,
			Detail: fmt.Sprintf("%d 个软链接指向不存在的目录", broken),
			Hint:   "重新执行一次 vfox use -g <sdk>@<version>",
		})
	} else {
		add(Check{ID: "symlinks", Title: "版本软链接", Status: OK, Detail: "未发现断链"})
	}

	unhealthy := 0
	for _, record := range opts.Venvs {
		ok, reason := venv.Health(record)
		if !ok {
			unhealthy++
			add(Check{
				ID: "venv-" + record.ID, Title: "虚拟环境 · " + record.Name, Status: Warn,
				Detail: reason,
				Hint:   "在虚拟环境页面执行重建",
			})
		}
	}
	if len(opts.Venvs) > 0 && unhealthy == 0 {
		add(Check{ID: "venvs", Title: "虚拟环境", Status: OK, Detail: fmt.Sprintf("%d 个环境均正常", len(opts.Venvs))})
	}

	if hasShellHook() {
		add(Check{ID: "shell", Title: "Shell 集成", Status: OK, Detail: "检测到 vfox hook"})
	} else {
		add(Check{
			ID: "shell", Title: "Shell 集成", Status: Info,
			Detail: "外部终端尚未接入 vfox 自动切换",
			Hint:   "enver 内嵌终端不受影响；如需在系统终端自动切换，可执行 vfox 的 shell hook 安装",
		})
	}
	return r
}

func writable(dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".enver-check-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	_ = os.Remove(name)
	return true
}

func isManaged(path string, m *vfox.Manager) bool {
	clean := filepath.Clean(path)
	roots := []string{filepath.Clean(m.SDKRoot()), filepath.Clean(m.Home)}
	for _, root := range roots {
		if root == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(clean), strings.ToLower(root)+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func hasShellHook() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	var files []string
	if runtime.GOOS == "windows" {
		docs := filepath.Join(home, "Documents")
		files = []string{
			filepath.Join(docs, "PowerShell", "Microsoft.PowerShell_profile.ps1"),
			filepath.Join(docs, "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
		}
	} else {
		files = []string{
			filepath.Join(home, ".bashrc"),
			filepath.Join(home, ".zshrc"),
			filepath.Join(home, ".config", "fish", "config.fish"),
		}
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err == nil && strings.Contains(strings.ToLower(string(data)), "vfox") {
			return true
		}
	}
	return false
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
