package packages

import (
	"strings"
	"testing"
)

func TestBuildExportNode(t *testing.T) {
	export := BuildExport("nodejs", []Package{
		{Name: "typescript", Version: "5.6.3"},
		{Name: "pnpm", Version: "9.12.0"},
	})
	if export.Filename != "global-packages.json" {
		t.Fatalf("unexpected filename: %s", export.Filename)
	}
	if !strings.Contains(export.Content, `"typescript": "5.6.3"`) {
		t.Fatalf("unexpected content: %s", export.Content)
	}
}

func TestBuildExportPython(t *testing.T) {
	export := BuildExport("python", []Package{
		{Name: "requests", Version: "2.32.3"},
	})
	if export.Filename != "requirements.txt" {
		t.Fatalf("unexpected filename: %s", export.Filename)
	}
	if export.Content != "requests==2.32.3\n" {
		t.Fatalf("unexpected content: %q", export.Content)
	}
}
