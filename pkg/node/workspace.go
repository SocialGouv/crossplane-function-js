package node

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	logger.WithField("workspace_count", len(workspaceMap)).
		Info("Loaded workspace packages")

	return workspaceMap, nil
}

// ResolveWorkspacePackage resolves a link: dependency to a workspace package
func ResolveWorkspacePackage(dependencyValue, workspaceRoot string, workspaceMap map[string]string, logger logger.Logger) (string, string, error) {
	var targetPath string

	if strings.HasPrefix(dependencyValue, "link:") {
		targetPath = strings.TrimPrefix(dependencyValue, "link:")
	} else {
		return dependencyValue, "", nil
	}

	// Clean the target path
	targetPath = filepath.Clean(targetPath)

	// Normalize the target path to handle relative paths like ../../../packages/sdk
	// Convert to absolute path and then back to relative from workspace root
	absoluteTargetPath := filepath.Join(workspaceRoot, targetPath)
	cleanAbsolutePath := filepath.Clean(absoluteTargetPath)
	normalizedTargetPath, err := filepath.Rel(workspaceRoot, cleanAbsolutePath)
	if err != nil {
		normalizedTargetPath = targetPath
	}

	// If the normalized path still contains "..", it means it's going outside the workspace
	// In this case, try to extract just the final part (e.g., "packages/sdk" from "../packages/sdk")
	if strings.Contains(normalizedTargetPath, "..") {
		// Split the path and find the part that doesn't start with ".."
		parts := strings.Split(normalizedTargetPath, string(filepath.Separator))
		var cleanParts []string
		for _, part := range parts {
			if part != ".." && part != "." && part != "" {
				cleanParts = append(cleanParts, part)
			}
		}
		if len(cleanParts) > 0 {
			candidatePath := filepath.Join(cleanParts...)
			normalizedTargetPath = candidatePath
		}
	}

	// Find the workspace package at this location
	var packageName string
	var packageLocation string

	for name, location := range workspaceMap {
		if location == normalizedTargetPath {
			packageName = name
			packageLocation = location
			break
		}
	}

	if packageName == "" {
		// Try to resolve by reading package.json at the target path
		absolutePath := filepath.Join(workspaceRoot, targetPath)
		packageJSONPath := filepath.Join(absolutePath, "package.json")

		if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
			return "", "", fmt.Errorf("no workspace package found at %s", targetPath)
		}

		// Read the package.json to get the package name
		packageJSONBytes, err := os.ReadFile(packageJSONPath)
		if err != nil {
			logger.WithField("package_json_path", packageJSONPath).
				WithField("error", err.Error()).
				Error("Failed to read package.json")
			return "", "", fmt.Errorf("failed to read package.json at %s: %w", packageJSONPath, err)
		}

		var pkg WorkspacePackage
		if err := json.Unmarshal(packageJSONBytes, &pkg); err != nil {
			logger.WithField("package_json_path", packageJSONPath).
				WithField("error", err.Error()).
				Error("Failed to parse package.json")
			return "", "", fmt.Errorf("failed to parse package.json at %s: %w", packageJSONPath, err)
		}

		if pkg.Name == "" {
			return "", "", fmt.Errorf("package name not found in package.json at %s", packageJSONPath)
		}

		// Check if this package is actually in the workspace
		if actualLocation, exists := workspaceMap[pkg.Name]; exists {
			if actualLocation != targetPath {
				logger.WithField("package_name", pkg.Name).
					WithField("expected_location", targetPath).
					WithField("actual_location", actualLocation).
					Warn("Package location mismatch, using actual location")
				packageLocation = actualLocation
			} else {
				packageLocation = targetPath
			}
			packageName = pkg.Name
		} else {
			return "", "", fmt.Errorf("package %s at %s is not in the workspace", pkg.Name, targetPath)
		}
	}

	// Return as link dependency with absolute path to /app
	linkRef := fmt.Sprintf("link:/app/%s", packageLocation)
	logger.WithField("package_name", packageName).
		WithField("resolved_to", linkRef).
		Info("Resolved workspace dependency")

	return linkRef, packageLocation, nil
}
