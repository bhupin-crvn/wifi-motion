package plugin

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PluginManifest models the expected manifest.yaml structure embedded inside a
// plugin bundle.
type PluginManifest struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
	EngineKey   string `yaml:"engine_key"`

	Permissions struct {
		Database struct {
			Mode     string `yaml:"mode"`     // required|optional|none
			Existing string `yaml:"existing"` // identifier of existing DB to re-use
			Driver   string `yaml:"driver"`   // postgres|mysql|...
			User     string `yaml:"user"`     // desired DB user
			Database string `yaml:"database"` // desired database name
		} `yaml:"database"`
	} `yaml:"permissions"`

	Docker struct {
		Frontend struct {
			Image    string            `yaml:"image"`
			Tag      string            `yaml:"tag"`
			Env      map[string]string `yaml:"env"`
			Replicas int32             `yaml:"replicas"`
			Ports    []int32           `yaml:"ports"`
		} `yaml:"frontend"`
		Backend struct {
			Image     string            `yaml:"image"`
			Tag       string            `yaml:"tag"`
			Env       map[string]string `yaml:"env"`
			Replicas  int32             `yaml:"replicas"`
			Ports     []int32           `yaml:"ports"`
			Migration bool              `yaml:"migration"`
		} `yaml:"backend"`
	} `yaml:"docker"`

	Logo struct {
		Path string `yaml:"path"`
		URL  string `yaml:"url"`
	} `yaml:"logo"`

	Kubernetes struct {
		Namespace string            `yaml:"namespace"`
		Labels    map[string]string `yaml:"labels"`
	} `yaml:"kubernetes"`
}

// manifestParseResult groups the parsed manifest with metadata derived from the
// file system.
type manifestParseResult struct {
	Manifest  PluginManifest
	Path      string
	SHA256Hex string
	RawBytes  []byte
}

func parseManifestFromFiles(files []string) (*manifestParseResult, error) {
	for _, path := range files {
		base := strings.ToLower(filepath.Base(path))
		if base == "manifest.yaml" || base == "manifest.yml" {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read manifest: %w", err)
			}

			var manifest PluginManifest
			if err := yaml.Unmarshal(content, &manifest); err != nil {
				return nil, fmt.Errorf("parse manifest: %w", err)
			}

			if err := validateManifest(manifest); err != nil {
				return nil, err
			}

			hash := sha256.Sum256(content)
			return &manifestParseResult{
				Manifest:  manifest,
				Path:      path,
				SHA256Hex: fmt.Sprintf("%x", hash[:]),
				RawBytes:  content,
			}, nil
		}
	}
	return nil, fmt.Errorf("manifest.yaml not found in plugin bundle")
}

func validateManifest(manifest PluginManifest) error {
	if manifest.EngineKey == "" {
		return fmt.Errorf("manifest validation failed: engine_key is required")
	}
	if manifest.Name == "" {
		return fmt.Errorf("manifest validation failed: name is required")
	}
	if manifest.Version == "" {
		return fmt.Errorf("manifest validation failed: version is required")
	}
	if manifest.Docker.Frontend.Image == "" {
		return fmt.Errorf("manifest validation failed: docker.frontend.image is required")
	}
	if manifest.Docker.Backend.Image == "" {
		return fmt.Errorf("manifest validation failed: docker.backend.image is required")
	}
	return nil
}
