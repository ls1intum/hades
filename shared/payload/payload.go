package payload

type BuildJob struct {
	// TODO: We need a build ID or something to identify the build
	Credentials struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	} `json:"credentials" binding:"required"`
	BuildConfig struct {
		Repositories       []Repository `json:"repositories" binding:"required,dive"`
		ExecutionContainer string       `json:"executionContainer" binding:"required"`
	} `json:"buildConfig" binding:"required"`
}

type Repository struct {
	URL  string `json:"url" binding:"required,url"`
	Path string `json:"path" binding:"required,dirpath"`
}
