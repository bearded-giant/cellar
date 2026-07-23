package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/bearded-giant/cellar/models"
)

// loadGlobal reads the global config file raw (no env expansion, no .cellar.toml
// merge) so a settings round-trip preserves stored values verbatim.
func loadGlobal(path string) (*Config, error) {
	cfg := defaultConfig()
	cfg.ConfigFile = path
	if data, err := os.ReadFile(path); err == nil {
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return cfg, nil
}

// appField resolves an AppConfig field by case-insensitive name.
func appField(cfg *models.AppConfig, key string) (reflect.Value, string, error) {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		if strings.EqualFold(t.Field(i).Name, key) {
			return v.Field(i), t.Field(i).Name, nil
		}
	}
	return reflect.Value{}, "", fmt.Errorf("unknown setting %q (see `cellar config list`)", key)
}

// GetAppSetting returns a setting's canonical name and current value.
func GetAppSetting(path, key string) (name, value string, err error) {
	cfg, err := loadGlobal(path)
	if err != nil {
		return "", "", err
	}
	f, name, err := appField(cfg.AppConfig, key)
	if err != nil {
		return "", "", err
	}
	return name, fmt.Sprint(f.Interface()), nil
}

// ApplyAppSetting parses value into the setting's type and sets it on app
// in place (no file I/O) — shared by SetAppSetting and the settings screen's
// live-apply.
func ApplyAppSetting(app *models.AppConfig, key, value string) (string, error) {
	f, name, err := appField(app, key)
	if err != nil {
		return "", err
	}
	switch f.Kind() {
	case reflect.String:
		f.SetString(value)
	case reflect.Int:
		n, err := strconv.Atoi(value)
		if err != nil {
			return "", fmt.Errorf("%s wants an integer: %w", name, err)
		}
		f.SetInt(int64(n))
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return "", fmt.Errorf("%s wants true/false: %w", name, err)
		}
		f.SetBool(b)
	default:
		return "", fmt.Errorf("setting %s has unsupported type %s", name, f.Kind())
	}
	return name, nil
}

// AppValue formats a setting's current in-memory value.
func AppValue(app *models.AppConfig, key string) (string, error) {
	f, _, err := appField(app, key)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(f.Interface()), nil
}

// SetAppSetting parses value into the setting's type and writes the config
// back (connections and other settings preserved).
func SetAppSetting(path, key, value string) (string, error) {
	cfg, err := loadGlobal(path)
	if err != nil {
		return "", err
	}
	name, err := ApplyAppSetting(cfg.AppConfig, key, value)
	if err != nil {
		return "", err
	}
	cfg.LocalConfigFile = "" // always the global file
	return name, cfg.SaveConnections(cfg.Connections)
}

// ListAppSettings returns every [application] setting as name/value pairs, in
// struct order.
func ListAppSettings(path string) ([][2]string, error) {
	cfg, err := loadGlobal(path)
	if err != nil {
		return nil, err
	}
	v := reflect.ValueOf(cfg.AppConfig).Elem()
	t := v.Type()
	out := make([][2]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		out = append(out, [2]string{t.Field(i).Name, fmt.Sprint(v.Field(i).Interface())})
	}
	return out, nil
}
