package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	stateDirName        = "state"
	cellarConfigDirName = "cellar"
	stateFileExtension  = ".json"
)

// Tab is one persisted query buffer. SavedName carries the saved-query binding
// so a restored buffer keeps re-saving in place.
type Tab struct {
	Name      string `json:"name,omitempty"`
	SQL       string `json:"sql"`
	Active    bool   `json:"active,omitempty"`
	SavedName string `json:"saved_name,omitempty"`
}

type State struct {
	Tabs    []Tab     `json:"tabs"`
	Updated time.Time `json:"updated"`
}

// Empty reports whether every tab is blank (nothing worth persisting).
func (s State) Empty() bool {
	for _, t := range s.Tabs {
		if strings.TrimSpace(t.SQL) != "" {
			return false
		}
	}
	return true
}

// GetAppConfigDir returns the application's configuration directory. App-free
// (inlined XDG resolution), same as internal/history and internal/saved.
func GetAppConfigDir() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, cellarConfigDirName), nil
}

// SanitizeFilename prepares a string to be used as a part of a filename.
// It replaces non-alphanumeric characters (except _, -, .) with underscores.
func SanitizeFilename(name string) string {
	if name == "" {
		return "default_connection"
	}
	reg := regexp.MustCompile(`[<>:"/\\|?*\s]+`)
	sanitized := reg.ReplaceAllString(name, "_")

	reg = regexp.MustCompile("[^a-zA-Z0-9_.-]+")
	sanitized = reg.ReplaceAllString(sanitized, "_")

	const maxLength = 100
	if len(sanitized) > maxLength {
		sanitized = sanitized[:maxLength]
	}
	return strings.ToLower(sanitized)
}

// GetStateFilePath returns the query-state file path for a connection,
// creating the state directory if needed.
func GetStateFilePath(connectionIdentifier string) (string, error) {
	appConfigDir, err := GetAppConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get app config dir: %w", err)
	}
	stateDirPath := filepath.Join(appConfigDir, stateDirName)
	if err := os.MkdirAll(stateDirPath, 0o700); err != nil {
		return "", fmt.Errorf("failed to create state directory %s: %w", stateDirPath, err)
	}
	return filepath.Join(stateDirPath, SanitizeFilename(connectionIdentifier)+stateFileExtension), nil
}

// Load reads a connection's query state. A missing file is not an error and
// returns an empty state; a corrupt file returns an empty state plus the error
// (callers treat load failure as non-fatal and start blank).
func Load(connectionIdentifier string) (State, error) {
	path, err := GetStateFilePath(connectionIdentifier)
	if err != nil {
		return State{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("failed to read state file %s: %w", path, err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("failed to unmarshal state from %s: %w", path, err)
	}
	return st, nil
}

// Save writes a connection's query state (0600 — SQL can hold literal PII,
// same risk class as history). An all-empty state removes the file instead so
// cleared scratch doesn't resurrect on the next connect.
func Save(connectionIdentifier string, st State) error {
	path, err := GetStateFilePath(connectionIdentifier)
	if err != nil {
		return err
	}
	if st.Empty() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove empty state file %s: %w", path, err)
		}
		return nil
	}
	st.Updated = time.Now().UTC()
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state for %s: %w", connectionIdentifier, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write state file %s: %w", path, err)
	}
	return nil
}
