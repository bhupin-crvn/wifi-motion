package plugin

import (
	"Kubernetes-api/helper"
	"Kubernetes-api/internal/sse"
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/valyala/fasthttp"
)

// @Description	Create Plugin Deployments Environment based on the specific user and plugin
// @Summary		Create Plugin Deployments
// @Tags		Plugins
// @Accept		json
// @Produce		json
// @Param 		createPluginDeploymentsRequest body PluginDeploymentsRequest true "Plugin Deployments Body"
// @Router		/api/plugin/deploy [post]
func CreatePlugin(c *fiber.Ctx) error {
	var req PluginDeploymentsRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}

	// Create a channel to receive the result and error from the goroutine
	resultChan := make(chan string)
	errorChan := make(chan error)

	go func() {
		var (
			result string
			err    error
		)

		if req.EngineKey != "" && req.ReleaseId != 0 {
			result, err = InstallPlugin(req)
		} else {
			result, err = CreatePluginDeployments(req)
		}

		resultChan <- result
		errorChan <- err
	}()

	url := <-resultChan
	err := <-errorChan

	if err != nil {
		log.Info(err)
		return helper.SendResponse(c, "Failed to create plugin deployment", nil, fiber.ErrBadRequest.Code)
	}

	message := "Plugin Deployment Created Successfully"
	log.Info(message)

	data := map[string]interface{}{
		"frontendUrl": strings.Split(url, ",")[0], // extract frontend URL
		"backendUrl":  strings.Split(url, ",")[1], // extract backend URL
	}

	return helper.SendResponse(c, message, data, fiber.StatusOK)
}

// @Description	Delete Plugin Deployments (frontend + backend) based on pluginName and serviceName
// @Summary		Delete Plugin Deployments
// @Tags		Plugins
// @Produce		json
// @Param		pluginName path string true "Plugin Name"
// @Param		serviceName path string true "Service Name"
// @Router		/api/plugin/deploy/{pluginName}/{serviceName} [delete]
func DeletePlugin(c *fiber.Ctx) error {
	var req DeletePluginRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.SendResponse(c, "Invalid request body", nil, fiber.StatusBadRequest)
	}

	if req.PluginName == "" || req.RoutePath == "" {
		return helper.SendResponse(c, "pluginName and routePath are required", nil, fiber.StatusBadRequest)
	}
	err := DeletePluginDeployments(req.PluginName, req.RoutePath)
	if err != nil {
		log.Error("Failed to delete plugin deployments: %v", err)
		return helper.SendResponse(c, "Failed to delete deployments", nil, fiber.StatusInternalServerError)
	}

	log.Info("Delete request for plugin=%s route=%s", req.PluginName, req.RoutePath)
	return helper.SendResponse(c, "Deployments deleted successfully", nil, fiber.StatusOK)
}

func GetPluginsSse(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(wr *bufio.Writer) {
		fmt.Println("WRITER PLUGIN")
		em := sse.NewBufioEmitter(wr, "notebooks")
		for {
			data, err := ListPlugins()
			if err != nil {
				log.Error("error listing notebooks for SSE: ", err)
				continue
			}

			sendJsonFlowErr := em.SendJSON("", "message", data)
			if sendJsonFlowErr.Err != nil {
				if sendJsonFlowErr.Next {
					continue
				} else {
					break
				}
			}

			time.Sleep(5 * time.Second)
		}
	}))

	return nil
}
