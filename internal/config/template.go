package config

import (
	_ "embed"
	"text/template"
)

//go:embed wtm.toml.tmpl
var projectConfigTemplate string

var parsedTemplate = template.Must(template.New("wtm.toml").Parse(projectConfigTemplate))
