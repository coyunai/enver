package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/enver-dev/enver/internal/buildinfo"
	"github.com/enver-dev/enver/internal/config"
	"github.com/enver-dev/enver/internal/doctor"
	"github.com/enver-dev/enver/internal/fsutil"
	"github.com/enver-dev/enver/internal/jobs"
	"github.com/enver-dev/enver/internal/packages"
	"github.com/enver-dev/enver/internal/platform"
	"github.com/enver-dev/enver/internal/preset"
	"github.com/enver-dev/enver/internal/run"
	"github.com/enver-dev/enver/internal/venv"
	"github.com/enver-dev/enver/internal/vfox"
)

// App wires the application services together.
type App struct {
	Config  *config.Store
	Presets *preset.Store
	Venvs   *venv.Store
	Jobs    *jobs.Manager

	DataDir  string
	CacheDir string
	Started  time.Time

	mu      sync.RWMutex
	vfoxMgr *vfox.Manager
	vfoxErr error

	// opMu serializes storage migrations against every vfox write operation.
	opMu sync.RWMutex
}

// New loads configuration and detects the local vfox installation.
func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	dataDir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	cacheDir, err := config.CacheDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	a := &App{
		Config:   cfg,
		DataDir:  dataDir,
		CacheDir: cacheDir,
		Jobs:     jobs.NewManager(),
		Started:  time.Now(),
	}
	presetPath := filepath.Join(dataDir, "presets.json")
	presets, err := preset.Load(presetPath)
	if err != nil {
		_ = os.Rename(presetPath, presetPath+".corrupt-"+time.Now().Format("20060102-150405"))
		presets, _ = preset.Load(presetPath)
	}
	a.Presets = presets
	venvPath := filepath.Join(dataDir, "venvs.json")
	venvs, err := venv.Load(venvPath)
	if err != nil {
		_ = os.Rename(venvPath, venvPath+".corrupt-"+time.Now().Format("20060102-150405"))
		venvs, _ = venv.Load(venvPath)
	}
	a.Venvs = venvs
	a.ReloadVfox()
	return a, nil
}

// ReloadVfox re-detects vfox using the configured binary override.
func (a *App) ReloadVfox() {
	data := a.Config.Data()
	mgr, err := vfox.Detect(data.VfoxBin)
	a.mu.Lock()
	a.vfoxMgr = mgr
	a.vfoxErr = err
	a.mu.Unlock()
}

// VfoxState returns the adapter or the detection error.
func (a *App) VfoxState() (*vfox.Manager, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.vfoxErr != nil {
		return nil, a.vfoxErr
	}
	if a.vfoxMgr == nil {
		return nil, errors.New("vfox is not available")
	}
	return a.vfoxMgr, nil
}

// VfoxErr reports why vfox could not be detected, if at all.
func (a *App) VfoxErr() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.vfoxErr
}

// SetVfoxBin persists a custom vfox binary path and reloads the adapter.
func (a *App) SetVfoxBin(bin string) error {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	if err := a.Config.Update(func(d *config.Data) { d.VfoxBin = strings.TrimSpace(bin) }); err != nil {
		return err
	}
	a.ReloadVfox()
	_, err := a.VfoxState()
	return err
}

// Overview is the dashboard payload.
type Overview struct {
	Vfox         *vfox.Info  `json:"vfox,omitempty"`
	VfoxError    string      `json:"vfoxError,omitempty"`
	SDKs         []vfox.SDK  `json:"sdks"`
	PresetCount  int         `json:"presetCount"`
	VenvCount    int         `json:"venvCount"`
	Storage      StorageInfo `json:"storage"`
	Platform     string      `json:"platform"`
	SystemPython string      `json:"systemPython,omitempty"`
	EnverVersion string      `json:"enverVersion"`
}

func (a *App) Overview(ctx context.Context) Overview {
	ov := Overview{
		PresetCount:  len(a.Presets.List()),
		VenvCount:    len(a.Venvs.List()),
		Storage:      a.Storage(),
		Platform:     runtime.GOOS,
		SystemPython: detectSystemPython(),
		EnverVersion: buildinfo.Version,
	}
	mgr, err := a.VfoxState()
	if err != nil {
		ov.VfoxError = err.Error()
		return ov
	}
	info := mgr.Info()
	ov.Vfox = &info
	if sdks, scanErr := mgr.Installed(ctx); scanErr == nil {
		ov.SDKs = sdks
	}
	return ov
}

// SDKs lists installed runtimes.
func (a *App) SDKs(ctx context.Context) ([]vfox.SDK, error) {
	mgr, err := a.VfoxState()
	if err != nil {
		return nil, err
	}
	return mgr.Installed(ctx)
}

// SearchVersions lists available versions for an SDK.
func (a *App) SearchVersions(ctx context.Context, sdk string) ([]vfox.Version, error) {
	if strings.TrimSpace(sdk) == "" {
		return nil, errors.New("sdk is required")
	}
	mgr, err := a.VfoxState()
	if err != nil {
		return nil, err
	}
	return mgr.Search(ctx, sdk)
}

// StartInstall begins an install job.
func (a *App) StartInstall(sdk, version string) (*jobs.Job, error) {
	mgr, err := a.VfoxState()
	if err != nil {
		return nil, err
	}
	if sdk == "" || version == "" {
		return nil, errors.New("sdk and version are required")
	}
	title := fmt.Sprintf("安装 %s@%s", sdk, version)
	return a.Jobs.Start("install", title, func(ctx context.Context, j *jobs.Job) error {
		a.opMu.RLock()
		defer a.opMu.RUnlock()
		j.Append("vfox install " + sdk + "@" + version)
		if err := mgr.Install(ctx, sdk, version, j.Append); err != nil {
			return err
		}
		j.Append("安装完成")
		return nil
	}), nil
}

// StartUninstall begins an uninstall job.
func (a *App) StartUninstall(sdk, version string) (*jobs.Job, error) {
	mgr, err := a.VfoxState()
	if err != nil {
		return nil, err
	}
	title := fmt.Sprintf("卸载 %s@%s", sdk, version)
	return a.Jobs.Start("uninstall", title, func(ctx context.Context, j *jobs.Job) error {
		a.opMu.RLock()
		defer a.opMu.RUnlock()
		j.Append("vfox uninstall " + sdk + "@" + version)
		if err := mgr.Uninstall(ctx, sdk, version, j.Append); err != nil {
			return err
		}
		j.Append("卸载完成")
		return nil
	}), nil
}

// StartAddPlugin begins a plugin install job.
func (a *App) StartAddPlugin(sdk string) (*jobs.Job, error) {
	mgr, err := a.VfoxState()
	if err != nil {
		return nil, err
	}
	title := "添加插件 " + sdk
	return a.Jobs.Start("plugin", title, func(ctx context.Context, j *jobs.Job) error {
		a.opMu.RLock()
		defer a.opMu.RUnlock()
		j.Append("vfox add " + sdk)
		if err := mgr.AddPlugin(ctx, sdk, j.Append); err != nil {
			return err
		}
		j.Append("插件已就绪")
		return nil
	}), nil
}

// Use switches a runtime version.
func (a *App) Use(ctx context.Context, sdk, version, scope, dir string, log func(string)) error {
	mgr, err := a.VfoxState()
	if err != nil {
		return err
	}
	if scope == "project" {
		info, statErr := os.Stat(dir)
		if statErr != nil || !info.IsDir() {
			return fmt.Errorf("project directory does not exist: %s", dir)
		}
	}
	a.opMu.RLock()
	defer a.opMu.RUnlock()
	return mgr.Use(ctx, sdk, version, scope, dir, log)
}

// ApplyPreset switches every runtime in a preset.
func (a *App) ApplyPreset(ctx context.Context, name, scope, dir string, log func(string)) error {
	p, ok := a.Presets.Get(name)
	if !ok {
		return fmt.Errorf("preset %q not found", name)
	}
	mgr, err := a.VfoxState()
	if err != nil {
		return err
	}
	if scope == "project" {
		info, statErr := os.Stat(dir)
		if statErr != nil || !info.IsDir() {
			return fmt.Errorf("project directory does not exist: %s", dir)
		}
	}
	a.opMu.RLock()
	defer a.opMu.RUnlock()
	installed, err := mgr.Installed(ctx)
	if err != nil {
		return err
	}
	have := map[string]map[string]bool{}
	for _, s := range installed {
		have[s.Name] = map[string]bool{}
		for _, v := range s.Versions {
			have[s.Name][v.Version] = true
		}
	}
	names := make([]string, 0, len(p.Runtimes))
	for sdk := range p.Runtimes {
		names = append(names, sdk)
	}
	sort.Strings(names)
	var missing []string
	for _, sdk := range names {
		if !have[sdk][p.Runtimes[sdk]] {
			missing = append(missing, sdk+"@"+p.Runtimes[sdk])
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("以下版本尚未安装：%s", strings.Join(missing, ", "))
	}
	for _, sdk := range names {
		version := p.Runtimes[sdk]
		if log != nil {
			log(fmt.Sprintf("切换 %s@%s", sdk, version))
		}
		if err := mgr.Use(ctx, sdk, version, scope, dir, log); err != nil {
			return fmt.Errorf("%s@%s: %w", sdk, version, err)
		}
	}
	_ = a.Config.Update(func(d *config.Data) { d.LastPreset = name })
	if log != nil {
		log("预设已应用")
	}
	return nil
}

// CreateVenvRequest describes a new Python environment.
type CreateVenvRequest struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	ProjectDir    string `json:"projectDir"`
	PythonVersion string `json:"pythonVersion"`
	PythonPath    string `json:"pythonPath"`
	Tool          string `json:"tool"`
	Preset        string `json:"preset"`
}

// CreateVenv creates and records a virtual environment.
func (a *App) CreateVenv(ctx context.Context, req CreateVenvRequest, log func(string)) (venv.Record, error) {
	var pythonVersion string
	var basePython string
	explicit := strings.TrimSpace(req.PythonPath)
	if explicit != "" {
		if !filepath.IsAbs(explicit) {
			return venv.Record{}, errors.New("外部解释器路径必须是绝对路径")
		}
		info, statErr := os.Stat(explicit)
		if statErr != nil || info.IsDir() {
			return venv.Record{}, fmt.Errorf("外部解释器不存在: %s", explicit)
		}
		basePython = filepath.Clean(explicit)
		version, versionErr := pythonVersionOf(ctx, basePython)
		if versionErr != nil {
			return venv.Record{}, versionErr
		}
		pythonVersion = version
	} else {
		mgr, err := a.VfoxState()
		if err != nil {
			return venv.Record{}, err
		}
		sdks, err := mgr.Installed(ctx)
		if err != nil {
			return venv.Record{}, err
		}
		var pySDK *vfox.SDK
		for i := range sdks {
			if sdks[i].Name == "python" {
				pySDK = &sdks[i]
				break
			}
		}
		if pySDK == nil || len(pySDK.Versions) == 0 {
			return venv.Record{}, errors.New("没有已安装的 Python 版本，请先在 SDK 页面安装，或填写外部解释器路径")
		}
		wanted := strings.TrimSpace(req.PythonVersion)
		if wanted == "" {
			wanted = pySDK.Current
		}
		var selected *vfox.Version
		for i := range pySDK.Versions {
			if wanted == "" || pySDK.Versions[i].Version == wanted {
				selected = &pySDK.Versions[i]
				break
			}
		}
		if selected == nil {
			return venv.Record{}, fmt.Errorf("Python %s 尚未安装", wanted)
		}
		basePython = venv.FindPython(selected.Path)
		if basePython == "" {
			return venv.Record{}, errors.New("该 Python 版本中未找到解释器")
		}
		pythonVersion = selected.Version
	}

	target := strings.TrimSpace(req.Path)
	if target == "" {
		if strings.TrimSpace(req.ProjectDir) != "" {
			target = filepath.Join(req.ProjectDir, ".venv")
		} else {
			name := strings.TrimSpace(req.Name)
			if name == "" {
				name = "venv"
			}
			target = filepath.Join(a.DataDir, "venvs", sanitize(name))
		}
	} else if !filepath.IsAbs(target) {
		if strings.TrimSpace(req.ProjectDir) == "" {
			return venv.Record{}, errors.New("相对路径需要同时提供项目目录")
		}
		target = filepath.Join(req.ProjectDir, target)
	}
	target = filepath.Clean(target)
	if err := ensureEmptyDir(target); err != nil {
		return venv.Record{}, err
	}
	tool, err := chooseTool(req.Tool)
	if err != nil {
		return venv.Record{}, err
	}
	if log != nil {
		log(fmt.Sprintf("使用 %s 创建 %s", tool, target))
	}
	if err := createVenv(ctx, tool, basePython, target, log); err != nil {
		return venv.Record{}, err
	}
	if _, err := os.Stat(venv.PythonExe(target)); err != nil {
		return venv.Record{}, errors.New("虚拟环境已创建，但未找到解释器，请检查日志")
	}
	record := venv.Record{
		Name:          strings.TrimSpace(req.Name),
		Path:          target,
		PythonPath:    basePython,
		PythonVersion: pythonVersion,
		Tool:          tool,
		ProjectDir:    strings.TrimSpace(req.ProjectDir),
		Preset:        strings.TrimSpace(req.Preset),
	}
	if record.Name == "" {
		record.Name = filepath.Base(target)
	}
	if err := a.Venvs.Add(&record); err != nil {
		return venv.Record{}, err
	}
	if log != nil {
		log("虚拟环境已就绪")
	}
	return record, nil
}

// RebuildVenv recreates an existing environment with the same settings.
func (a *App) RebuildVenv(ctx context.Context, id string, log func(string)) (venv.Record, error) {
	old, ok := a.Venvs.Get(id)
	if !ok {
		return venv.Record{}, venv.ErrNotFound
	}
	if err := os.RemoveAll(old.Path); err != nil {
		return venv.Record{}, err
	}
	record, err := a.CreateVenv(ctx, CreateVenvRequest{
		Name:          old.Name,
		Path:          old.Path,
		ProjectDir:    old.ProjectDir,
		PythonVersion: old.PythonVersion,
		PythonPath:    old.PythonPath,
		Tool:          old.Tool,
		Preset:        old.Preset,
	}, log)
	if err != nil {
		return venv.Record{}, err
	}
	_ = a.Venvs.Delete(id)
	return record, nil
}

// DeleteVenv removes an environment from disk and from the registry.
func (a *App) DeleteVenv(id string) error {
	record, ok := a.Venvs.Get(id)
	if !ok {
		return venv.ErrNotFound
	}
	clean := filepath.Clean(record.Path)
	if clean == "" || clean == string(os.PathSeparator) || clean == "." {
		return errors.New("refusing to delete an unsafe path")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.EqualFold(clean, filepath.Clean(home)) {
		return errors.New("refusing to delete the home directory")
	}
	if err := os.RemoveAll(clean); err != nil {
		return err
	}
	return a.Venvs.Delete(id)
}

// VenvView is a record with health information.
type VenvView struct {
	venv.Record
	Healthy   bool   `json:"healthy"`
	Health    string `json:"health,omitempty"`
	PythonExe string `json:"pythonExe"`
}

func (a *App) VenvViews() []VenvView {
	records := a.Venvs.List()
	out := make([]VenvView, 0, len(records))
	for _, r := range records {
		ok, reason := venv.Health(r)
		out = append(out, VenvView{
			Record:    r,
			Healthy:   ok,
			Health:    reason,
			PythonExe: venv.PythonExe(r.Path),
		})
	}
	return out
}

// Packages lists globally installed libraries for a runtime version.
func (a *App) Packages(ctx context.Context, sdk, version string) ([]packages.Package, error) {
	path, _, err := a.versionPath(ctx, sdk, version)
	if err != nil {
		return nil, err
	}
	return packages.List(ctx, sdk, path)
}

// ExportPackages renders a portable manifest for the selected runtime.
func (a *App) ExportPackages(ctx context.Context, sdk, version string) (packages.Export, error) {
	pkgs, err := a.Packages(ctx, sdk, version)
	if err != nil {
		return packages.Export{}, err
	}
	return packages.BuildExport(sdk, pkgs), nil
}

func (a *App) versionPath(ctx context.Context, sdk, version string) (string, string, error) {
	mgr, err := a.VfoxState()
	if err != nil {
		return "", "", err
	}
	sdks, err := mgr.Installed(ctx)
	if err != nil {
		return "", "", err
	}
	for _, s := range sdks {
		if s.Name != sdk {
			continue
		}
		wanted := version
		if wanted == "" {
			wanted = s.Current
		}
		for _, v := range s.Versions {
			if wanted == "" || v.Version == wanted {
				return v.Path, v.Version, nil
			}
		}
		return "", "", fmt.Errorf("%s@%s 尚未安装", sdk, wanted)
	}
	return "", "", fmt.Errorf("SDK %s 未安装", sdk)
}

// Doctor runs diagnostics.
func (a *App) Doctor() doctor.Report {
	mgr, err := a.VfoxState()
	return doctor.Run(doctor.Options{Vfox: mgr, VfoxErr: err, Venvs: a.Venvs.List()})
}

// StorageInfo describes where vfox keeps its state.
type StorageInfo struct {
	Home       string `json:"home"`
	SDKRoot    string `json:"sdkRoot"`
	Registry   string `json:"registry"`
	ConfigPath string `json:"configPath"`
	Version    string `json:"version"`
	FreeBytes  uint64 `json:"freeBytes"`
	Platform   string `json:"platform"`
}

func (a *App) Storage() StorageInfo {
	home := vfox.HomePath()
	info := StorageInfo{Home: home, Platform: runtime.GOOS}
	if mgr, err := a.VfoxState(); err == nil {
		meta := mgr.Info()
		info.SDKRoot = meta.SDKRoot
		info.Registry = meta.Registry
		info.ConfigPath = meta.ConfigPath
		info.Version = meta.Version
	}
	probe := info.SDKRoot
	if probe == "" {
		probe = home
	}
	if free, err := platform.DiskFree(probe); err == nil {
		info.FreeBytes = free
	}
	return info
}

// SetStoragePath only changes the vfox SDK root, without moving files.
func (a *App) SetStoragePath(ctx context.Context, path string) error {
	mgr, err := a.VfoxState()
	if err != nil {
		return err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("storage path is required")
	}
	if !filepath.IsAbs(path) {
		return errors.New("storage path must be absolute")
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	a.opMu.Lock()
	defer a.opMu.Unlock()
	return mgr.ConfigSet(ctx, "storage.sdkPath", filepath.Clean(path))
}

// StartMigrate moves installed SDKs to a new root in the background.
func (a *App) StartMigrate(ctx context.Context, to string, deleteOld bool) (*jobs.Job, error) {
	mgr, err := a.VfoxState()
	if err != nil {
		return nil, err
	}
	title := "迁移 SDK 存储目录"
	return a.Jobs.Start("storage", title, func(jobCtx context.Context, j *jobs.Job) error {
		a.opMu.Lock()
		defer a.opMu.Unlock()
		from := mgr.SDKRoot()
		return a.migrate(jobCtx, mgr, from, to, deleteOld, j.Append)
	}), nil
}

func (a *App) migrate(ctx context.Context, mgr *vfox.Manager, from, to string, deleteOld bool, log func(string)) error {
	to = strings.TrimSpace(to)
	if !filepath.IsAbs(to) {
		return errors.New("目标路径必须是绝对路径")
	}
	to = filepath.Clean(to)
	from = filepath.Clean(from)
	if strings.EqualFold(from, to) {
		return errors.New("目标路径与当前路径相同")
	}
	if isInside(to, from) {
		return errors.New("目标路径不能位于当前 SDK 目录内部")
	}
	if isInside(from, to) {
		return errors.New("目标路径不能是当前 SDK 目录的上级目录")
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	if err := ensureWritable(to); err != nil {
		return fmt.Errorf("目标目录不可写: %w", err)
	}
	size, _ := fsutil.DirSize(from)
	if free, err := platform.DiskFree(to); err == nil && free < uint64(size)+(256<<20) {
		return fmt.Errorf("目标磁盘空间不足：需要约 %s，可用 %s", humanBytes(uint64(size)), humanBytes(free))
	}

	sdks, _ := mgr.Installed(ctx)
	current := map[string]string{}
	for _, s := range sdks {
		if s.Current != "" {
			current[s.Name] = s.Current
		}
	}
	log(fmt.Sprintf("从 %s 迁移到 %s", from, to))
	log(fmt.Sprintf("已安装 SDK %d 个，预计复制 %s", len(sdks), humanBytes(uint64(size))))

	if _, err := os.Stat(from); err == nil {
		lastPct := -1
		err := fsutil.CopyTree(ctx, from, to, func(p fsutil.CopyProgress) {
			pct := 0
			if p.TotalBytes > 0 {
				pct = int(p.Bytes * 100 / p.TotalBytes)
			} else if p.TotalFiles > 0 {
				pct = int(p.Files * 100 / p.TotalFiles)
			}
			if pct != lastPct && pct%2 == 0 {
				lastPct = pct
				log(fmt.Sprintf("复制中 %d%% (%d/%d 文件)", pct, p.Files, p.TotalFiles))
			}
		})
		if err != nil {
			return fmt.Errorf("复制失败，vfox 配置未修改: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// vfox 1.0.2-1.0.7 stored SDKs under <sdkPath>/cache/<sdk>. Normalize a
	// copied legacy tree so the current layout expects <sdkPath>/<sdk>.
	legacy := filepath.Join(to, "cache")
	if entries, readErr := os.ReadDir(legacy); readErr == nil {
		for _, entry := range entries {
			src := filepath.Join(legacy, entry.Name())
			dst := filepath.Join(to, entry.Name())
			if _, statErr := os.Stat(dst); statErr == nil {
				log("警告：新旧布局同时存在，保留 " + dst)
				continue
			}
			if renameErr := os.Rename(src, dst); renameErr != nil {
				log(fmt.Sprintf("警告：无法规范化旧布局 %s: %v", src, renameErr))
				continue
			}
		}
		_ = os.Remove(legacy)
		log("已规范化旧版 cache/ 布局")
	}

	if err := mgr.ConfigSet(ctx, "storage.sdkPath", to); err != nil {
		return fmt.Errorf("写入 vfox 配置失败: %w", err)
	}
	log("vfox 配置已更新")

	names := make([]string, 0, len(current))
	for name := range current {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		version := current[name]
		log(fmt.Sprintf("重新链接 %s@%s", name, version))
		if err := mgr.Use(ctx, name, version, "global", "", log); err != nil {
			log(fmt.Sprintf("警告：重新链接 %s 失败：%v", name, err))
		}
	}

	for _, s := range sdks {
		for _, v := range s.Versions {
			newPath := filepath.Join(to, s.Name, "v-"+v.Version)
			if _, err := os.Stat(newPath); err != nil {
				return fmt.Errorf("校验失败：%s 不存在，旧目录已保留", newPath)
			}
		}
	}
	log("迁移完成，已校验新目录")

	if deleteOld {
		if strings.EqualFold(filepath.Clean(from), filepath.Clean(mgr.Home)) {
			log("警告：旧目录是 vfox 主目录，已跳过删除")
		} else if err := os.RemoveAll(from); err != nil {
			log("警告：无法删除旧目录：" + err.Error())
		} else {
			log("已删除旧目录 " + from)
		}
	} else {
		log("旧目录已保留：" + from)
	}
	return nil
}

// TerminalEnv builds the environment for an interactive shell.
func (a *App) TerminalEnv(ctx context.Context, presetName, venvID, cwd string) ([]string, error) {
	paths := []string{}
	extra := map[string]string{}
	venvPath := ""

	if presetName != "" {
		if p, ok := a.Presets.Get(presetName); ok {
			if mgr, err := a.VfoxState(); err == nil {
				if sdks, scanErr := mgr.Installed(ctx); scanErr == nil {
					byName := map[string]vfox.SDK{}
					for _, s := range sdks {
						byName[s.Name] = s
					}
					names := make([]string, 0, len(p.Runtimes))
					for sdk := range p.Runtimes {
						names = append(names, sdk)
					}
					sort.Strings(names)
					for _, sdk := range names {
						s, ok := byName[sdk]
						if !ok {
							continue
						}
						for _, v := range s.Versions {
							if v.Version == p.Runtimes[sdk] {
								paths = append(paths, vfox.BinDir(sdk, v.Path))
								break
							}
						}
					}
				}
			}
			for k, v := range p.Env {
				extra[k] = v
			}
			if p.Python != nil && p.Python.Venv != "" {
				candidate := p.Python.Venv
				if !filepath.IsAbs(candidate) {
					if cwd == "" {
						candidate = ""
					} else {
						candidate = filepath.Join(cwd, candidate)
					}
				}
				if candidate != "" {
					if info, err := os.Stat(candidate); err == nil && info.IsDir() {
						venvPath = candidate
					}
				}
			}
		}
	}
	if venvID != "" {
		if record, ok := a.Venvs.Get(venvID); ok {
			venvPath = record.Path
		}
	}
	if venvPath != "" {
		paths = append(paths, venv.BinDir(venvPath))
		extra["VIRTUAL_ENV"] = venvPath
	}

	mgr, err := a.VfoxState()
	if err != nil {
		base := vfox.PrependPaths(os.Environ(), paths)
		base = vfox.MergeEnv(base, extra)
		if venvPath != "" {
			base = removeEnv(base, "PYTHONHOME")
		}
		return base, nil
	}
	env, err := mgr.Environment(ctx, paths, extra)
	if err != nil {
		return nil, err
	}
	if venvPath != "" {
		env = removeEnv(env, "PYTHONHOME")
	}
	return env, nil
}

func createVenv(ctx context.Context, tool, basePython, target string, log func(string)) error {
	switch tool {
	case "uv":
		uv, err := exec.LookPath("uv")
		if err != nil {
			return errors.New("uv 未安装，请改用 python -m venv 或先安装 uv")
		}
		env := vfox.MergeEnv(os.Environ(), map[string]string{"UV_PYTHON_DOWNLOADS": "never"})
		args := []string{"venv", "--python", basePython, target}
		if err := run.Command(ctx, uv, args, "", env, log); err != nil {
			return fmt.Errorf("uv venv 失败: %w", err)
		}
		return nil
	default:
		if err := run.Command(ctx, basePython, []string{"-m", "venv", target}, "", os.Environ(), log); err != nil {
			return fmt.Errorf("python -m venv 失败: %w", err)
		}
		return nil
	}
}

func chooseTool(requested string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(requested)) {
	case "", "auto":
		if _, err := exec.LookPath("uv"); err == nil {
			return "uv", nil
		}
		return "venv", nil
	case "uv":
		if _, err := exec.LookPath("uv"); err != nil {
			return "", errors.New("未找到 uv，请先安装 uv 或选择 stdlib venv")
		}
		return "uv", nil
	case "venv", "stdlib":
		return "venv", nil
	default:
		return "", fmt.Errorf("未知的隔离工具 %q", requested)
	}
}

func detectSystemPython() string {
	candidates := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		candidates = []string{"python.exe", "python3.exe", "python", "python3"}
	}
	fallback := ""
	for _, name := range candidates {
		p, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(p))
		if runtime.GOOS == "windows" && (ext == ".bat" || ext == ".cmd") {
			if fallback == "" {
				fallback = p
			}
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return p
		}
		return abs
	}
	return fallback
}

func pythonVersionOf(ctx context.Context, exe string) (string, error) {
	c, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := run.Output(c, exe, []string{"-c", "import sys; print(sys.version.split()[0])"}, "", nil)
	if err != nil {
		return "", fmt.Errorf("无法读取 Python 版本: %s", strings.TrimSpace(out))
	}
	line := strings.TrimSpace(strings.Split(out, "\n")[0])
	if line == "" {
		return "", errors.New("无法读取 Python 版本")
	}
	return line, nil
}

func ensureEmptyDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.MkdirAll(path, 0o755)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s 已存在且不是目录", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s 已存在且非空", path)
	}
	return nil
}

func ensureWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".enver-write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}

func isInside(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != ".."
}

func sanitize(name string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	out := strings.TrimSpace(replacer.Replace(name))
	if out == "" || out == "." || out == ".." {
		return "venv"
	}
	return out
}

func removeEnv(env []string, key string) []string {
	out := env[:0]
	for _, kv := range env {
		idx := strings.IndexByte(kv, '=')
		if idx > 0 && strings.EqualFold(kv[:idx], key) {
			continue
		}
		out = append(out, kv)
	}
	return out
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
