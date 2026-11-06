package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mitmAny/internal/agent"
	"mitmAny/internal/capture"
)

func TestConnectCodexAppLifecycle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, "codex-config"))
	dir := t.TempDir()
	store, err := capture.NewStore(filepath.Join(dir, "session"))
	if err != nil {
		t.Fatal(err)
	}
	service := agent.New(store, func() any { return map[string]any{"running": false} }, func() uint64 { return 0 })
	server, err := agent.Listen(dir, service)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	app := &App{dir: dir, agentServer: server}
	if s := app.GetCodexStatus(); s.Ready || !s.ServiceRunning || s.LastActivity != 0 {
		t.Fatal(s)
	}
	status := app.ConnectCodex()
	if !status.Ready || status.Error != "" {
		t.Fatal(status)
	}
	installed, err := os.ReadFile(status.SkillPath)
	if err != nil || string(installed) != string(codexSkill) {
		t.Fatal("bundled skill not installed", err)
	}
	if _, err = agent.RemoteCall(context.Background(), dir, "capture_status", agent.Args{}); err != nil {
		t.Fatal(err)
	}
	if s := app.GetCodexStatus(); !s.Ready || s.LastActivity == 0 {
		t.Fatal(s)
	}
}
