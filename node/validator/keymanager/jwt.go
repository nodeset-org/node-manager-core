package keymanager

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

const (
	// The default length of the API authorization key, in bytes.
	// The spec requires at least 256 bits but we use 384 bits for extra security.
	DefaultKeyLength int = 384 / 8

	// The minimum length of the API authorization key, in bytes.
	MinKeyLength int = 256 / 8

	// The permissions to set on the API authorization key file
	KeyPermissions fs.FileMode = 0600
)

// Loads the JWT key from disk.
// If it doesn't exist, one is generated and saved first.
// The directory to contain the key file must already exist.
func GenerateOrLoadAuthKey(path string, keyLengthInBytes int) (string, error) {
	// Check if the file exists
	exists := true
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		exists = false
	} else if err != nil {
		return "", fmt.Errorf("error checking if key [%s] exists: %w", path, err)
	}

	// Read it if it already exists
	if exists {
		key, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("error reading JWT key from [%s]: %w", path, err)
		}
		return string(key), nil
	}

	// Generate the key
	if keyLengthInBytes < MinKeyLength {
		return "", fmt.Errorf("key length must be at least %d bytes", MinKeyLength)
	}
	buffer := make([]byte, keyLengthInBytes)
	_, err = rand.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("error generating random key: %w", err)
	}

	// Encode the key as a hex string
	hexBuffer := hex.EncodeToString(buffer)

	// Write the key to disk
	err = os.WriteFile(path, []byte(hexBuffer), KeyPermissions)
	if err != nil {
		return "", fmt.Errorf("error writing key to [%s]: %w", path, err)
	}
	return hexBuffer, nil
}
