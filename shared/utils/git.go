package utils

import (
	"fmt"
	"github.com/Mtze/HadesCI/shared/payload"
	"strings"
)

func BuildCloneCommand(repo payload.Repository) string {
	return fmt.Sprintf("git clone %s %s", repo.URL, repo.Path)
}

func BuildCloneCommands(repos ...payload.Repository) string {
	var builder strings.Builder
	for i, repo := range repos {
		if i > 0 {
			builder.WriteString(" && ")
		}
		builder.WriteString(BuildCloneCommand(repo))
	}
	return builder.String()
}
