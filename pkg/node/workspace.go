package node

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/socialgouv/xfuncjs-server/pkg/logger"
)

// WorkspacePackage represents a package in the yarn workspace
type WorkspacePackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string
}

// WorkspaceInfo represents the output from yarn workspaces list --json
type WorkspaceInfo struct {
	Location string `json:"location"`
	Name     string `json:"name"`
}

// Simple bootstrap cache - load once at startup, never changes
var (
	workspaceCache map[string]string
	workspaceOnce  sync.Once
)

// GetWorkspacePackages discovers workspace packages using yarn workspaces list --json
// Uses simple bootstrap cache - loads once and never changes during runtime
func GetWorkspacePackages(workspaceRoot string, logger logger.Logger) (map[string]string, error) {
	var err error
	workspaceOnce.Do(func() {
		workspaceCache, err = loadWorkspacePackages(workspaceRoot, logger)
	})
	return workspaceCache, err
}

// loadWorkspacePackages loads workspace packages from yarn
func loadWorkspacePackages(workspaceRoot string, logger logger.Logger) (map[string]string, error) {
	logger.Info("Loading workspace packages")

	// Run yarn workspaces list --json from the workspace root
	cmd := exec.Command("yarn", "workspaces", "list", "--json")
	cmd.Dir = workspaceRoot

	output, err := cmd.Output()
	if err != nil {
		logger.WithField("error", err.Error()).
			Error("Failed to run yarn workspaces list command")
		return nil, fmt.Errorf("failed to run yarn workspaces list: %w", err)
	}

	// Parse the output - each line is a separate JSON object
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	workspaceMap := make(map[string]string)

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var workspace WorkspaceInfo
		if err := json.Unmarshal([]byte(line), &workspace); err != nil {
			logger.WithField("line", line).
				WithField("error", err.Error()).
				Warn("Failed to parse workspace info line")
			continue
		}

		// Skip the root workspace (location is ".")
		if workspace.Location == "." {
			continue
		}

		workspaceMap[workspace.Name] = workspace.Location
	}

	return workspaceMap, nil
}

// ResolveWorkspacePackage resolves a link: dependency to a workspace package
func ResolveWorkspacePackage(packageName, workspaceRoot string, workspaceMap map[string]string, logger logger.Logger) (string, string, error) {
	// Check if this package is actually in the workspace
	if actualLocation, exists := workspaceMap[packageName]; exists {
		packageLocation := actualLocation
		// Return as link dependency with absolute path to /app
		linkRef := fmt.Sprintf("link:/app/%s", packageLocation)
		logger.WithField("package_name", packageName).
			WithField("resolved_to", linkRef).
			Info("Resolved workspace dependency")
		return linkRef, packageLocation, nil
	} else {
		return "", "", fmt.Errorf("package %s is not in the workspace", packageName)
	}

}
