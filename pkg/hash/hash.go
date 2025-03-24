package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// GenerateHash generates a deterministic hash for the given code
// It normalizes the code by removing whitespace and comments to ensure
// that functionally equivalent code produces the same hash
func GenerateHash(code string) string {
	// For now, we'll use a simple SHA-256 hash of the code
	// In a more sophisticated implementation, we might want to parse the code
	// and normalize it to ensure that functionally equivalent code produces
	// the same hash
	hash := sha256.Sum256([]byte(code))
	return hex.EncodeToString(hash[:])
}

// GenerateProcessID generates a unique ID for a Node.js process based on the code
func GenerateProcessID(code string) string {
	hash := GenerateHash(code)
	return "node-" + hash[:16] // Use first 16 chars of hash for brevity
}

// GenerateTempFilename generates a filename for a temporary file containing the code
func GenerateTempFilename(code string, extension string) string {
	hash := GenerateHash(code)
	// Ensure the extension starts with a dot
	if !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}
	return hash[:16] + extension
}

// GenerateInputHash generates a hash based on the entire input spec
func GenerateInputHash(specJSON []byte) string {
	hash := sha256.Sum256(specJSON)
	return hex.EncodeToString(hash[:])
}
