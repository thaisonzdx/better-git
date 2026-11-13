package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type config struct {
	GitPath string `json:"gitPath"`
}

func loadConfig() (config, error) {
	var cfg config
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(filepath.Join(home, ".bettergit", "config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	return cfg, json.Unmarshal(data, &cfg)
}
