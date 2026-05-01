package template

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/goccy/go-yaml"
)

//go:embed templates.yaml
var defaultTemplatesFS []byte

var ErrTemplateNotFound = fmt.Errorf("template not found")

// MetricTemplate represents a single PromQL template.
type MetricTemplate struct {
	Name        string   `yaml:"-"`
	Query       string   `yaml:"query"`
	Type        string   `yaml:"type"`
	ResultType  string   `yaml:"result"`
	GroupBy     []string `yaml:"group_by"`
	Description string   `yaml:"description"`
	System      bool     `yaml:"-"`
}

type templateFile struct {
	Metrics map[string]MetricTemplate `yaml:"metrics"`
}

// TemplateRegistry holds all loaded metric templates.
type TemplateRegistry struct {
	templates map[string]*MetricTemplate
}

// NewRegistry creates a registry and loads default embedded templates.
func NewRegistry() (*TemplateRegistry, error) {
	r := &TemplateRegistry{templates: make(map[string]*MetricTemplate)}
	if err := r.loadDefaults(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *TemplateRegistry) loadDefaults() error {
	var file templateFile
	if err := yaml.Unmarshal(defaultTemplatesFS, &file); err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}
	for name, tpl := range file.Metrics {
		t := tpl
		t.Name = name
		t.System = true
		r.templates[name] = &t
	}
	return nil
}

// Get returns a metric template by type.
func (r *TemplateRegistry) Get(metricType string) (*MetricTemplate, error) {
	tpl, ok := r.templates[metricType]
	if !ok {
		return nil, fmt.Errorf("metric template not found: %s: %w", metricType, ErrTemplateNotFound)
	}
	return tpl, nil
}

// Render executes the template and returns the PromQL string.
func (r *TemplateRegistry) Render(metricType string, params map[string]any) (string, error) {
	tpl, err := r.Get(metricType)
	if err != nil {
		return "", err
	}
	t, err := template.New("promql").Parse(tpl.Query)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}
