package docker

import "io/ioutil"

func writeBashScriptToFile(bashScriptContent string) (string, error) {
	tmpFile, err := ioutil.TempFile("", "bash-script-*.sh")
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
