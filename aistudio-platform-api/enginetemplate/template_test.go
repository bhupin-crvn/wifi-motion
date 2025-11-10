package enginetemplate

import (
	"os"
	"testing"

	"github.com/spf13/afero"
)

var expPath = "/tmp/foo/19"

func TestGetTemplate(t *testing.T) {
	templateEngine := Template{
		TemplateBaseURL: "https://github.com/go-git/go-git/",
		TemplateVersion: "v5.4.2",
		ExpPath:         expPath,
	}
	gitToken := os.Getenv("git_token")
	err := templateEngine.GetTemplate(gitToken)

	if err != nil {
		t.Error("Error downloading the template file", err)
	}

	AppFs := afero.NewOsFs()

	s, err := afero.Exists(AppFs, expPath+"/status.go")

	if s == false || err != nil {
		t.Error("Error Downloading the template file", err)
	}
}
