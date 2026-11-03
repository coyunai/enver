package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/coyunai/enver/internal/app"
	"github.com/coyunai/enver/internal/buildinfo"
	"github.com/coyunai/enver/internal/httpapi"
	"github.com/coyunai/enver/internal/jobs"
)

var jsonOutput bool

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "enver:", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "enver",
		Short:         "Cross-platform environment orchestration built on vfox",
		Long:          "enver composes vfox runtimes into switchable environments, manages Python virtual environments and inspects globally installed packages.",
		Version:       buildinfo.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUI(7788, true, true, "127.0.0.1", "")
		},
	}
	root.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output machine readable JSON")

	root.AddCommand(newUICommand())
	root.AddCommand(newDoctorCommand())
	root.AddCommand(newSDKCommand())
	root.AddCommand(newInstallCommand())
	root.AddCommand(newUninstallCommand())
	root.AddCommand(newUseCommand())
	root.AddCommand(newPresetCommand())
	root.AddCommand(newVenvCommand())
	root.AddCommand(newPackageCommand())
	root.AddCommand(newEnvCommand())
	root.AddCommand(newStorageCommand())
	return root
}

func newUICommand() *cobra.Command {
	var port int
	var noOpen bool
	var noAuth bool
	var bind string
	var fixedToken string
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Start the local web UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUI(port, !noOpen, !noAuth, bind, fixedToken)
		},
	}
	cmd.Flags().IntVar(&port, "port", 7788, "port to listen on")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "do not open the browser")
	cmd.Flags().BoolVar(&noAuth, "no-auth", false, "disable the session token (local development only)")
	cmd.Flags().StringVar(&bind, "bind", "127.0.0.1", "address to bind")
	cmd.Flags().StringVar(&fixedToken, "token", "", "use a fixed session token (automation)")
	return cmd
}

func runUI(port int, open, auth bool, bind, fixedToken string) error {
	a, err := app.New()
	if err != nil {
		return err
	}
	token := randomToken()
	if fixedToken != "" {
		token = fixedToken
	}
	server := httpapi.New(a, token, auth)
	ln, err := listen(bind, port)
	if err != nil {
		return err
	}
	actualPort := port
	if addr, ok := ln.Addr().(*net.TCPAddr); ok {
		actualPort = addr.Port
	}
	host := bind
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	url := fmt.Sprintf("http://%s:%d", host, actualPort)
	if auth {
		url += "/auth?token=" + token
	}
	fmt.Printf("enver %s\n", buildinfo.Version)
	fmt.Printf("UI  %s\n", url)
	if a.VfoxErr() != nil {
		fmt.Printf("vfox  %v\n", a.VfoxErr())
	}

	if open {
		openBrowser(url)
	}
	httpServer := &http.Server{
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		if serveErr := httpServer.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case serveErr := <-errCh:
		return serveErr
	case <-sigCh:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return httpServer.Shutdown(ctx)
}

func listen(bind string, port int) (net.Listener, error) {
	attempts := 1
	if port == 7788 {
		attempts = 20
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		p := port + i
		ln, err := net.Listen("tcp", net.JoinHostPort(bind, strconv.Itoa(p)))
		if err == nil {
			return ln, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func randomToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func mustApp() *app.App {
	a, err := app.New()
	if err != nil {
		fatal(err)
	}
	return a
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "enver:", err)
	os.Exit(1)
}

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the local environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := mustApp()
			report := a.Doctor()
			if jsonOutput {
				return printJSON(report)
			}
			for _, c := range report.Checks {
				fmt.Printf("[%s] %s\n      %s\n", strings.ToUpper(string(c.Status)), c.Title, c.Detail)
				if c.Hint != "" {
					fmt.Printf("      -> %s\n", c.Hint)
				}
			}
			if report.Summary["error"] > 0 {
				return errors.New("doctor found blocking problems")
			}
			return nil
		},
	}
}

func newSDKCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "sdk", Short: "Inspect installed and available runtimes"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List installed runtimes",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := mustApp()
			sdks, err := a.SDKs(context.Background())
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(sdks)
			}
			for _, s := range sdks {
				for _, v := range s.Versions {
					marker := " "
					if v.Current {
						marker = "*"
					}
					fmt.Printf("%s %-10s %s\n", marker, s.Name, v.Version)
				}
				if len(s.Versions) == 0 {
					fmt.Printf("  %-10s (no versions)\n", s.Name)
				}
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "search <sdk>",
		Short: "List available versions for an SDK",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := mustApp()
			versions, err := a.SearchVersions(context.Background(), args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(versions)
			}
			for _, v := range versions {
				suffix := ""
				if v.Installed {
					suffix += " installed"
				}
				if v.LTS {
					suffix += " lts"
				}
				fmt.Printf("%-18s%s\n", v.Version, suffix)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "add <sdk>",
		Short: "Install a vfox plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := mustApp()
			job, err := a.StartAddPlugin(args[0])
			if err != nil {
				return err
			}
			return streamJob(job)
		},
	})
	return cmd
}

func newInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install <sdk>@<version>",
		Short: "Install a runtime version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sdk, version, err := parseTarget(args[0])
			if err != nil {
				return err
			}
			a := mustApp()
			job, err := a.StartInstall(sdk, version)
			if err != nil {
				return err
			}
			return streamJob(job)
		},
	}
}

func newUninstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <sdk>@<version>",
		Short: "Uninstall a runtime version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sdk, version, err := parseTarget(args[0])
			if err != nil {
				return err
			}
			a := mustApp()
			job, err := a.StartUninstall(sdk, version)
			if err != nil {
				return err
			}
			return streamJob(job)
		},
	}
}

func newUseCommand() *cobra.Command {
	var project bool
	var dir string
	cmd := &cobra.Command{
		Use:   "use <sdk>@<version>",
		Short: "Switch the active runtime version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sdk, version, err := parseTarget(args[0])
			if err != nil {
				return err
			}
			scope := "global"
			if project {
				scope = "project"
				if dir == "" {
					dir, _ = os.Getwd()
				}
			}
			a := mustApp()
			return a.Use(context.Background(), sdk, version, scope, dir, func(line string) {
				fmt.Println(line)
			})
		},
	}
	cmd.Flags().BoolVarP(&project, "project", "p", false, "apply to the current project instead of globally")
	cmd.Flags().StringVar(&dir, "dir", "", "project directory (defaults to the current directory)")
	return cmd
}

func newPresetCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "preset", Short: "Manage environment presets"}
	cmd.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "List presets",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := mustApp()
			presets := a.Presets.List()
			if jsonOutput {
				return printJSON(presets)
			}
			for _, p := range presets {
				fmt.Printf("%-16s %s\n", p.Name, formatRuntimes(p.Runtimes))
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show <name>",
		Short: "Show a preset",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := mustApp()
			p, ok := a.Presets.Get(args[0])
			if !ok {
				return fmt.Errorf("preset %q not found", args[0])
			}
			return printJSON(p)
		},
	})
	var scope string
	var dir string
	apply := &cobra.Command{
		Use:   "apply <name>",
		Short: "Apply a preset",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if scope == "project" && dir == "" {
				dir, _ = os.Getwd()
			}
			a := mustApp()
			return a.ApplyPreset(context.Background(), args[0], scope, dir, func(line string) {
				fmt.Println(line)
			})
		},
	}
	apply.Flags().StringVar(&scope, "scope", "global", "global or project")
	apply.Flags().StringVar(&dir, "dir", "", "project directory (defaults to the current directory)")
	cmd.AddCommand(apply)
	return cmd
}

func newVenvCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "venv", Short: "Manage Python virtual environments"}
	cmd.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "List virtual environments",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := mustApp()
			views := a.VenvViews()
			if jsonOutput {
				return printJSON(views)
			}
			for _, v := range views {
				state := "ok"
				if !v.Healthy {
					state = v.Health
				}
				fmt.Printf("%s  %-16s %-10s %s  [%s]\n", v.ID[:8], v.Name, v.PythonVersion, v.Path, state)
			}
			return nil
		},
	})
	var name, path, project, pythonVersion, pythonPath, tool, presetName string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a virtual environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := mustApp()
			record, err := a.CreateVenv(context.Background(), app.CreateVenvRequest{
				Name:          name,
				Path:          path,
				ProjectDir:    project,
				PythonVersion: pythonVersion,
				PythonPath:    pythonPath,
				Tool:          tool,
				Preset:        presetName,
			}, func(line string) {
				if !jsonOutput {
					fmt.Println(line)
				}
			})
			if err != nil {
				return err
			}
			return printJSON(record)
		},
	}
	create.Flags().StringVar(&name, "name", "", "display name")
	create.Flags().StringVar(&path, "path", "", "environment path")
	create.Flags().StringVar(&project, "project", "", "project directory")
	create.Flags().StringVar(&pythonVersion, "python", "", "base Python version")
	create.Flags().StringVar(&pythonPath, "python-path", "", "external base interpreter (compatibility mode)")
	create.Flags().StringVar(&tool, "tool", "", "uv or venv (default: auto)")
	create.Flags().StringVar(&presetName, "preset", "", "attach to a preset")
	cmd.AddCommand(create)
	cmd.AddCommand(&cobra.Command{
		Use:   "rm <id>",
		Short: "Delete a virtual environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return mustApp().DeleteVenv(args[0])
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "rebuild <id>",
		Short: "Rebuild a virtual environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			record, err := mustApp().RebuildVenv(context.Background(), args[0], func(line string) {
				if !jsonOutput {
					fmt.Println(line)
				}
			})
			if err != nil {
				return err
			}
			return printJSON(record)
		},
	})
	return cmd
}

func newPackageCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "pkg", Short: "Inspect globally installed libraries"}
	cmd.AddCommand(&cobra.Command{
		Use:   "ls <sdk>[@version]",
		Short: "List global packages for a runtime version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sdk, version := splitOptionalVersion(args[0])
			list, err := mustApp().Packages(context.Background(), sdk, version)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(list)
			}
			for _, p := range list {
				fmt.Printf("%-32s %s\n", p.Name, p.Version)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "export <sdk>[@version]",
		Short: "Export global packages as a manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sdk, version := splitOptionalVersion(args[0])
			export, err := mustApp().ExportPackages(context.Background(), sdk, version)
			if err != nil {
				return err
			}
			fmt.Print(export.Content)
			return nil
		},
	})
	return cmd
}

func newEnvCommand() *cobra.Command {
	var presetName, venvID, cwd string
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Print the environment for a preset or virtual environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cwd == "" {
				cwd, _ = os.Getwd()
			}
			env, err := mustApp().TerminalEnv(context.Background(), presetName, venvID, cwd)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(env)
			}
			for _, kv := range env {
				idx := strings.IndexByte(kv, '=')
				if idx <= 0 {
					continue
				}
				if runtime.GOOS == "windows" {
					fmt.Printf("$env:%s=%q\n", kv[:idx], kv[idx+1:])
				} else {
					fmt.Printf("export %s=%q\n", kv[:idx], kv[idx+1:])
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&presetName, "preset", "", "preset to apply")
	cmd.Flags().StringVar(&venvID, "venv", "", "virtual environment id")
	cmd.Flags().StringVar(&cwd, "cwd", "", "working directory for relative venv paths")
	return cmd
}

func newStorageCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "storage", Short: "Inspect and migrate the SDK storage directory"}
	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show storage locations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printJSON(mustApp().Storage())
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set <path>",
		Short: "Change the SDK storage path without moving files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := mustApp()
			if err := a.SetStoragePath(context.Background(), args[0]); err != nil {
				return err
			}
			return printJSON(a.Storage())
		},
	})
	var deleteOld bool
	migrate := &cobra.Command{
		Use:   "migrate <path>",
		Short: "Move installed SDKs to a new path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			job, err := mustApp().StartMigrate(context.Background(), args[0], deleteOld)
			if err != nil {
				return err
			}
			return streamJob(job)
		},
	}
	migrate.Flags().BoolVar(&deleteOld, "delete-old", false, "delete the old directory after verification")
	cmd.AddCommand(migrate)
	return cmd
}

func streamJob(job *jobs.Job) error {
	snapshot := job.Snapshot()
	history, ch := job.Subscribe()
	for _, e := range history {
		if e.Line != "" {
			fmt.Println(e.Line)
		}
	}
	switch snapshot.Status {
	case jobs.StatusSuccess:
		return nil
	case jobs.StatusFailed:
		return errors.New(snapshot.Error)
	case jobs.StatusCanceled:
		return errors.New("canceled")
	}
	for e := range ch {
		switch e.Type {
		case "log":
			fmt.Println(e.Line)
		case "done":
			if e.Status != jobs.StatusSuccess {
				if e.Error != "" {
					return errors.New(e.Error)
				}
				return fmt.Errorf("job %s", e.Status)
			}
			return nil
		}
	}
	return nil
}

func parseTarget(arg string) (string, string, error) {
	idx := strings.LastIndex(arg, "@")
	if idx <= 0 || idx == len(arg)-1 {
		return "", "", fmt.Errorf("expected <sdk>@<version>, got %q", arg)
	}
	return arg[:idx], arg[idx+1:], nil
}

func splitOptionalVersion(arg string) (string, string) {
	idx := strings.LastIndex(arg, "@")
	if idx <= 0 || idx == len(arg)-1 {
		return arg, ""
	}
	return arg[:idx], arg[idx+1:]
}

func formatRuntimes(r map[string]string) string {
	names := make([]string, 0, len(r))
	for k := range r {
		names = append(names, k)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"@"+r[name])
	}
	return strings.Join(parts, "  ")
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
