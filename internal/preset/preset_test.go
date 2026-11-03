package preset

import (
	"path/filepath"
	"testing"
)

func TestValidateNormalizes(t *testing.T) {
	p := Preset{
		Name:     "fullstack",
		Runtimes: map[string]string{" nodejs ": " 20.11.1 "},
	}
	if err := Validate(&p); err != nil {
		t.Fatal(err)
	}
	if p.Runtimes["nodejs"] != "20.11.1" {
		t.Fatalf("unexpected runtimes: %#v", p.Runtimes)
	}
}

func TestValidateRejectsBadInput(t *testing.T) {
	cases := []Preset{
		{Name: "", Runtimes: map[string]string{"nodejs": "20"}},
		{Name: "bad name", Runtimes: map[string]string{"nodejs": "20"}},
		{Name: "empty", Runtimes: map[string]string{}},
		{Name: "space", Runtimes: map[string]string{"node js": "20"}},
	}
	for _, p := range cases {
		if err := Validate(&p); err == nil {
			t.Fatalf("expected validation error for %#v", p)
		}
	}
}

func TestStoreLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "presets.json")
	store, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p := Preset{Name: "web", Runtimes: map[string]string{"nodejs": "20.11.1"}, Env: map[string]string{"NODE_ENV": "development"}}
	if err := store.Save(p); err != nil {
		t.Fatal(err)
	}
	list := store.List()
	if len(list) != 1 || list[0].Name != "web" {
		t.Fatalf("unexpected list: %#v", list)
	}
	if _, ok := store.Get("web"); !ok {
		t.Fatal("expected preset to exist")
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := reloaded.Get("web"); !ok || got.Env["NODE_ENV"] != "development" {
		t.Fatalf("unexpected reloaded preset: %#v", got)
	}
	if err := store.Delete("web"); err != nil {
		t.Fatal(err)
	}
	if len(store.List()) != 0 {
		t.Fatal("expected preset to be deleted")
	}
	if err := store.Delete("web"); err == nil {
		t.Fatal("expected delete of missing preset to fail")
	}
}
