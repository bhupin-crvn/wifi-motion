package artifacts

import (
	"Kubernetes-api/helper"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/spf13/afero"
)

func getNotebookArtifacts(c *fiber.Ctx) error {

	userName := c.Params("id")
	path := "./" + "artifact/jl-" + userName + "-0/"

	ignorePath := path + "/.studioignore"
	result, err := GetArtifacts(path, ignorePath)
	if err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}
	return helper.SendResponse(c, "Notebook Artifacts Request Sucessfully", result, fiber.StatusOK)
}

func registerModel(c *fiber.Ctx) error {
	var register Register
	if err := c.BodyParser(&register); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	fs := afero.NewOsFs()
	username := register.Username
	modelname := register.Modelname
	version := strings.Replace(register.Version, ".", "-", -1)
	src := "./" + "artifact" + "/" + "jl-" + username + "-0"

	dst := "./" + "artifact/ModelRegistry/" + username + "/" + modelname + "-" + version
	exists, err := afero.DirExists(fs, src)
	if err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}
	if !exists {
		return helper.SendResponse(c, "Source Directory Doesnot Exist", nil, 400)
	}
	result, err := CopyAllArtifacts(src, dst)
	if err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}
	log.Info(result)
	return helper.SendResponse(c, "Model Registration Completed Successfully", nil, fiber.StatusOK)
}

func modelArtifacts(c *fiber.Ctx) error {
	username := c.Query("username")
	modelname := c.Query("modelname")
	version := strings.Replace(c.Query("version"), ".", "-", -1)

	path := "./" + "artifact/ModelRegistry/" + username + "/" + modelname + "-" + version
	ignorePath := path + "/.studioignore"
	result, err := GetArtifacts(path, ignorePath)
	if err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}
	return helper.SendResponse(c, "Model Artifacts Request Sucessfully", result, fiber.StatusOK)
}

func filesPreview(c *fiber.Ctx) error {
	var result string
	var err error
	username := c.Query("username")
	modelname := c.Query("modelname")
	version := strings.Replace(c.Query("version"), ".", "-", -1)
	filename := c.Query("filename")

	path := "./" + "artifact/ModelRegistry/" + username + "/" + modelname + "-" + version
	result, err = ReadFiles(path, filename)
	if err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}
	return helper.SendResponse(c, "Lab Files and Folder Preview Request Sucessfully", result, fiber.StatusOK)
}

func labFilePreview(c *fiber.Ctx) error {
	var result string
	var err error
	username := c.Query("username")
	filename := c.Query("filename")

	path := "./artifact/jl-" + username + "-0"
	result, err = ReadFiles(path, filename)
	if err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}
	return helper.SendResponse(c, "Lab Files and Folder Preview Request Sucessfully", result, fiber.StatusOK)
}

func deleteModel(c *fiber.Ctx) error {
	username := c.Query("username")
	modelname := c.Query("modelname")
	version := strings.Replace(c.Query("version"), ".", "-", -1)

	fs := afero.NewOsFs()
	modelPath := "./artifact/ModelRegistry/" + username + "/" + modelname + "-" + version

	exists, err := afero.DirExists(fs, modelPath)
	if err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}
	if !exists {
		return helper.SendResponse(c, "Source directory does not exist", nil, 400)
	}

	result, err := deleteAllArtifacts(modelPath)
	if err != nil {
		return helper.SendResponse(c, "Invalid Request", nil, fiber.ErrBadRequest.Code)
	}
	log.Info(result)
	return helper.SendResponse(c, "Model Deletion Completed Successfully", nil, fiber.StatusOK)
}