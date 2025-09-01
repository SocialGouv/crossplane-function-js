package node

import (
	"strings"

	"github.com/socialgouv/xfuncjs-server/pkg/logger"
)

// DependencyResolver handles dependency resolution operations
type DependencyResolver struct {
	logger logger.Logger
}

// NewDependencyResolver creates a new dependency resolver
func NewDependencyResolver(logger logger.Logger) *DependencyResolver {
	return &DependencyResolver{
		logger: logger.WithField("component", "dependency-resolver"),
	}
}

// ResolveDependencies processes and resolves dependencies from the input specification
func (dr *DependencyResolver) ResolveDependencies(inputDependencies map[string]string, workspaceMap map[string]string, workspaceRoot string, logger logger.Logger) (map[string]interface{}, error) {
	dependencies := make(map[string]interface{})

	logger.WithField("input_dependencies", inputDependencies).
		Info("Starting dependency processing")

	// Add user-specified dependencies
	for k, v := range inputDependencies {
		resolvedDep := v

		if strings.HasPrefix(k, "@crossplane-js/") {
			// Resolve crossplane package dependencies
			if resolved, _, resolveErr := ResolveWorkspacePackage(k, workspaceRoot, workspaceMap, logger); resolveErr != nil {
				logger.WithField("dependency", k).
					WithField("value", v).
					WithField("error", resolveErr.Error()).
					Warn("Failed to resolve crossplane dependency, using original value")
			} else {
				resolvedDep = resolved
			}
		}

		if strings.HasPrefix(v, "link:") {
			// Resolve workspace package dependencies
			if resolved, _, resolveErr := ResolveWorkspacePackage(k, workspaceRoot, workspaceMap, logger); resolveErr != nil {
				logger.WithField("dependency", k).
					WithField("value", v).
					WithField("error", resolveErr.Error()).
					Warn("Failed to resolve workspace dependency, using original value")
			} else {
				resolvedDep = resolved
			}
		}

		dependencies[k] = resolvedDep
	}

	logger.WithField("total_dependencies", len(dependencies)).
		Info("Completed dependency processing")

	return dependencies, nil
}

// ValidateDependencies validates that all dependencies are properly formatted
func (dr *DependencyResolver) ValidateDependencies(dependencies map[string]interface{}) error {
	// Add validation logic here if needed
	// For now, we'll just return nil as the current implementation doesn't require validation
	return nil
}
