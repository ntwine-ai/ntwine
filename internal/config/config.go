package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	APIKey       string            `json:"api_key"`
	Models       []string          `json:"models"`
	TavilyKey    string            `json:"tavily_api_key"`
	ProviderKeys map[string]string `json:"provider_keys,omitempty"`
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".socratic-slopinar")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

func Load() (Config, error) {
	data, wasEncrypted, err := readAndDecrypt(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return Config{Models: []string{}, ProviderKeys: map[string]string{}}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Models == nil {
		cfg.Models = []string{}
	}
	if cfg.ProviderKeys == nil {
		cfg.ProviderKeys = map[string]string{}
	}
	if !wasEncrypted {
		_ = Save(cfg)
	}
	return cfg, nil
}

func Save(cfg Config) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return encryptAndWrite(configPath(), data)
}

func AddModel(cfg Config, model string) Config {
	for _, m := range cfg.Models {
		if m == model {
			return cfg
		}
	}
	models := make([]string, len(cfg.Models)+1)
	copy(models, cfg.Models)
	models[len(cfg.Models)] = model
	return Config{
		APIKey:       cfg.APIKey,
		Models:       models,
		TavilyKey:    cfg.TavilyKey,
		ProviderKeys: cfg.ProviderKeys,
	}
}

func RemoveModel(cfg Config, model string) Config {
	models := make([]string, 0, len(cfg.Models))
	for _, m := range cfg.Models {
		if m != model {
			models = append(models, m)
		}
	}
	return Config{
		APIKey:       cfg.APIKey,
		Models:       models,
		TavilyKey:    cfg.TavilyKey,
		ProviderKeys: cfg.ProviderKeys,
	}
}

func BuildProviderKeys(cfg Config) map[string]string {
	keys := make(map[string]string, len(cfg.ProviderKeys)+1)
	for k, v := range cfg.ProviderKeys {
		keys[k] = v
	}
	if cfg.APIKey != "" {
		if _, exists := keys["openrouter"]; !exists {
			keys["openrouter"] = cfg.APIKey
		}
	}
	return keys
}
