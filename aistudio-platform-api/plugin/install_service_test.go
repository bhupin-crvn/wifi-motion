package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseManifestFromFiles(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.yaml")
	err := os.WriteFile(manifestPath, []byte(`
name: Sample Plugin
description: Demo plugin
version: 1.2.3
engine_key: sample-engine
docker:
  frontend:
    image: sample/frontend
  backend:
    image: sample/backend
`), 0o644)
	if err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	res, err := parseManifestFromFiles([]string{manifestPath})
	if err != nil {
		t.Fatalf("expected manifest to parse, got error: %v", err)
	}
	if res.Manifest.EngineKey != "sample-engine" {
		t.Fatalf("expected engine key sample-engine, got %s", res.Manifest.EngineKey)
	}
	if res.Manifest.Docker.Backend.Image != "sample/backend" {
		t.Fatalf("unexpected backend image: %s", res.Manifest.Docker.Backend.Image)
	}
}

func TestEnsureDatabaseCredentials(t *testing.T) {
	dir := t.TempDir()
	artifactDir := filepath.Join(dir, "plugin")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("create artifact dir: %v", err)
	}

	manifest := PluginManifest{}
	manifest.Permissions.Database.Driver = "postgres"
	manifest.Permissions.Database.User = "custom_user"
	manifest.Permissions.Database.Database = "custom_db"

	creds, created, err := ensureDatabaseCredentials(artifactDir, "engine-x", manifest)
	if err != nil {
		t.Fatalf("ensure credentials: %v", err)
	}
	if !created {
		t.Fatalf("expected credentials to be created")
	}
	if creds.User != "custom_user" {
		t.Fatalf("expected custom user, got %s", creds.User)
	}

	// Ensure credentials are reused on subsequent calls.
	creds2, created2, err := ensureDatabaseCredentials(artifactDir, "engine-x", manifest)
	if err != nil {
		t.Fatalf("ensure credentials second call: %v", err)
	}
	if created2 {
		t.Fatalf("expected credentials to be reused")
	}
	if creds.DSN != creds2.DSN {
		t.Fatalf("expected DSN to be reused")
	}

	if err := rollbackDatabaseCredentials(artifactDir); err != nil {
		t.Fatalf("rollback credentials: %v", err)
	}
	if _, err := os.Stat(filepath.Join(artifactDir, "database.json")); !os.IsNotExist(err) {
		t.Fatalf("expected database.json to be removed")
	}
}
