package artifacts

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

var logs = logrus.New()

func DirSize(fs afero.Fs, path string) (int64, error) {
	var size int64
	err := afero.Walk(fs, path, func(_ string, info os.FileInfo, _ error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
func getIgnoreItem(path string) map[string]bool {
	ignoreItems := make(map[string]bool)
	file, err := os.Open(path)
	if err != nil {
		logs.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ignorePath := scanner.Text()
		ignoreItems[ignorePath] = true

		// If the ignored item is a directory, add all its contents to the ignoreItems map
		if info, err := os.Stat(ignorePath); err == nil && info.IsDir() {
			err := filepath.Walk(ignorePath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				ignoreItems[path] = true
				return nil
			})
			if err != nil {
				logs.Fatal(err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		logs.Fatal(err)
	}

	return ignoreItems
}

func GetArtifacts(path string, ignore_path string) (map[string]interface{}, error) {
	fs := afero.NewOsFs()
	dirDict := make(map[string]interface{})
	dirDict["fileList"] = []string{}
	ignoreItems := make(map[string]bool)
	exists, err := afero.Exists(fs, ignore_path)
	if err != nil {
		return nil, err
	}
	if exists {
		ignoreItems = getIgnoreItem(ignore_path)
	}

	files, err := afero.ReadDir(fs, path)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			// Ignore directories

			if _, ok := ignoreItems[file.Name()]; ok {
				continue
			}
			if file.Name() == ".ipynb_checkpoints" || file.Name() == ".git" || file.Name() == "venv" || file.Name() == ".aim" || file.Name() == "__pycache__" {
				continue
			}

			subDirPath := path + "/" + file.Name()
			// Ignore directories larger than 10GB
			dirSize, _ := DirSize(fs, subDirPath)
			if dirSize > 100e9 {
				continue
			}
			if _, ok := ignoreItems[subDirPath]; ok {
				continue
			}
			subDirDict, err := GetArtifacts(subDirPath, ignore_path)
			if err != nil {
				return nil, err
			}

			dirDict[file.Name()] = subDirDict
		} else {
			// Ignore files with certain extensions

			if ignoreItems[file.Name()] {
				continue
			}
			ext := filepath.Ext(file.Name())
			if ignoreItems["*"+ext] {
				continue
			}

			fileInfo, _ := fs.Stat(path + "/" + file.Name())
			fileSize := fileInfo.Size()
			lastUpdated := fileInfo.ModTime().Format("2006-01-02 15:04:05")

			var sizeStr string
			if fileSize < 1e3 { // less than 1KB
				sizeStr = fmt.Sprintf("%d bytes", fileSize)
			} else if fileSize < 1e6 { // less than 1MB
				sizeStr = fmt.Sprintf("%.3f KB", float64(fileSize)/1e3)
			} else if fileSize < 1e9 { // less than 1GB
				sizeStr = fmt.Sprintf("%.3f MB", float64(fileSize)/1e6)
			} else { // 1GB or more
				sizeStr = fmt.Sprintf("%.3f GB", float64(fileSize)/1e9)
			}

			dirDict["fileList"] = append(dirDict["fileList"].([]string), fmt.Sprintf("%s (%s, %s)", file.Name(), sizeStr, lastUpdated)) // changed from "files" to "fileList"
		}
	}
	return dirDict, nil
}

func CopyAllArtifacts(src string, dst string) (string, error) {
	fs := afero.NewOsFs()
	ignoreItems := getIgnoreItem(src + "/.studioignore")
	err := afero.Walk(fs, src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if _, ok := ignoreItems[info.Name()]; ok {
				return filepath.SkipDir
			}
			relPath, _ := filepath.Rel(src, path)
			destPath := filepath.Join(dst, relPath)

			exists, err := afero.DirExists(fs, destPath)
			if err != nil {
				return err
			}
			if !exists {
				if err := fs.MkdirAll(destPath, os.ModePerm); err != nil {
					return err
				}
			}
			return nil
		}
		relPath, _ := filepath.Rel(src, path)
		destPath := filepath.Join(dst, relPath)

		if _, ok := ignoreItems[filepath.Base(path)]; ok {
			return nil
		}

		srcFile, err := fs.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		destFile, err := fs.Create(destPath)
		if err != nil {
			return err
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		return err
	})
	if err != nil {
		return "", err
	}
	return "Process completed successfully", nil
}

func CopyModelArtifactsFile(username string, deploymentName string, modelname string, version string, modelFileNames []string) (bool, error) {
	fs := afero.NewOsFs()
	success := false
	src := "./" + "artifact/ModelRegistry/" + username + "/" + modelname + "-" + version
	dst := "./" + "artifact/pvc-" + deploymentName + "/"
	exists, err := afero.DirExists(fs, src)
	if err != nil {
		return success, err
	}
	if !exists {
		return success, fmt.Errorf("source directory does not exist: %s", src)
	}
	err = afero.Walk(fs, src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		for _, modelFileName := range modelFileNames {
			if filepath.Base(path) == modelFileName {
				// relPath, _ := filepath.Rel(src, path)
				// destPath := filepath.Join(dst, relPath)

				newFileName := modelname + version + "/" + modelFileName
				// if filepath.Ext(modelFileName) == ".py" {
				// 	newFileName = modelname + version + "/" + "app.py"
				// }
				if filepath.Ext(modelFileName) == ".sh" {
					newFileName = modelname + version + "/" + "dependency.sh"
				}
				destPath := filepath.Join(dst, newFileName)

				destDirs := filepath.Dir(destPath)
				if _, err := fs.Stat(destDirs); os.IsNotExist(err) {
					err = fs.MkdirAll(destDirs, 0755)
					if err != nil {
						return err
					}
				}

				srcFile, err := fs.Open(path)
				if err != nil {
					return err
				}
				defer srcFile.Close()

				destFile, err := fs.Create(destPath)
				if err != nil {
					return err
				}
				defer destFile.Close()

				_, err = io.Copy(destFile, srcFile)
				if err != nil {
					return err
				}
				success = true
			}
		}
		return nil
	})
	if err != nil {
		return success, err
	}
	return success, nil
}

func CopyModelArtifactsFiles(username string, deploymentName string, modelname string, version string, models []string) (bool, error) {
	fs := afero.NewOsFs()
	success := false
	src := "./" + "artifact/ModelRegistry/" + username + "/" + modelname + "-" + version
	dst := "./" + "artifact/pvc-" + deploymentName + "/"
	exists, err := afero.DirExists(fs, src)
	if err != nil {
		return success, err
	}
	if !exists {
		return success, fmt.Errorf("source directory does not exist: %s", src)
	}

	modelsMap := make(map[string]bool)
	for _, model := range models {
		modelsMap[model] = true
	}

	err = afero.Walk(fs, src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(src, path)
		for modelPath, _ := range modelsMap {
			if strings.HasPrefix(relPath, modelPath) {
				newFileName := modelname + version + "/" + relPath
				if filepath.Ext(relPath) == ".sh" {
					newFileName = modelname + version + "/" + "dependency.sh"
				}
				destPath := filepath.Join(dst, newFileName)

				if err := fs.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
					return err
				}

				if info.IsDir() {
					return nil
				}

				srcFile, err := fs.Open(path)
				if err != nil {
					return err
				}
				defer srcFile.Close()

				destFile, err := fs.Create(destPath)
				if err != nil {
					return err
				}
				defer destFile.Close()

				_, err = io.Copy(destFile, srcFile)
				if err != nil {
					return err
				}
				success = true
				break
			}
		}
		return nil
	})
	if err != nil {
		return success, err
	}
	return success, nil
}

func CopyModelFile(username string, modelname string, version string, modelFileNames []string) (bool, error) {
	fs := afero.NewOsFs()
	success := false
	src := "./" + "artifact/ModelRegistry/" + username + "/" + modelname + "-" + version
	dst := "./" + "artifact/pvc-" + username + "/"

	exists, err := afero.DirExists(fs, src)
	if err != nil {
		return success, err
	}
	if !exists {
		return success, fmt.Errorf("source directory does not exist: %s", src)
	}

	// First, copy all files from source to destination
	err = afero.Walk(fs, src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		for _, modelFileName := range modelFileNames {
			if filepath.Base(path) == modelFileName {
				relPath, _ := filepath.Rel(src, path)
				destPath := filepath.Join(dst, relPath)

				destDirectory := modelname + version + "/" + modelFileName
				destDir := filepath.Dir(destDirectory)
				if _, err := fs.Stat(destDir); os.IsNotExist(err) {
					err = fs.MkdirAll(destDir, 0755)
					if err != nil {
						return err
					}
				}

				srcFile, err := fs.Open(path)
				if err != nil {
					return err
				}
				defer srcFile.Close()

				destFile, err := fs.Create(destPath)
				if err != nil {
					return err
				}
				defer destFile.Close()

				_, err = io.Copy(destFile, srcFile)
				if err != nil {
					return err
				}
				success = true
			}
		}
		return nil
	})
	if err != nil {
		return success, err
	}

	for _, modelFileName := range modelFileNames {
		if filepath.Ext(modelFileName) == ".py" {
			oldPath := filepath.Join(dst, modelFileName)
			newPath := filepath.Join(dst, "app.py")
			err := fs.Rename(oldPath, newPath)
			if err != nil {
				return false, err
			}
		}
	}

	return success, nil
}

func CloneSelectedArtifacts(src string, dst string, artifactsFilesNames []string) (string, error) {
	fs := afero.NewOsFs()
	logs.Info("Starting artifact cloning", artifactsFilesNames)

	artifactsFilesMap := make(map[string]bool)
	for _, name := range artifactsFilesNames {
		cleanName := strings.TrimPrefix(name, "/")
		artifactsFilesMap[cleanName] = true
	}

	err := afero.Walk(fs, src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logs.Error("Error walking the file tree: ", err)
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			logs.Error("Error calculating relative path: ", err)
			return err
		}

		// Check if the relative path starts with any of the specified artifact paths
		for artifactPath := range artifactsFilesMap {
			if relPath == artifactPath || strings.HasPrefix(relPath, artifactPath+string(filepath.Separator)) || strings.HasPrefix(relPath, artifactPath) && !info.IsDir() {
				destPath := filepath.Join(dst, relPath)

				if info.IsDir() {
					exists, err := afero.DirExists(fs, destPath)
					if err != nil {
						logs.Error("Error checking destination directory: ", err)
						return err
					}
					if !exists {
						if err := fs.MkdirAll(destPath, os.ModePerm); err != nil {
							logs.Error("Error creating destination directory: ", err)
							return err
						}
					}
					return nil
				}

				// Handle file copying
				destDir := filepath.Dir(destPath)
				if err := fs.MkdirAll(destDir, os.ModePerm); err != nil {
					logs.Error("Error creating destination directory structure: ", err)
					return err
				}

				srcFile, err := fs.Open(path)
				if err != nil {
					logs.Error("Error opening source file: ", err)
					return err
				}
				defer srcFile.Close()

				destFile, err := fs.Create(destPath)
				if err != nil {
					logs.Error("Error creating destination file: ", err)
					return err
				}
				defer destFile.Close()

				_, err = io.Copy(destFile, srcFile)
				if err != nil {
					logs.Error("Error copying file: ", err)
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		logs.Error("Error after walking the file tree: ", err)
		return "", err
	}
	return "Files copied successfully", nil
}

func ReadFiles(path string, filename string) (string, error) {

	fullPath := filepath.Join(path, filename)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

func deleteAllArtifacts(modelPath string) (string, error) {
	err := os.RemoveAll(modelPath)
	if err != nil {
		logs.Error(err.Error(), "Error while Deleteing model", modelPath)
		return "", err
	}
	return "Artifacts deleted successfully", nil
}
