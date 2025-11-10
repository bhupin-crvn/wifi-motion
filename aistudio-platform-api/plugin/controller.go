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

	result, err := installPluginCore(req)
	if err != nil {
		log.Warnf("failed to install plugin: %v", err)
		return helper.SendResponse(c, err.Error(), nil, fiber.ErrBadRequest.Code)
	}

	message := formatInstallMessage(result)
	log.Info(message)

	data := map[string]interface{}{
		"frontendUrl":       result.FrontendURL,
		"backendUrl":        result.BackendURL,
		"namespace":         result.Namespace,
		"artifactsLocation": result.ArtifactsLocation,
		"manifestPath":      result.ManifestPath,
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

	engineKey := strings.TrimSpace(req.EngineKey)
	if engineKey == "" {
		engineKey = strings.TrimSpace(req.PluginName)
	}
	routePath := strings.TrimSpace(req.RoutePath)

	if engineKey == "" || routePath == "" {
		return helper.SendResponse(c, "engineKey/pluginName and routePath are required", nil, fiber.StatusBadRequest)
	}
	err := DeletePluginDeployments(engineKey, routePath)
	if err != nil {
		log.Errorf("Failed to delete plugin deployments: %v", err)
		return helper.SendResponse(c, "Failed to delete deployments", nil, fiber.StatusInternalServerError)
	}

	log.Infof("Delete request for plugin=%s route=%s", engineKey, routePath)
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
