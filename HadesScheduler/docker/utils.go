package docker

import (
	"os"
	"strings"
)

func writeBashScriptToFile(bashScriptLines ...string) (string, error) {
	bashScriptContent := strings.Join(bashScriptLines, "\n")
	tmpFile, err := os.CreateTemp("", "bash-script-*.sh")
	if err != nil {
		return "", err
	}

	_, err = tmpFile.Write([]byte(bashScriptContent))
	if err != nil {
		tmpFile.Close()
		return "", err
	}

	path := tmpFile.Name()
	tmpFile.Close()
	return path, nil
}
