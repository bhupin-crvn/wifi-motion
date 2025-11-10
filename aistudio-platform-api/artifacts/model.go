package artifacts


type Register struct {
	Username  string `json:"username"`
	Modelname string `json:"modelname"`
	Version   string `json:"version"`
}

type Artifact struct {
	Username string `json:"username"`
	Selectedartifacts []string `json:"selectedartifacts"`
}

type Notebookfile struct {
	Cells []Cell `json:"cells"`
}

type Cell struct {
	CellType string   `json:"cell_type"`
	Source   []string `json:"source"`
}