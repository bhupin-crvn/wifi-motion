package deployments

import (
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(router fiber.Router) {
	modeldeployment := router.Group("/modeldeployment")
	modeldeployment.Post("/", CreateModelDeployment)
	modeldeployment.Get("/", GetModelDeployments)
	modeldeployment.Get("/describepod", GetPodDescription)
	modeldeployment.Get("/sse", GetModelsSse)
	modeldeployment.Get("/metrics", GetModelMetrics)
	modeldeployment.Get("/logs", GetModelDeploymentLogs)
	modeldeployment.Delete("/:id", DeleteModelDeployment)
	modeldeployment.Get("/:id", GetOneDeployment)
}
