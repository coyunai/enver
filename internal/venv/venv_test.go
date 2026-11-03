package venv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHealthDetectsMissingBase(t *testing.T) {
	root := t.TempDir()
	envPath := filepath.Join(root, "env")
	if err := os.MkdirAll(filepath.Join(envPath, "Scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(envPath, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "home = /missing/python\nversion = 3.12.4\n"
	if err := os.WriteFile(filepath.Join(envPath, "pyvenv.cfg"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(PythonExe(envPath), []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	record := Record{
		ID:            "test",
		Name:          "test",
		Path:          envPath,
		PythonPath:    filepath.Join(root, "missing-python"),
		PythonVersion: "3.12.4",
	}
	ok, reason := Health(record)
	if ok {
		t.Fatal("expected unhealthy environment when base interpreter is missing")
	}
	if reason == "" {
		t.Fatal("expected a reason")
	}
}

func TestReadConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyvenv.cfg"), []byte("home = C:\\Python312\nversion_info = 3.12.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := ReadConfig(dir)
	if cfg["home"] != `C:\Python312` || cfg["version_info"] != "3.12.4" {
		t.Fatalf("unexpected cfg: %#v", cfg)
	}
}

func TestFindPythonMissing(t *testing.T) {
	if got := FindPython(t.TempDir()); got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}
