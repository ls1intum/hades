package payload

type BuildJob struct {
	Name string `json:"name" binding:"required"`

	Credentials struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	} `json:"credentials" binding:"required"`
	BuildConfig struct {
		Repositories       []Repository `json:"repositories" binding:"required,dive"`
		ExecutionContainer string       `json:"executionContainer" binding:"required"`
		BuildScript        string       `json:"buildScript" binding:"required"`
	} `json:"buildConfig" binding:"required"`
}

type Repository struct {
	URL  string `json:"url" binding:"required,url"`
	Path string `json:"path" binding:"required,dirpath"`
}
