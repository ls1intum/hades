package main

import "testing"

func TestFromMap(t *testing.T) {
	m := map[string]string{
		"URL":      "https://github.com/example/repo",
		"USERNAME": "john",
		"PASSWORD": "secret",
		"BRANCH":   "main",
		"PATH":     "/path/to/repo",
	}

	expected := Repository{
		URL:      "https://github.com/example/repo",
		Username: "john",
		Password: "secret",
		Branch:   "main",
		Path:     "/path/to/repo",
	}

	result := FromMap(m)

	if result != expected {
		t.Errorf("FromMap() = %v, want %v", result, expected)
	}
}

func TestFromEmptyMap(t *testing.T) {
	m := map[string]string{}

	expected := Repository{}

	result := FromMap(m)

	if result != expected {
		t.Errorf("FromMap() = %v, want %v", result, expected)
	}
}
