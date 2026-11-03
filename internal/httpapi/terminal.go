package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/aymanbagabas/go-pty"
	"github.com/gorilla/websocket"
)

type termMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 16384,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		return err == nil && strings.EqualFold(u.Host, r.Host)
	},
}

func (s *Server) terminal(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	cwd := strings.TrimSpace(query.Get("cwd"))
	if cwd == "" {
		cwd, _ = os.UserHomeDir()
	}
	if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		apiError(w, http.StatusBadRequest, "cwd_invalid", "working directory does not exist")
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	env, err := s.App.TerminalEnv(ctx, query.Get("preset"), query.Get("venv"), cwd)
	if err != nil {
		apiError(w, http.StatusBadRequest, "env_failed", err.Error())
		return
	}

	p, err := pty.New()
	if err != nil {
		apiError(w, http.StatusInternalServerError, "pty_failed", err.Error())
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		_ = p.Close()
		return
	}
	defer conn.Close()
	defer p.Close()

	shell, args := shellCommand()
	cmd := p.Command(shell, args...)
	cmd.Env = append(env, "TERM=xterm-256color", "COLORTERM=truecolor")
	cmd.Dir = cwd
	if err := cmd.Start(); err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n[enver] 启动 shell 失败: "+err.Error()+"\r\n"))
		return
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	cols := parseDimension(query.Get("cols"), 120)
	rows := parseDimension(query.Get("rows"), 30)
	_ = p.Resize(cols, rows)

	go func() {
		buf := make([]byte, 8192)
		for {
			n, readErr := p.Read(buf)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if readErr != nil {
				return
			}
		}
	}()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	conn.SetReadLimit(1 << 20)
	_ = conn.SetReadDeadline(time.Now().Add(24 * time.Hour))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(24 * time.Hour))
	})

	for {
		msgType, data, readErr := conn.ReadMessage()
		if readErr != nil {
			break
		}
		if msgType != websocket.TextMessage {
			continue
		}
		var msg termMessage
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		switch msg.Type {
		case "input":
			if _, err := p.Write([]byte(msg.Data)); err != nil {
				return
			}
		case "resize":
			if msg.Cols > 0 && msg.Rows > 0 {
				_ = p.Resize(msg.Cols, msg.Rows)
			}
		}
	}
	cancel()
}

func shellCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("pwsh.exe"); err == nil {
			return p, []string{"-NoLogo"}
		}
		if p, err := exec.LookPath("powershell.exe"); err == nil {
			return p, []string{"-NoLogo"}
		}
		if comspec := os.Getenv("COMSPEC"); comspec != "" {
			return comspec, nil
		}
		return "cmd.exe", nil
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	return shell, []string{"-i"}
}

func parseDimension(raw string, fallback int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 || n > 1000 {
		return fallback
	}
	return n
}
