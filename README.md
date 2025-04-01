# Crossplane Function JS

Development environment for the Crossplane Function JS project.

## Development Environment Setup

This project uses [Devbox](https://www.jetpack.io/devbox/) to manage the development environment.

### Prerequisites

1. Install Devbox:

   ```bash
   curl -fsSL https://get.jetpack.io/devbox | bash
   ```

2. Install direnv:

   - macOS: `brew install direnv`
   - Linux: Use your package manager (e.g., `apt install direnv`)

3. Set up direnv in your shell:
   - Bash: Add `eval "$(direnv hook bash)"` to your `.bashrc`
   - Zsh: Add `eval "$(direnv hook zsh)"` to your `.zshrc`

### Getting Started

1. Clone the repository
2. Navigate to the project directory
3. Run `direnv allow` to automatically load the development environment
4. Start developing!

### Available Tools

The development environment includes:

- docker
- git
- nodejs (v22)
- yarn
- jq
- kubectl
- kubernetes-helm
- kind
- k9s
- go (v1.23)
- go-task

### Environment Variables

The following environment variables are automatically set:

- `KUBECONFIG="$PWD/.kubeconfig"` - Points to a local Kubernetes configuration file
- `GOROOT` - Set to the Go installation path

## Project Structure

- `packages/` - Contains the project packages:
  - `cli/` - Command-line interface
  - `libs/` - Shared libraries
  - `sdk/` - Software Development Kit
  - `server/` - Server implementation

## Tasks

This project uses [Task](https://taskfile.dev/) for running common development tasks. See `Taskfile.yml` for available tasks.
