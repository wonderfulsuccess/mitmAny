package main

import (
	_ "embed"
	"mitmAny/internal/codex"
)

// Embed the canonical source so installers work offline from every release ZIP.
//
//go:embed skills/mitmany-network-analysis/SKILL.md
var codexSkill []byte

func (a *App) GetCodexStatus() codex.Status {
	installer, err := codex.New(codexSkill, a.dir)
	if err != nil {
		return codex.Status{Error: err.Error()}
	}
	return a.codexRuntimeStatus(installer.Status())
}
func (a *App) ConnectCodex() codex.Status {
	installer, err := codex.New(codexSkill, a.dir)
	if err != nil {
		return codex.Status{Error: err.Error()}
	}
	return a.codexRuntimeStatus(installer.Install())
}
func (a *App) codexRuntimeStatus(s codex.Status) codex.Status {
	if a.agentServer != nil {
		s.ServiceRunning = true
		s.LastActivity = a.agentServer.LastActivity()
	}
	return s
}
