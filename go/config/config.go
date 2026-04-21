package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// DefaultConfigPath returns the platform-appropriate config file location:
//
//	macOS   → ~/Library/Application Support/ksp-moder/config.json
//	Windows → %AppData%\Roaming\ksp-moder\config.json
//	Linux   → ~/.config/ksp-moder/config.json
func DefaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ksp-moder", "config.json"), nil
}

// Settings holds user preferences.
type Settings struct {
	AccentColor   string `json:"accent_color"`
	LogLines      int    `json:"log_lines"`
	ConfirmRemove bool   `json:"confirm_remove"`
	SortModsBy    string `json:"sort_mods_by"`
}

// Config is the top-level structure persisted to config.json.
type Config struct {
	KSPPath     *string             `json:"ksp_path"`
	AllInstalls []string            `json:"all_installs"`
	Profiles    map[string][]string `json:"profiles"`
	ModNotes    map[string]string   `json:"mod_notes"`
	Settings    Settings            `json:"settings"`
}

func defaults() Config {
	return Config{
		KSPPath:     nil,
		AllInstalls: []string{},
		Profiles:    map[string][]string{},
		ModNotes:    map[string]string{},
		Settings: Settings{
			AccentColor:   "#8AC04A",
			LogLines:      500,
			ConfirmRemove: true,
			SortModsBy:    "name",
		},
	}
}

// Manager provides thread-safe access to the config file.
type Manager struct {
	path string
	mu   sync.Mutex
}

// NewManager creates a Manager for the given file path.
func NewManager(path string) *Manager {
	return &Manager{path: path}
}

// EnsureConfig creates the config directory and file with defaults if they do not exist.
func (m *Manager) EnsureConfig() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(m.path); os.IsNotExist(err) {
		return m.write(defaults())
	}
	return nil
}

// Load reads and returns the current config, applying defaults for missing fields.
func (m *Manager) Load() (Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := defaults()
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	// Ensure collections are non-nil after unmarshalling.
	if cfg.AllInstalls == nil {
		cfg.AllInstalls = []string{}
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string][]string{}
	}
	if cfg.ModNotes == nil {
		cfg.ModNotes = map[string]string{}
	}
	return cfg, nil
}

// Save writes cfg to disk.
func (m *Manager) Save(cfg Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.write(cfg)
}

func (m *Manager) write(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0644)
}
