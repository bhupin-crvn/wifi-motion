package router

import (
	"github.com/gofiber/fiber/v2"
	utils "Kubernetes-api/kubeutils"
	helper "Kubernetes-api/helper"
)

// @Description	Get Detail of resouce avilable in kubernetes
// @Summary		Get Detail of resouce avilable in kubernetes for each nodes
// @Tags		Resources 
// @Accept		json
// @Produce		json
// @Router		/api/resource [get]

var kc = utils.NewKubernetesConfig()

func GetResources(c *fiber.Ctx) error {
	resource, err := kc.GetRemainingNodeResources()
	if err != nil {
		return helper.SendResponse(c, err.Error(), nil, fiber.ErrBadRequest.Code)
	}
	return helper.SendResponse(c, "Resorce Requested sucessfully", resource, fiber.StatusOK)
}

// @Description	Get Detail of resouce avilable in kubernetes
// @Summary		Get Detail of resouce avilable in kubernetes for each nodes
// @Tags		Resources 
// @Accept		json
// @Produce		json
// @Router		/api/resource [get]
func GetTotalResouces(c *fiber.Ctx) error {
	resource, err := kc.GetNodeTotalResources()
	if err != nil {
		return helper.SendResponse(c, err.Error(), nil, fiber.ErrBadRequest.Code)
	}
	return helper.SendResponse(c, "Resorce Requested sucessfully", resource, fiber.StatusOK)
}

// @Description	Get Detail of resouce avilable in kubernetes
// @Summary		Get Detail of resouce avilable in kubernetes for each nodes
// @Tags		Resources 
// @Accept		json
// @Produce		json
// @Router		/api/resource [get]
func GetClusterResources(c *fiber.Ctx) error {
	
	resource, err := kc.GetClusterNodeResources()
	if err != nil {
		return helper.SendResponse(c, err.Error(), nil, fiber.ErrBadRequest.Code)
	}
	return helper.SendResponse(c, "Resorce Requested sucessfully", resource, fiber.StatusOK)
}

func CheckHealth(c *fiber.Ctx) error {
	return helper.SendResponse(c, "OK", nil, fiber.StatusOK)
}