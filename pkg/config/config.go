package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type DebugConfig struct {
	Enabled         bool `json:"enabled"`
	ShowChatPackets bool `json:"show_chat_packets"`
	ShowRawHex      bool `json:"show_raw_hex"`
	ShowJSON        bool `json:"show_json"`
}

type TranslationConfig struct {
	Enabled    bool   `json:"enabled"`
	TargetLang string `json:"target_lang"`
	Verbose    bool   `json:"verbose"`
}

type SorterConfig struct {
	AutoEchest bool `json:"auto_echest"`
}

type CombatConfig struct {
	AutoEat   bool `json:"auto_eat"`
	AutoTotem bool `json:"auto_totem"`
	AutoArmor bool `json:"auto_armor"`
}

type KeepaliveConfig struct {
	Enabled          bool    `json:"enabled"`
	HealthThreshold  float64 `json:"health_threshold"`
	VoidYThreshold   float64 `json:"void_y_threshold"`
}

type ServerConfig struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type AccountConfig struct {
	Username      string `json:"username"`
	OwnerUsername string `json:"owner_username,omitempty"`
	RefreshToken  string `json:"refresh_token"`
}

type CloudTunnelConfig struct {
	Enabled      bool   `json:"enabled"`
	HealthURL    string `json:"health_url"`
	APIToken     string `json:"api_token"`
	GitHubRepo   string `json:"github_repo"`
	TunnelConfig string `json:"tunnel_config"`
}

type Config struct {
	Server      ServerConfig      `json:"server"`
	Debug       DebugConfig       `json:"debug"`
	Translation TranslationConfig `json:"translation"`
	Sorter      SorterConfig      `json:"sorter"`
	Combat      CombatConfig      `json:"combat"`
	Keepalive   KeepaliveConfig   `json:"keepalive"`
	Account     AccountConfig     `json:"account"`
	CloudTunnel CloudTunnelConfig `json:"cloud_tunnel"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Address: "localhost",
			Port:    25565,
		},
		Translation: TranslationConfig{
			Enabled:    true,
			TargetLang: "en",
		},
		Sorter: SorterConfig{
			AutoEchest: true,
		},
		Combat: CombatConfig{
			AutoEat:   true,
			AutoTotem: true,
			AutoArmor: true,
		},
		Keepalive: KeepaliveConfig{
			Enabled:         true,
			HealthThreshold: 10,
			VoidYThreshold:  -50,
		},
		Account: AccountConfig{
			Username: "",
		},
	}
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jurobot"), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func LangPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "langua.json"), nil
}

func Load() (Config, string, error) {
	cfg := Default()

	paths := []string{}

	p, err := ConfigPath()
	if err == nil {
		paths = append(paths, p)
	}

	paths = append(paths, "config.json")

	exe, err := os.Executable()
	if err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "config.json"))
	}

	var loadedPath string
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			json.Unmarshal(data, &cfg)
			loadedPath = p
			break
		}
	}

	return cfg, loadedPath, nil
}

func Save(cfg Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
