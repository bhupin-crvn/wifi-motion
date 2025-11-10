package enginetemplate

import (
	"os"

	"github.com/gofiber/fiber/v2"
)

// @Description	Download the template and add in the project
// @Summary		Download Template API for Experiment
// @Tags		Template
// @Accept		json
// @Produce		json
// @Param 		template body Template true "Template Body"
// @Router		/api/template [post]
func AddTemplateInExperiment(c *fiber.Ctx) error {
	var t Template
	if err := c.BodyParser(&t); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	gitToken := os.Getenv("git_token")
	err := t.GetTemplate(gitToken)
	if err != nil {
		return c.Status(200).JSON(fiber.Map{"message": err.Error()})
	}
	return c.Status(200).JSON(fiber.Map{"message": "Template Downloaded Successfully"})
}

func GetTemplateValidation(c *fiber.Ctx) error {

	url := c.Query("templateURL")
	version := c.Query("version")
	gitToken := os.Getenv("git_token")
	flag, _ := IsValidGitHubRepo(url, version, gitToken)
	if !flag {
		return c.Status(400).JSON(fiber.Map{"message": "invalid github repo"})
	}
	return c.Status(200).JSON(fiber.Map{"message": "valid github repo"})
}
