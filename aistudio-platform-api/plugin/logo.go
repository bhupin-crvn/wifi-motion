package plugin

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Kubernetes-api/helper"
)

func processLogo(manifest PluginManifest, extractDir, artifactDir string) (string, string, error) {
	var sourcePath string
	var cleanup func()

	if manifest.Logo.Path != "" {
		sourcePath = manifest.Logo.Path
		if !filepath.IsAbs(sourcePath) {
			sourcePath = filepath.Join(extractDir, manifest.Logo.Path)
		}
		if _, err := os.Stat(sourcePath); err != nil {
			sourcePath = ""
		}
	}

	if sourcePath == "" && manifest.Logo.URL != "" {
		tmpPath := filepath.Join(artifactDir, fmt.Sprintf("logo-%d.tmp", time.Now().UnixNano()))
		if err := helper.DownloadFile(manifest.Logo.URL, tmpPath); err != nil {
			return "", "", fmt.Errorf("download logo: %w", err)
		}
		sourcePath = tmpPath
		cleanup = func() {
			_ = os.Remove(tmpPath)
		}
	}

	if sourcePath == "" {
		return "", "", nil
	}
	if cleanup != nil {
		defer cleanup()
	}

	if err := os.MkdirAll(filepath.Join(artifactDir, "assets"), 0o755); err != nil {
		return "", "", fmt.Errorf("prepare logo dir: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(sourcePath))
	if ext == "" {
		ext = ".png"
	}
	destPath := filepath.Join(artifactDir, "assets", "logo"+ext)

	var backupPath string
	if _, err := os.Stat(destPath); err == nil {
		backupPath = fmt.Sprintf("%s.bak-%d", destPath, time.Now().UnixNano())
		if err := os.Rename(destPath, backupPath); err != nil {
			return "", "", fmt.Errorf("backup existing logo: %w", err)
		}
	}

	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return "", "", fmt.Errorf("open logo source: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return "", "", fmt.Errorf("create logo dest: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return "", "", fmt.Errorf("copy logo: %w", err)
	}

	return destPath, backupPath, nil
}

func restoreLogoBackup(newPath, backupPath string) error {
	if newPath != "" {
		_ = os.Remove(newPath)
	}
	if backupPath == "" {
		return nil
	}
	return os.Rename(backupPath, newPath)
}
