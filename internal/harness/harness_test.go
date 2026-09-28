package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledFiltersMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
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
	t.Setenv("HOME", t.TempDir())
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

func TestCmdForAgentClaudeAndCodexBare(t *testing.T) {
	for _, id := range []string{"claude", "codex"} {
		in := Installed{Spec: Spec{ID: id}, Path: "/usr/bin/" + id}
		cmd := in.CmdForAgent("luna")
		if len(cmd) != 1 || cmd[0] != "/usr/bin/"+id {
			t.Fatalf("%s: got %v", id, cmd)
		}
	}
}

func TestIDFromBin(t *testing.T) {
	cases := map[string]string{
		"/usr/bin/claude": "claude",
		"codex":           "codex",
		"agent":           "cursor",
		"cursor-agent":    "cursor",
		"omp":             "omp",
		"pi":              "pi",
		"bash":            "",
	}
	for in, want := range cases {
		if got := IDFromBin(in); got != want {
			t.Fatalf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestPrepareSessionClaudeCodex(t *testing.T) {
	home := t.TempDir()
	env := PrepareSession(home, "claude", "luna")
	if len(env) != 1 || !strings.HasPrefix(env[0], "CLAUDE_CONFIG_DIR=") {
		t.Fatalf("claude env %v", env)
	}
	if _, err := os.Stat(ClaudeConfigDir(home, "luna")); err != nil {
		t.Fatal(err)
	}
	env = PrepareSession(home, "codex", "nova")
	if len(env) != 1 || !strings.HasPrefix(env[0], "CODEX_HOME=") {
		t.Fatalf("codex env %v", env)
	}
	if _, err := os.Stat(CodexHome(home, "nova")); err != nil {
		t.Fatal(err)
	}
	if env := PrepareSession(home, "omp", "kite"); len(env) != 0 {
		t.Fatalf("omp should not set extra env, got %v", env)
	}
}

func TestMissingCmdMentionsBinary(t *testing.T) {
	cmd := MissingCmd("claude", "luna")
	if len(cmd) != 3 || cmd[0] != "bash" {
		t.Fatalf("got %v", cmd)
	}
	if !strings.Contains(cmd[2], "claude not found") {
		t.Fatalf("help %s", cmd[2])
	}
	cmd = MissingCmd("codex", "nova")
	if !strings.Contains(cmd[2], "codex not found") {
		t.Fatalf("help %s", cmd[2])
	}
}

func TestFindBinUsesLocalBinWhenPATHEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	local := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	omp := filepath.Join(local, "omp")
	if err := os.WriteFile(omp, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := lookPath
	lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	t.Cleanup(func() { lookPath = old })

	in, err := Resolve("omp")
	if err != nil {
		t.Fatal(err)
	}
	if in.Path != omp {
		t.Fatalf("got %s want %s", in.Path, omp)
	}
	path := PATHEnv()
	if !strings.Contains(path, local) {
		t.Fatalf("PATHEnv missing %s: %s", local, path)
	}
}

func TestLooksLikeMissingCmd(t *testing.T) {
	if !LooksLikeMissingCmd(MissingCmd("omp", "risa")) {
		t.Fatal("placeholder should match")
	}
	if LooksLikeMissingCmd([]string{"/home/u/.local/bin/omp", "--profile", "risa"}) {
		t.Fatal("real cmd")
	}
	if !Placeholder(nil, "omp not found on PATH.\n") {
		t.Fatal("screen hint")
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
