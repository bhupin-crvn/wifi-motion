package llm


import (
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(router fiber.Router) {
	llmdeploy := router.Group("/llm")
	llmdeploy.Get("/", GetDefaultLlms)
	llmdeploy.Post("/", CreateLLMDeployment)
	llmdeploy.Get("/backendtype", GetSupportedBackend)
	llmdeploy.Delete("/:id", DeleteLLMDeployment)
}
