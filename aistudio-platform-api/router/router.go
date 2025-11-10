package router

import (
	model "Kubernetes-api/deployments"
	artifacts "Kubernetes-api/artifacts"
	llm "Kubernetes-api/llm"
	"Kubernetes-api/enginetemplate"
	JupyterLabs "Kubernetes-api/labs/jupyterlabs"
	plugin "Kubernetes-api/plugin"
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	api := app.Group("/api")
	api.Get("/resources", GetResources)
	api.Get("/totalresources", GetTotalResouces)
	api.Get("/clusterresources", GetClusterResources)
	api.Get("health/check", CheckHealth)
	JupyterLabs.SetupRoutes(api)
	enginetemplate.SetupRoutes(api)
	artifacts.SetupRoutes(api)
	model.SetupRoutes(api)
	llm.SetupRoutes(api)
	plugin.SetupRoutes(api)
}
