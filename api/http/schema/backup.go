package schema

type RestoreStagedResponse struct {
	StagedPath string `json:"staged_path"`
	Message    string `json:"message"`
}
