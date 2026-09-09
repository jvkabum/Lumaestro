package core

import (
	"os"
	"path/filepath"
	"testing"

	"Lumaestro/internal/agents/acp"
	"Lumaestro/internal/config"
)

func TestWorkspacePersistenceAndRegistration(t *testing.T) {
	tempBase := t.TempDir()
	projA := filepath.Join(tempBase, "ProjectAlpha")
	projB := filepath.Join(tempBase, "ProjectBeta")
	_ = os.MkdirAll(projA, 0755)
	_ = os.MkdirAll(projB, 0755)

	cfg := &config.Config{
		ExternalProjects: []config.ProjectScan{},
		ActiveWorkspace:  "",
	}

	exec := acp.NewACPExecutor("", "")
	app := &App{
		executor: exec,
		config:   cfg,
	}

	// 1. Initial workspace should be sandbox / empty
	ws := app.GetWorkspace()
	if ws["path"] != "" {
		t.Fatalf("expected empty initial workspace, got: %s", ws["path"])
	}

	// 2. SetWorkspace(projA) should activate and register in ExternalProjects
	err := app.SetWorkspace(projA)
	if err != nil {
		t.Fatalf("SetWorkspace failed: %v", err)
	}

	if app.config.ActiveWorkspace != projA {
		t.Errorf("expected ActiveWorkspace %s, got %s", projA, app.config.ActiveWorkspace)
	}
	if len(app.config.ExternalProjects) != 1 {
		t.Fatalf("expected 1 external project, got %d", len(app.config.ExternalProjects))
	}
	if app.config.ExternalProjects[0].Path != projA {
		t.Errorf("expected external project path %s, got %s", projA, app.config.ExternalProjects[0].Path)
	}
	if app.config.ExternalProjects[0].CoreNode != "ProjectAlpha" {
		t.Errorf("expected CoreNode ProjectAlpha, got %s", app.config.ExternalProjects[0].CoreNode)
	}

	// 3. SetWorkspace(projA) again should NOT duplicate in ExternalProjects
	err = app.SetWorkspace(projA)
	if err != nil {
		t.Fatalf("SetWorkspace again failed: %v", err)
	}
	if len(app.config.ExternalProjects) != 1 {
		t.Fatalf("expected still 1 external project after re-setting, got %d", len(app.config.ExternalProjects))
	}

	// 4. SetWorkspace(projB) should register projB as well
	err = app.SetWorkspace(projB)
	if err != nil {
		t.Fatalf("SetWorkspace projB failed: %v", err)
	}
	if len(app.config.ExternalProjects) != 2 {
		t.Fatalf("expected 2 external projects, got %d", len(app.config.ExternalProjects))
	}

	// 5. GetWorkspace should reflect projB
	ws = app.GetWorkspace()
	if ws["path"] != projB {
		t.Errorf("expected GetWorkspace path %s, got %s", projB, ws["path"])
	}
	if ws["name"] != "ProjectBeta" {
		t.Errorf("expected GetWorkspace name ProjectBeta, got %s", ws["name"])
	}

	// 6. ClearWorkspace should return to sandbox but KEEP ExternalProjects list
	ws = app.ClearWorkspace()
	if ws["path"] != "" {
		t.Errorf("expected empty path after ClearWorkspace, got: %s", ws["path"])
	}
	if len(app.config.ExternalProjects) != 2 {
		t.Errorf("expected ExternalProjects to retain both projects after clear, got %d", len(app.config.ExternalProjects))
	}

	// 7. UnlinkProject(projA) should remove projA from ExternalProjects
	err = app.UnlinkProject(projA)
	if err != nil {
		t.Fatalf("UnlinkProject failed: %v", err)
	}
	if len(app.config.ExternalProjects) != 1 {
		t.Fatalf("expected 1 project remaining after unlink, got %d", len(app.config.ExternalProjects))
	}
	if app.config.ExternalProjects[0].Path != projB {
		t.Errorf("expected remaining project to be projB, got %s", app.config.ExternalProjects[0].Path)
	}
}
