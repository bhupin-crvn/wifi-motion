package llm

import (
	"Kubernetes-api/helper"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// @Description	Create Jupyter ModelDeployments Environment based on the specific users and project
// @Summary		Create ModelDeployments Environment
// @Tags		JupyterLabs ModelDeployments
// @Accept		json
// @Produce		json
// @Param 		createModelDeploymentsRequest body CreateModelDeploymentsRequest true "ModelDeployments Body"
// @Router		/api/modeldeplyment [post]
// @Router		/api/modeldeplyment [post]
func CreateLLMDeployment(c *fiber.Ctx) error {
	var req CreateLlmDeploymentsRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}

	// Create a channel to receive the result and error from the goroutine
	resultChan := make(chan string)
	errorChan := make(chan error)

	go func() {
		url, err := CreateLlmDeployments(req)
		resultChan <- url
		errorChan <- err
	}()

	url := <-resultChan
	err := <-errorChan

	if err != nil {
		log.Info(err)
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}
	message := "LLM Deployment Created Sucessfully"
	log.Info(message)

	data := map[string]interface{}{
		"inferenceUrl": url,
	}
	return helper.SendResponse(c, message, data, fiber.StatusOK)
}

// @Description	Delete or Stop a specific notebook experiments
// @Summary		Delete Specific notebook
// @Tags		JupyterLabs ModelDeployments
// @Accept		json
// @Param 		id  path string true "Pod Username"
// @Produce		json
// @Router		/api/modeldeplyment/{id} [delete]
func DeleteLLMDeployment(c *fiber.Ctx) error {
	deploymentName := c.Params("id")
	DeleteLlmDeployments(deploymentName)
	log.Info("Delete request for LLM Deployment: ", deploymentName)
	return helper.SendResponse(c, "LLM deleted successfully", nil, fiber.StatusOK)
}

func GetDefaultLlms(c *fiber.Ctx) error{
	path := "./artifact/pvc-llm/"
	folders, err := getFolderNames(path)
	if err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}
	return helper.SendResponse(c, "querry list of default LLm", folders, fiber.StatusOK)
}

func GetSupportedBackend(c *fiber.Ctx) error {
	data := map[string][]string{
		// "backendType": {"vllm_model", "python", "tensorRT"},
		"backendType": {"vllm_model"},
	}

	return helper.SendResponse(c, "query list of default backend", data, fiber.StatusOK)
}