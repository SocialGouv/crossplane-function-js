# Skyhook Libs

This package contains shared libraries for Crossplane Skyhook.

## Recent Changes

### YAML Parsing in `pkg/node/process.go`

The code now properly parses the `.yarnrc.yml` file using the `sigs.k8s.io/yaml` library instead of string manipulation.

### Direct Yarn Execution

We've improved the yarn installation process by directly executing the yarn binary from the path specified in the `.yarnrc.yml` file. This approach:

1. Extracts the `yarnPath` from the `.yarnrc.yml` file
2. Constructs the absolute path to the yarn executable
3. Executes it directly with Node.js

## Implementation Details

The Go code now:

1. Parses the `.yarnrc.yml` file to extract the `yarnPath` value
2. Falls back to a default path if not found
3. Executes the yarn binary directly:

```go
// Extract the yarnPath from the .yarnrc.yml file
var yarnPath string
if yarnConfig != nil {
    if path, ok := yarnConfig["yarnPath"].(string); ok {
        yarnPath = path
    }
}

// If yarnPath is not found, use the default yarn executable
if yarnPath == "" {
    yarnPath = ".yarn/releases/yarn-4.7.0.cjs"
}

// Construct the absolute path to the yarn executable
yarnExecPath := filepath.Join("/app", yarnPath)

// Run yarn install using the extracted yarn executable
yarnCmd := exec.Command("node", yarnExecPath, "install")
yarnCmd.Dir = uniqueDirPath // Set the working directory
```

This approach ensures that the yarn installation process runs in a separate process and doesn't block the main Node.js process, while also being more robust by using the exact yarn executable specified in the project configuration.
