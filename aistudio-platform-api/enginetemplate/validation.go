package enginetemplate

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2/log"
	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
)

func IsValidGitHubRepo(url string, version string, gitToken string) (bool, error) {
	
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gitToken}, 
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	parts := strings.Split(strings.TrimPrefix(url, "https://github.com/"), "/")
	if len(parts) != 2 {
		log.Error("Invalid GitHub URL format")
		return false, fmt.Errorf("invalid GitHub URL format")
	}
	owner, repo := parts[0], parts[1]

	repoInfo, resp, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		if resp != nil {
			log.Errorf("Error fetching repository info: %v", err)
			log.Errorf("HTTP Response: %v", resp)
			if resp.StatusCode == http.StatusNotFound {
				return false, fmt.Errorf("repository not found")
			} else if resp.StatusCode == http.StatusUnauthorized {
				return false, fmt.Errorf("unauthorized access to the repository")
			}
		} else {
			log.Errorf("Error fetching repository info with no response: %v", err)
		}
		return false, err
	}

	if repoInfo.GetPrivate() && resp.StatusCode == http.StatusUnauthorized {
		log.Error("Unauthorized access to the private repository")
		return false, fmt.Errorf("unauthorized access to the private repository")
	}

	return true, nil
}

