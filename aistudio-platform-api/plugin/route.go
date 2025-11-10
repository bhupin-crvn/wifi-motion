package plugin

import (
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(router fiber.Router) {
	plugin := router.Group("/plugin")
	plugin.Get("/sse", GetPluginsSse)
	plugin.Post("/", CreatePlugin)
	plugin.Delete("/", DeletePlugin)
	install := plugin.Group("/install")
	install.Post("/:identifier", InstallPluginHandler)
	install.Put("/:identifier", UpdatePluginHandler)
	install.Delete("/:identifier", RollbackPluginHandler)
}
