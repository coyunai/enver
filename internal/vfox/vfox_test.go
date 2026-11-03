package vfox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.4", "1.2.3", 1},
		{"1.10.0", "1.9.9", 1},
		{"v20.11.1", "20.11.0", 1},
		{"1.0.0", "1.0.0-beta", 1},
		{"1.0.0-beta", "1.0.0-alpha", 1},
		{"2.0", "2.0.0", 0},
	}
	for _, tc := range cases {
		if got := CompareVersions(tc.a, tc.b); got != tc.want {
			t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestVersionAtLeast(t *testing.T) {
	if !VersionAtLeast("1.0.13", MinVersion) {
		t.Fatal("expected 1.0.13 to satisfy the minimum")
	}
	if VersionAtLeast("1.0.12", MinVersion) {
		t.Fatal("expected 1.0.12 to be too old")
	}
}

func TestVersionFromPath(t *testing.T) {
	cases := map[string]string{
		`C:\Users\me\.vfox\cache\nodejs\v-22.23.4\nodejs-22.23.4`: "22.23.4",
		"/home/me/.vfox/cache/python/v-3.12.4/python-3.12.4":      "3.12.4",
		"/home/me/.vfox/sdks/nodejs":                              "",
	}
	for input, want := range cases {
		if got := versionFromPath(input); got != want {
			t.Fatalf("versionFromPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseToolsToml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".vfox.toml")
	content := `# project tools
[tools]
nodejs = "20.11.1"
python = '3.12.4'

[env]
FOO = "bar"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ParseToolsToml(path)
	if got["nodejs"] != "20.11.1" || got["python"] != "3.12.4" {
		t.Fatalf("unexpected tools: %#v", got)
	}
	if _, ok := got["FOO"]; ok {
		t.Fatal("env section must not leak into tools")
	}
}

func TestProjectVersionsMissingFile(t *testing.T) {
	if got := ProjectVersions(t.TempDir()); len(got) != 0 {
		t.Fatalf("expected empty map, got %#v", got)
	}
}

func TestStripANSI(t *testing.T) {
	input := "\x1b[32mnodejs\x1b[0m 22.23.4"
	if got := StripANSI(input); got != "nodejs 22.23.4" {
		t.Fatalf("StripANSI = %q", got)
	}
}
