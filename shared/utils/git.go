package utils

import (
	"fmt"
	"github.com/Mtze/HadesCI/shared/payload"
	"net/url"
	"strings"
)

func BuildCloneCommand(username, password string, repo payload.Repository) string {
	username = url.PathEscape(username)
	password = url.PathEscape(password)
	cloneURL := strings.Replace(repo.URL, "https://", fmt.Sprintf("https://%s:%s@", username, password), 1)
	return fmt.Sprintf("git clone %s %s", cloneURL, repo.Path)
}

func BuildCloneCommands(credentials payload.Credentials, repos ...payload.Repository) string {
	var builder strings.Builder
	for i, repo := range repos {
		if i > 0 {
			builder.WriteString(" && ")
		}
		builder.WriteString(BuildCloneCommand(credentials.Username, credentials.Password, repo))
	}
	return builder.String()
}
