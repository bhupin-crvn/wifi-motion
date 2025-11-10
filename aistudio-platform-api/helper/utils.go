package helper

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type APIResponse struct {
	Message    string      `json:"message,omitempty"`
	StatusCode int         `json:"statusCode"`
	Status     bool        `json:"status"`
	Data       interface{} `json:"data"`
}

func SendResponse(c *fiber.Ctx, message string, data interface{}, statuscode int) error {

	status := false
	if statuscode == 200 {
		status = true
	}
	response := APIResponse{
		Message:    message,
		StatusCode: statuscode,
		Status:     status,
		Data:       data,
	}

	c.Status(statuscode)
	return c.JSON(response)
}

func ExtractZip(src, dest string) ([]string, error) {
	var extractedFiles []string

	r, err := zip.OpenReader(src)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return nil, fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return nil, fmt.Errorf("failed to create directory %s: %w", fpath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create parent directory for %s: %w", fpath, err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return nil, fmt.Errorf("failed to create file %s: %w", fpath, err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return nil, fmt.Errorf("failed to open zip entry %s: %w", f.Name, err)
		}

		_, err = io.Copy(outFile, rc)
		if err != nil {
			outFile.Close()
			rc.Close()
			return nil, fmt.Errorf("failed to write file %s: %w", fpath, err)
		}

		outFile.Close()
		rc.Close()
		extractedFiles = append(extractedFiles, fpath)
	}

	return extractedFiles, nil
}

// Helper: Download file from URL
func DownloadFile(url, filepath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
