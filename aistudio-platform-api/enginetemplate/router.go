package enginetemplate

import (
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(router fiber.Router) {
	etemplate := router.Group("/template")
	etemplate.Post("/", AddTemplateInExperiment)
	etemplate.Get("/validate", GetTemplateValidation)

}
