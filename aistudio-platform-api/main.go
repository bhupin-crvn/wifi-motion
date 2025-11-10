package main

import (
	"Kubernetes-api/router"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

//	@title			Fusemachines AI Studio Platform API
//	@version		v1.15.0
//	@host			localhost:8080
//	@BasePath		/

func main() {
	app := fiber.New()
	router.SetupRoutes(app)
	log.Fatal(app.Listen(":8080"))
}
