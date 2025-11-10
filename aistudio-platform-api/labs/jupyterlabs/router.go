package JupyterLabs

import (
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(router fiber.Router) {
	notebooks := router.Group("/notebooks")
	notebooks.Post("/", CreateNotebooks)
	notebooks.Post("/restart", RestartNotebooks)
	notebooks.Post("/clone-artifacts", CloneArtifactsCreateNotebook)
	notebooks.Get("/", GetNotebooks)
	notebooks.Get("/sse", GetNotebooksSse)
	notebooks.Get("/metrics", GetLabsMetrics)
	notebooks.Get("/preview", LabFilesPreview)
	notebooks.Get("/:id", GetOneNotebookHandler)
	notebooks.Delete("/stop/:id", StopNotebookHandler)
	notebooks.Delete("/:id", DeleteNotebookHandler)
}