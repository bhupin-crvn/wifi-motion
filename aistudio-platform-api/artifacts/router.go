package artifacts

import (
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(router fiber.Router) {
	artifact := router.Group("/artifacts")
	artifact.Get("/model", modelArtifacts)
	artifact.Post("/register", registerModel)
	artifact.Get("/preview", filesPreview)
	artifact.Get("/labpreview", labFilePreview)
	artifact.Get("/:id", getNotebookArtifacts)
	artifact.Delete("/deletemodel", deleteModel)
}
