package plugin

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type databaseCredentials struct {
	DSN       string    `json:"dsn"`
	Driver    string    `json:"driver"`
	User      string    `json:"user"`
	Database  string    `json:"database"`
	CreatedAt time.Time `json:"createdAt"`
}

func ensureDatabaseCredentials(artifactDir, identifier string, manifest PluginManifest) (*databaseCredentials, bool, error) {
	path := filepath.Join(artifactDir, "database.json")
	if data, err := os.ReadFile(path); err == nil {
		var creds databaseCredentials
		if err := json.Unmarshal(data, &creds); err != nil {
			return nil, false, fmt.Errorf("parse stored database credentials: %w", err)
		}
		return &creds, false, nil
	}

	driver := manifest.Permissions.Database.Driver
	if driver == "" {
		driver = "postgres"
	}
	user := manifest.Permissions.Database.User
	if user == "" {
		user = fmt.Sprintf("%s_user", identifier)
	}
	dbName := manifest.Permissions.Database.Database
	if dbName == "" {
		dbName = fmt.Sprintf("%s_db", identifier)
	}

	password, err := generateSecret(24)
	if err != nil {
		return nil, false, err
	}

	dsn := fmt.Sprintf("%s://%s:%s@%s-db/%s", driver, user, password, identifier, dbName)
	creds := &databaseCredentials{
		DSN:       dsn,
		Driver:    driver,
		User:      user,
		Database:  dbName,
		CreatedAt: time.Now().UTC(),
	}

	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return nil, false, fmt.Errorf("prepare artifact dir for db: %w", err)
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return nil, false, fmt.Errorf("marshal database credentials: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, false, fmt.Errorf("store database credentials: %w", err)
	}
	return creds, true, nil
}

func rollbackDatabaseCredentials(artifactDir string) error {
	return os.Remove(filepath.Join(artifactDir, "database.json"))
}

func generateSecret(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var builder strings.Builder

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random secret: %w", err)
	}

	for _, b := range bytes {
		builder.WriteByte(alphabet[int(b)%len(alphabet)])
	}
	return builder.String(), nil
}
