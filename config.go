package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type config struct {
	GitPath  string    `json:"gitPath"`
	Profiles []profile `json:"profiles"`
}

type profile struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bettergit"), nil
}

func loadConfig() (config, error) {
	var cfg config
	path, err := configPath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	return cfg, json.Unmarshal(data, &cfg)
}

// saveProfile appends p to the profiles in the config file, leaving every other key as it is.
func saveProfile(p profile) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	raw := map[string]json.RawMessage{}
	data, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(data, &raw)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var profiles []profile
	if raw["profiles"] != nil {
		if err := json.Unmarshal(raw["profiles"], &profiles); err != nil {
			return err
		}
	}
	if raw["profiles"], err = json.Marshal(append(profiles, p)); err != nil {
		return err
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}
