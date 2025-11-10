package enginetemplate

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type Template struct {
	TemplateVersion string `json:"templateVersion"`
	TemplateBaseURL string `json:"templateUrl"`
	ExpPath         string `json:"expPath"`
}

func (t *Template) GetTemplate(gitToken string) error {
	log.Printf("Pulling engine template from: %s, version: %s", t.TemplateBaseURL, t.TemplateVersion)

	// Use the git token in the Git clone URL
	gitCloneURL := fmt.Sprintf("%s", t.TemplateBaseURL)

	cloneOptions := &git.CloneOptions{
		Auth: &http.BasicAuth{
			Username: "studio", // GitHub API ignores the username and relies on the token
			Password: gitToken,
		},
		URL:      gitCloneURL,
		Progress: os.Stdout,
	}

	// Check if the version is a tag or a branch
	if isTag(t.TemplateVersion) {
		log.Printf("Cloning tag: %s", t.TemplateVersion)
		cloneOptions.ReferenceName = plumbing.NewTagReferenceName(t.TemplateVersion)
	} else {
		log.Printf("Cloning branch: %s", t.TemplateVersion)
		cloneOptions.ReferenceName = plumbing.NewBranchReferenceName(t.TemplateVersion)
	}

	_, err := git.PlainClone(t.ExpPath, false, cloneOptions)
	if err != nil {
		log.Printf("Downloading failed: template from: %s, version: %s, error: %v", gitCloneURL, t.TemplateVersion, err)
		return err
	}

	// Update file and directory permissions
	err = filepath.Walk(t.ExpPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.Chmod(path, 0777) // Directories need executable permissions
		}
		return os.Chmod(path, 0666) // Files can have write permissions
	})
	if err != nil {
		log.Printf("Failed to change file/directory permissions: %v", err)
		return err
	}

	log.Printf("Downloading successful: template from: %s, version: %s", t.TemplateBaseURL, t.TemplateVersion)
	return nil
}

// Helper function to determine if the version is a tag
func isTag(version string) bool {
	// Regex pattern for matching tags like "v0.1.2", "v0.1", "v1",
	// as well as versions with suffixes like "v0.1.1-stableBlank" or "v0.1.1-stableDemo"
	tagPattern := `^v\d+(\.\d+)*(-[a-zA-Z0-9]+)?$`
	re := regexp.MustCompile(tagPattern)
	return re.MatchString(version)
}

