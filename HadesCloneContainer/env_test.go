package main

import (
	"os"
	"testing"
)

func TestReadFromEnv(t *testing.T) {
	// Set up the environment variables
	os.Setenv("DEBUG", "true")
	os.Setenv("REPOSITORY_DIR", "/tmp/repositories")
	os.Setenv("HADES_group1_URL", "https://github.com/group1/repo1.git")
	os.Setenv("HADES_group1_USERNAME", "username1")
	os.Setenv("HADES_group1_PASSWORD", "password1")
	os.Setenv("HADES_group1_BRANCH", "main")
	os.Setenv("HADES_group1_PATH", "path/to/repo1")
	os.Setenv("HADES_group1_ORDER", "1")
	os.Setenv("HADES_group2_URL", "https://github.com/group2/repo2.git")
	os.Setenv("HADES_group2_USERNAME", "username2")
	os.Setenv("HADES_group2_PASSWORD", "password2")
	os.Setenv("HADES_group2_BRANCH", "dev")
	os.Setenv("HADES_group2_ORDER", "2")

	// Clean up the environment variables after the test
	defer func() {
		os.Unsetenv("DEBUG")
		os.Unsetenv("REPOSITORY_DIR")
		os.Unsetenv("HADES_group1_URL")
		os.Unsetenv("HADES_group1_USERNAME")
		os.Unsetenv("HADES_group1_PASSWORD")
		os.Unsetenv("HADES_group1_BRANCH")
		os.Unsetenv("HADES_group1_PATH")
		os.Unsetenv("HADES_group1_ORDER")
		os.Unsetenv("HADES_group2_URL")
		os.Unsetenv("HADES_group2_USERNAME")
		os.Unsetenv("HADES_group2_PASSWORD")
		os.Unsetenv("HADES_group2_BRANCH")
		os.Unsetenv("HADES_group2_ORDER")
	}()

	// Add your assertions here
	actual := getReposFromEnv()

	expected := map[string]map[string]string{
		"group1": {
			"URL":      "https://github.com/group1/repo1.git",
			"USERNAME": "username1",
			"PASSWORD": "password1",
			"BRANCH":   "main",
			"PATH":     "path/to/repo1",
			"ORDER":    "1",
		},
		"group2": {
			"URL":      "https://github.com/group2/repo2.git",
			"USERNAME": "username2",
			"PASSWORD": "password2",
			"BRANCH":   "dev",
			"ORDER":    "2",
		},
	}

	if len(actual) != len(expected) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}

	for key, value := range actual {
		if _, ok := expected[key]; !ok {
			t.Errorf("Expected %v, got %v", expected, actual)
		}

		for k, v := range value {
			if expected[key][k] != v {
				t.Errorf("Expected %v, got %v for [%v][%v]", expected, actual, key, k)
			}
		}
	}
}
