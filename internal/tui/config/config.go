package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

type Config struct {
	ConfigFile      string
	LocalConfigFile string
	AppConfig       *models.AppConfig   `toml:"application"`
	Connections     []models.Connection `toml:"database"`
	Keymaps         models.KeymapConfig `toml:"keymap"`
}

func defaultConfig() *Config {
	return &Config{
		AppConfig: &models.AppConfig{
			DefaultPageSize:              300,
			SidebarOverlay:               false,
			MaxQueryHistoryPerConnection: 100,
			TreeWidth:                    30,
			JSONViewerWordWrap:           false,
			EnterOpensJSONViewer:         false,
		},
	}
}

func DefaultConfigFile() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		configDir = dir
	}
	return filepath.Join(configDir, "lazysql", "config.toml"), nil
}

// FindLocalConfig walks up from CWD to find a `.lazysql.toml` file.
// It stops at the git repository root (`.git` directory/file).
func FindLocalConfig() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		configPath := filepath.Join(dir, ".lazysql.toml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return "", nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", nil
}

func mergeMaps(global, local map[string]any) map[string]any {
	result := make(map[string]any, len(global))
	for k, v := range global {
		result[k] = v
	}
	for k, localVal := range local {
		globalVal, exists := result[k]
		if !exists {
			result[k] = localVal
			continue
		}
		result[k] = mergeValues(globalVal, localVal)
	}
	return result
}

func mergeValues(globalVal, localVal any) any {
	globalMap, globalIsMap := globalVal.(map[string]any)
	localMap, localIsMap := localVal.(map[string]any)
	if globalIsMap && localIsMap {
		return mergeMaps(globalMap, localMap)
	}
	// local [[database]] replaces global connections wholesale, does not append
	return localVal
}

// expandEnvVars expands ${env:VAR_NAME}; bare ${port} is left unchanged for
// connection-time interpolation.
func expandEnvVars(s string) string {
	return os.Expand(s, func(key string) string {
		if envKey, found := strings.CutPrefix(key, "env:"); found {
			return os.Getenv(envKey)
		}
		return "${" + key + "}"
	})
}

func LoadConfig(configFile string) (*Config, error) {
	cfg := defaultConfig()
	cfg.ConfigFile = configFile

	file, err := os.ReadFile(configFile)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	expanded := expandEnvVars(string(file))

	var globalMap map[string]any
	if err := toml.Unmarshal([]byte(expanded), &globalMap); err != nil {
		return nil, err
	}

	localConfigPath, err := FindLocalConfig()
	if err != nil {
		return nil, err
	}

	mergedMap := globalMap
	if localConfigPath != "" {
		cfg.LocalConfigFile = localConfigPath

		localFile, err := os.ReadFile(localConfigPath)
		if err != nil {
			return nil, err
		}

		localExpanded := expandEnvVars(string(localFile))

		var localMap map[string]any
		if err := toml.Unmarshal([]byte(localExpanded), &localMap); err != nil {
			return nil, err
		}

		mergedMap = mergeMaps(globalMap, localMap)
	}

	// marshal merged map back to TOML then unmarshal into cfg so defaultConfig
	// defaults survive for keys absent from the files
	mergedBytes, err := toml.Marshal(mergedMap)
	if err != nil {
		return nil, err
	}

	if err := toml.Unmarshal(mergedBytes, cfg); err != nil {
		return nil, err
	}

	for i, conn := range cfg.Connections {
		cfg.Connections[i].URL = parseConfigURL(&conn)
	}

	return cfg, nil
}

func (c *Config) SaveConnections(connections []models.Connection) error {
	c.Connections = connections

	configFile := c.ConfigFile
	if c.LocalConfigFile != "" {
		configFile = c.LocalConfigFile
	}

	if err := os.MkdirAll(filepath.Dir(configFile), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(configFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	// Owner-only: file holds DB URL creds + SSH host/user/key path. O_CREATE
	// mode only applies on first create, so chmod covers pre-existing files.
	if err := file.Chmod(0o600); err != nil {
		return err
	}

	// SSH passphrase/password are secrets — blanked in the on-disk copy only.
	// In-memory connections keep them so same-session reconnects still work.
	out := *c
	out.Connections = make([]models.Connection, len(connections))
	for i, conn := range connections {
		conn.SSHPassphrase = ""
		conn.SSHPassword = ""
		out.Connections[i] = conn
	}

	return toml.NewEncoder(file).Encode(&out)
}

// parseConfigURL synthesizes a URL from the connection struct when URL is empty.
// Only MSSQL is supported; other providers return the (empty) URL unchanged.
func parseConfigURL(conn *models.Connection) string {
	if conn.URL != "" {
		return conn.URL
	}

	if conn.Provider != drivers.DriverMSSQL {
		return conn.URL
	}

	user := url.QueryEscape(conn.Username)
	pass := url.QueryEscape(conn.Password)

	return fmt.Sprintf(
		"%s://%s:%s@%s:%s?database=%s%s",
		conn.Provider,
		user,
		pass,
		conn.Hostname,
		conn.Port,
		conn.DBName,
		conn.URLParams,
	)
}
