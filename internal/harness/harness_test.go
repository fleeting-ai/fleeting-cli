package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledFiltersMissing(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, "claude")
	if err := os.WriteFile(claude, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := lookPath
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return claude, nil
		}
		return "", os.ErrNotExist
	}
	t.Cleanup(func() { lookPath = old })

	got := InstalledOnPATH()
	if len(got) != 1 || got[0].ID != "claude" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveUnknown(t *testing.T) {
	if _, err := Resolve("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCursorPrefersAgent(t *testing.T) {
	dir := t.TempDir()
	agent := filepath.Join(dir, "agent")
	cursor := filepath.Join(dir, "cursor")
	for _, p := range []string{agent, cursor} {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	old := lookPath
	lookPath = func(file string) (string, error) {
		switch file {
		case "agent":
			return agent, nil
		case "cursor":
			return cursor, nil
		default:
			return "", os.ErrNotExist
		}
	}
	t.Cleanup(func() { lookPath = old })
	in, err := Resolve("cursor")
	if err != nil {
		t.Fatal(err)
	}
	if in.Bin != "agent" {
		t.Fatalf("want agent, got %s", in.Bin)
	}
}

func TestCmdForAgentOMPProfile(t *testing.T) {
	in := Installed{
		Spec: Spec{ID: "omp"},
		Path: "/usr/bin/omp",
		Bin:  "omp",
		Cmd:  []string{"/usr/bin/omp"},
	}
	cmd := in.CmdForAgent("luna")
	if len(cmd) != 3 || cmd[1] != "--profile" || cmd[2] != "luna" {
		t.Fatalf("got %v", cmd)
	}
}

func TestCatalogSplitsOMPAndPi(t *testing.T) {
	var omp, pi bool
	for _, sp := range Catalog {
		if sp.ID == "omp" && sp.Title == "Oh My Pi" {
			omp = true
		}
		if sp.ID == "pi" && sp.Title == "Pi" && len(sp.Bins) == 1 && sp.Bins[0] == "pi" {
			pi = true
		}
	}
	if !omp || !pi {
		t.Fatal("omp and pi must be separate menu entries")
	}
}
