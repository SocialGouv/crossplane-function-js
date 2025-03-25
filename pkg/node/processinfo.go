package node

import (
	"os/exec"
	"sync"
	"time"
)

// ProcessInfo holds information about a Node.js process
type ProcessInfo struct {
	Process     *exec.Cmd
	Client      *NodeClient
	LastUsed    time.Time
	Lock        sync.Mutex
	Port        int    // Store the assigned port for this process
	TempDirPath string // Path to the temporary directory for this process
}
