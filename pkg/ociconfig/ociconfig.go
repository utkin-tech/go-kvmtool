package ociconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/runtime-spec/specs-go"
)

var Spec specs.Spec
var RootfsPath string

func Load(bundlePath string) error {
	configPath := filepath.Join(bundlePath, "config.json")

	f, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", configPath, err)
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&Spec); err != nil {
		return fmt.Errorf("failed to decode config.json: %w", err)
	}

	RootfsPath = filepath.Join(bundlePath, Spec.Root.Path)

	return nil
}
