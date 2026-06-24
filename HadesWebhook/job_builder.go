package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/template"
)

// defaultTemplate is used when JOB_TEMPLATE_PATH is not set.
// It produces a minimal echo job useful for verifying the webhook pipeline.
const defaultTemplate = `{
  "name": {{ printf "%s: %s@%s" .EventType .RepoFullName .ShortSHA | json }},
  "priority": 3,
  "steps": [
    {
      "id": 1,
      "name": "echo",
      "image": "alpine:latest",
      "script": {{ printf "echo 'event=%s repo=%s branch=%s sha=%s sender=%s'" .EventType .RepoFullName .Branch .ShortSHA .SenderLogin | json }}
    }
  ]
}`

type jobBuilder struct {
	tmpl *template.Template
}

func newJobBuilder(templatePath string) (*jobBuilder, error) {
	funcMap := template.FuncMap{
		// env returns the value of an environment variable.
		// Use in templates: {{ env "MY_VAR" | json }}
		"env": os.Getenv,
		// json marshals any value to its JSON representation.
		// String values are returned with surrounding quotes and proper escaping.
		// Use for all string fields: "field": {{ .Value | json }}
		"json": func(v any) (string, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
	}

	src := defaultTemplate
	if templatePath != "" {
		content, err := os.ReadFile(templatePath)
		if err != nil {
			return nil, fmt.Errorf("read template file %q: %w", templatePath, err)
		}
		src = string(content)
	}

	tmpl, err := template.New("job").Funcs(funcMap).Parse(src)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	return &jobBuilder{tmpl: tmpl}, nil
}

// build renders the job template with the given event context and returns the
// resulting JSON payload ready to POST to HadesAPI /build.
func (b *jobBuilder) build(ctx EventContext) ([]byte, error) {
	var buf bytes.Buffer
	if err := b.tmpl.Execute(&buf, ctx); err != nil {
		return nil, fmt.Errorf("render template: %w", err)
	}
	if !json.Valid(buf.Bytes()) {
		return nil, fmt.Errorf("template produced invalid JSON; check that string fields use the 'json' template function")
	}
	return buf.Bytes(), nil
}
