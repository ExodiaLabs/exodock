// Package config loads optional local preferences without storing Docker credentials.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type Settings struct {
	Version        int    `json:"version"`
	Theme          string `json:"theme"`
	ASCII          bool   `json:"ascii"`
	NoColor        bool   `json:"no_color"`
	RefreshSeconds int    `json:"refresh_seconds"`
}

func Default() Settings { return Settings{Version: 1, Theme: "dark", RefreshSeconds: 5} }
func Path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "exodock", "config.json"), nil
}
func Load(path string) (Settings, error) {
	settings := Default()
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return settings, errors.New("cannot read Exodock preferences")
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil || len(raw) > 64*1024 {
		return Default(), errors.New("preferences exceed the 64 KiB limit or cannot be read")
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return Default(), errors.New("preferences must contain a JSON object")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&settings); err != nil {
		return Default(), errors.New("invalid Exodock preferences JSON")
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return Default(), errors.New("preferences must contain a single JSON object")
	}
	if err = settings.Validate(); err != nil {
		return Default(), err
	}
	return settings, nil
}
func (s Settings) Validate() error {
	if s.Version != 1 {
		return errors.New("unsupported Exodock preferences version (expected 1)")
	}
	if s.Theme != "dark" && s.Theme != "light" && s.Theme != "auto" {
		return errors.New("theme must be dark, light or auto")
	}
	if s.RefreshSeconds < 1 || s.RefreshSeconds > 60 {
		return errors.New("refresh_seconds must be between 1 and 60")
	}
	return nil
}
