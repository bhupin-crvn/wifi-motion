package deployments

import (
	"Kubernetes-api/helper"
	"Kubernetes-api/internal/sse"
	utils "Kubernetes-api/kubeutils"
	"bufio"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"

	"github.com/valyala/fasthttp"
)

// @Description	Create Jupyter ModelDeployments Environment based on the specific users and project
// @Summary		Create ModelDeployments Environment
// @Tags		JupyterLabs ModelDeployments
// @Accept		json
// @Produce		json
// @Param 		createModelDeploymentsRequest body CreateModelDeploymentsRequest true "ModelDeployments Body"
// @Router		/api/modeldeplyment [post]
// @Router		/api/modeldeplyment [post]
func CreateModelDeployment(c *fiber.Ctx) error {
	var req CreateModelDeploymentsRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}

	// Create a channel to receive the result and error from the goroutine
	resultChan := make(chan string)
	errorChan := make(chan error)

	go func() {
		url, err := CreateModelDeployments(req.Username, req.DeploymentName, req.Modelname, req.Version, req.Modelartifacts, req.CPURequest, req.GPURequest, req.MemoryRequest, req.CPULimit, req.MemoryLimit, req.DiskStorage, req.NodeSelector)
		// CreateLLMDeployments
		// url, err := CreateLLMDeployments(req.Username, req.DeploymentName, req.Modelname, req.Version, req.Template, req.Modelartifacts, req.CPURequest, req.GPURequest, req.MemoryRequest, req.CPULimit, req.MemoryLimit, req.DiskStorage, req.NodeSelector)
		resultChan <- url
		errorChan <- err
	}()

	url := <-resultChan
	err := <-errorChan

	if err != nil {
		log.Info(err)
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}
	message := "Model Deployment Created Sucessfully"
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
func DeleteModelDeployment(c *fiber.Ctx) error {
	podUsername := c.Params("id")
	DeleteModelDeployments(podUsername)
	log.Info("Delete request for pod: ", podUsername)
	return helper.SendResponse(c, "Deployments deleted successfully", nil, fiber.StatusOK)
}

// @Description	Get Jupyter ModelDeployments Environment Lists and Details
// @Summary		Get List of Jupyter ModelDeployments
// @Tags		JupyterLabs ModelDeployments
// @Produce		json
// @Router		/api/modeldeplyment [get]
func GetModelDeployments(c *fiber.Ctx) error {
	data, err := ListModelDeployments()

	if err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}

	var models []Model

	for _, element := range data {
		restart, err := strconv.Atoi(element["restarts"])
		if err != nil {
			print("error in getting age")
		}
		model := Model{Name: element["name"], Ready: element["ready"], Status: element["status"], Restart: uint(restart), Age: element["age"]}
		models = append(models, model)
	}

	return helper.SendResponse(c, "Request labspace list sucessfully", models, fiber.StatusOK)
}

// @Description	Get Detail of Single MLModelDeployments Pods
// @Summary		Get Detail of Single MLModelDeployments
// @Tags		JupyterLabs ModelDeployments
// @Accept		json
// @Param 		id  path string true "Pod Username"
// @Produce		json
// @Router		/api/modeldeplyment/{id} [get]
func GetOneDeployment(c *fiber.Ctx) error {
	podUsername := c.Params("id")
	element, err := getOneDeployment(podUsername)

	if err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}

	restart, err := strconv.Atoi(element["restarts"])
	if err != nil {
		print("error in getting age")
	}
	model := Model{Name: element["name"], Ready: element["ready"], Status: element["status"], Restart: uint(restart), Age: element["age"]}

	return helper.SendResponse(c, "Request Model Sucessfully", model, fiber.StatusOK)
}

// @Description	Get the data of all the pods with status as server sent events
// @Summary		Get ModelDeployments status and details server sent events
// @Tags		JupyterLabs ModelDeployments
// @Accept		json
// @Produce		text/event-stream
// @Router		/api/modeldeplyment/sse [get]
func GetModelsSse(c *fiber.Ctx) error {
	timeGap := time.Duration(5) * time.Second
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(wr *bufio.Writer) {
		fmt.Println("WRITER MODEL")
		em := sse.NewBufioEmitter(wr, "model deployments")
		for {
			data, errs := ListModelDeployments()
			if errs != nil {
				log.Error("Error finding the list of model deployments")
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

			time.Sleep(timeGap)
		}
	}))

	return nil
}

func GetPodDescription(c *fiber.Ctx) error {

	podName := c.Query("deploymentName")
	data, err := getPodDescription(podName)
	if err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}
	return c.Status(200).JSON(data)
}

func GetModelDeploymentLogs(c *fiber.Ctx) error {
	deploymentName := c.Query("deploymentName")

	if deploymentName == "" {
		return helper.SendResponse(c, "Error Deployment Name Doesnot Exist", nil, 500)
	}
	ctx := c.Context()
	opts := utils.NewLogsOptions()
	opts.Namespace = modelNamespace
	opts.PodName = deploymentName

	c.Set("Content-Type", "text/plain; charset=utf-8")

	err := kc.GetDeploymentLog(deploymentName, opts.Namespace, ctx, opts, c.Response().BodyWriter())
	if err != nil {
		return helper.SendResponse(c, err.Error(), nil, 500)
	}
	return nil
}

func GetModelMetrics(c *fiber.Ctx) error {

	podMetrics, err := getModelMetrics()
	if err != nil {
		log.Errorf("Error getting pod metrics: %v", err)
		return helper.SendResponse(c, "Error fetching model metrics", nil, 500)
	}
	return helper.SendResponse(c, "Model metrics fetched successfully", podMetrics, 200)
}
