package JupyterLabs

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"

	"Kubernetes-api/artifacts"
	"Kubernetes-api/enginetemplate"
	"Kubernetes-api/helper"
	"Kubernetes-api/internal/sse"
)

// CreateNotebooks handles the creation of a Jupyter notebook environment.
// @Description Create Jupyter Notebook Environment based on the specific users and project
// @Summary Create Notebook Environment
// @Tags JupyterLabs Notebook
// @Accept json
// @Produce json
// @Param createNotebookRequest body CreateLabRequest true "Notebook Body"
// @Router /api/notebooks [post]
func CreateNotebooks(c *fiber.Ctx) error {
	var request CreateLabRequest
	if err := c.BodyParser(&request); err != nil {
		log.Error("error parsing request body: ", err)
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}

	gitToken := os.Getenv(GitTokenEnv)
	if valid, err := enginetemplate.IsValidGitHubRepo(request.TemplateBaseURL, request.TemplateVersion, gitToken); !valid {
		log.Error("failed to validate GitHub repository: ", err)
		return helper.SendResponse(c, "Git Token Error For Template Download", nil, fiber.StatusInternalServerError)
	}

	message, err := CreateNotebook(
		request.Username, request.Password, request.CPURequest, request.GPURequest,
		request.MemoryRequest, request.CPULimit, request.MemoryLimit, request.DiskStorage,
		request.NodeSelector, request.WorkSpaceType, request.LabspaceType,
	)
	if err != nil {
		log.Error("failed to create notebook: ", err, message)
		return helper.SendResponse(c, "Failed to create labspace", nil, fiber.StatusInternalServerError)
	}

	errChan := make(chan error, 1)
	go func() {
		template := enginetemplate.Template{
			TemplateBaseURL: request.TemplateBaseURL,
			TemplateVersion: request.TemplateVersion,
			ExpPath:         fmt.Sprintf("%s%s%s", ArtifactPathPrefix, PersistentVolumePrefix, request.Username+PersistentVolumeSuffix),
		}

		if err := template.GetTemplate(gitToken); err != nil {
			log.Error("failed to get template: ", err)
			if delErr := DeleteNotebook(request.Username); delErr != nil {
				logrus.Errorf("failed to clean up notebook after template error: %v", delErr)
			}
			errChan <- helper.SendResponse(c, "Error in creating labspace", nil, fiber.StatusBadRequest)
			return
		}
		errChan <- nil
	}()

	if err := <-errChan; err != nil {
		return err
	}

	log.Info("notebook created successfully with template: ", request.TemplateBaseURL, request.TemplateVersion)
	return helper.SendResponse(c, "Labspace created successfully", nil, fiber.StatusOK)
}

// RestartNotebooks handles the restart of a Jupyter notebook environment.
// @Description Create Jupyter Notebook Environment based on the specific users and project
// @Summary Create Notebook Environment
// @Tags JupyterLabs Notebook
// @Accept json
// @Produce json
// @Param createNotebookRequest body CreateLabRequest true "Notebook Body"
// @Router /api/notebooks [post]
func RestartNotebooks(c *fiber.Ctx) error {
	var request CreateLabRequest
	if err := c.BodyParser(&request); err != nil {
		log.Error("error parsing request body: ", err)
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}

	message, err := CreateNotebook(
		request.Username, request.Password, request.CPURequest, request.GPURequest,
		request.MemoryRequest, request.CPULimit, request.MemoryLimit, request.DiskStorage,
		request.NodeSelector, request.WorkSpaceType, request.LabspaceType,
	)
	if err != nil {
		log.Error("failed to restart notebook: ", err)
		return helper.SendResponse(c, "Failed to restart labspace", nil, fiber.ErrBadRequest.Code)
	}

	log.Info("notebook restarted successfully: ", message)
	return helper.SendResponse(c, "Labspace restarted successfully", nil, fiber.StatusOK)
}

// GetNotebooks retrieves a list of all Jupyter notebook environments.
// @Description Get Jupyter Notebook Environment Lists and Details
// @Summary Get List of Jupyter Notebook
// @Tags JupyterLabs Notebook
// @Produce json
// @Router /api/notebooks [get]
func GetNotebooks(c *fiber.Ctx) error {
	data, err := ListNotebooks()
	if err != nil {
		log.Error("error listing notebooks: ", err)
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}

	var notebooks []Notebook
	for _, element := range data {
		restart, err := strconv.Atoi(element["restarts"])
		if err != nil {
			log.Warn("error parsing restarts for notebook: ", element["name"])
			continue
		}
		notebook := Notebook{
			Name:    element["name"],
			Ready:   element["ready"],
			Status:  element["status"],
			Restart: uint(restart),
			Age:     element["age"],
		}
		notebooks = append(notebooks, notebook)
	}

	return helper.SendResponse(c, "Labspace list retrieved successfully", notebooks, fiber.StatusOK)
}

// GetNotebooksSse streams notebook status as server-sent events.
// @Description Get the data of all the pods with status as server sent events
// @Summary Get Notebook status and details server sent events
// @Tags JupyterLabs Notebook
// @Accept json
// @Produce text/event-stream
// @Router /api/notebooks/sse [get]
func GetNotebooksSse(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(wr *bufio.Writer) {
		fmt.Println("WRITER LABS")
		em := sse.NewBufioEmitter(wr, "notebooks")
		for {
			data, err := ListNotebooks()
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

// DeleteNotebookHandler deletes a specific notebook and its resources.
// @Description Delete a specific notebook experiments delete with all the files and resource behind it
// @Summary Delete Specific notebook
// @Tags JupyterLabs Notebook
// @Accept json
// @Param id path string true "Pod Username"
// @Produce json
// @Router /api/notebooks/{id} [delete]
func DeleteNotebookHandler(c *fiber.Ctx) error {
	username := c.Params("id")
	if err := DeleteNotebook(username); err != nil {
		log.Error("error deleting notebook: ", err)
		return helper.SendResponse(c, "Failed to delete labspace", nil, fiber.StatusInternalServerError)
	}

	log.Info("deleted notebook for user: ", username)
	return helper.SendResponse(c, "Labspace deleted successfully", nil, fiber.StatusOK)
}

// StopNotebookHandler stops a specific notebook.
// @Description Stop a specific notebook experiments
// @Summary Delete Specific notebook
// @Tags JupyterLabs Notebook
// @Accept json
// @Param id path string true "Pod Username"
// @Produce json
// @Router /api/notebooks/{id} [delete]
func StopNotebookHandler(c *fiber.Ctx) error {
	username := c.Params("id")
	if err := StopNotebook(username); err != nil {
		log.Error("error stopping notebook: ", err)
		return helper.SendResponse(c, "Failed to stop labspace", nil, fiber.StatusInternalServerError)
	}

	log.Info("stopped notebook for user: ", username)
	return helper.SendResponse(c, "Labspace stopped successfully", nil, fiber.StatusOK)
}

// GetOneNotebookHandler retrieves details of a single notebook.
// @Description Get Detail of Single JupyterLab Notebook Pods
// @Summary Get Detail of Single JupyterLab Notebook
// @Tags JupyterLabs Notebook
// @Accept json
// @Param id path string true "Pod Username"
// @Produce json
// @Router /api/notebooks/{id} [get]
func GetOneNotebookHandler(c *fiber.Ctx) error {
	username := c.Params("id")
	element, err := GetOneNotebook(username)
	if err != nil {
		log.Error("error retrieving notebook details: ", err)
		return helper.SendResponse(c, err.Error(), nil, fiber.StatusBadRequest)
	}

	restart, err := strconv.Atoi(element["restarts"])
	if err != nil {
		log.Warn("error parsing restarts for notebook: ", username)
	}
	notebook := Notebook{
		Name:    element["name"],
		Ready:   element["ready"],
		Status:  element["status"],
		Restart: uint(restart),
		Age:     element["age"],
	}

	return helper.SendResponse(c, "Labspace retrieved successfully", notebook, fiber.StatusOK)
}

// CloneArtifactsCreateNotebook creates a notebook by cloning artifacts.
// @Description Create Jupyter Notebook Environment based on the specific users and project
// @Summary Create Notebook Environment by cloning artifacts form registered model
// @Tags JupyterLabs Notebook
// @Accept json
// @Produce json
// @Param createNotebookRequest body CloneNotebookRequest true "Notebook Body"
// @Router /api/notebooks/cloneartifacts [post]
func CloneArtifactsCreateNotebook(c *fiber.Ctx) error {

	var request CloneNotebookRequest
	if err := c.BodyParser(&request); err != nil {
		log.Error("error parsing clone request body: ", err)
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}

	message, err := CloneArtifactsNotebook(request)
	if err != nil {
		log.Error("error cloning artifacts notebook: ", err)
		return helper.SendResponse(c, message, nil, fiber.ErrBadRequest.Code)
	}

	return helper.SendResponse(c, "Labspace created successfully with model request", nil, fiber.StatusOK)
}

// LabFilesPreview previews files in a labspace.
// @Description Preview files in a specific labspace
// @Summary Preview labspace files
// @Tags JupyterLabs Notebook
// @Accept json
// @Produce json
// @Router /api/notebooks/files [get]
func LabFilesPreview(c *fiber.Ctx) error {
	username := c.Query("username")
	filename := c.Query("filename")
	path := fmt.Sprintf("%s%s%s", ArtifactPathPrefix, PersistentVolumePrefix, username+PersistentVolumeSuffix)

	result, err := artifacts.ReadFiles(path, filename)
	if err != nil {
		log.Error("error reading labspace file: ", err)
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}

	return helper.SendResponse(c, "Labspace file preview successful", result, fiber.StatusOK)
}

// GetLabsMetricsSse streams labspace metrics as server-sent events.
// @Description Get labspace metrics as server sent events
// @Summary Get labspace metrics server sent events
// @Tags JupyterLabs Notebook
// @Accept json
// @Produce text/event-stream
// @Router /api/notebooks/metrics/sse [get]
func GetLabsMetricsSse(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		em := sse.NewBufioEmitter(w, "pod metrics")
		for {
			podMetrics, err := GetLabspacesMetrics()
			if err != nil {
				log.Errorf("error getting pod metrics: %v", err)
				break
			}

			sendJsonFlowErr := em.SendJSON("", "message", podMetrics)
			if sendJsonFlowErr.Err != nil {
				break
			}

			time.Sleep(10 * time.Second)
		}
	}))

	return nil
}

// GetLabsMetrics retrieves labspace metrics.
// @Description Get labspace metrics
// @Summary Get labspace metrics
// @Tags JupyterLabs Notebook
// @Accept json
// @Produce json
// @Router /api/notebooks/metrics [get]
func GetLabsMetrics(c *fiber.Ctx) error {
	podMetrics, err := GetLabspacesMetrics()
	if err != nil {
		log.Errorf("Error getting lab metrics: %v", err)
		return helper.SendResponse(c, "Error fetching lab metrics", nil, fiber.StatusInternalServerError)
	}

	return helper.SendResponse(c, "Labs metrics fetched successfully", podMetrics, fiber.StatusOK)
}
