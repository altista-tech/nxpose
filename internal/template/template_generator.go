package template

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

// TemplateData represents data for generating HTML pages
type TemplateData struct {
	Title       string
	Message     string
	Providers   []ProviderInfo
	ErrorInfo   string
	CallbackURL string
	StateToken  string
	Certificate string
}

// ProviderInfo contains OAuth provider information
type ProviderInfo struct {
	Name     string
	URL      string
	IconPath string
}

// GenerateLoginPage creates a login page with provider options
func GenerateLoginPage(data TemplateData) (string, error) {
	tmpl, err := loadTemplate("login.html")
	if err != nil {
		return "", fmt.Errorf("failed to load login template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute login template: %w", err)
	}

	return buf.String(), nil
}

// GenerateCallbackPage creates a callback page for OAuth flow
func GenerateCallbackPage(data TemplateData) (string, error) {
	tmpl, err := loadTemplate("callback.html")
	if err != nil {
		return "", fmt.Errorf("failed to load callback template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute callback template: %w", err)
	}

	return buf.String(), nil
}

// GenerateErrorPage creates an error page
func GenerateErrorPage(data TemplateData) (string, error) {
	tmpl, err := loadTemplate("error.html")
	if err != nil {
		return "", fmt.Errorf("failed to load error template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute error template: %w", err)
	}

	return buf.String(), nil
}

// GenerateSuccessPage creates a success page
func GenerateSuccessPage(data TemplateData) (string, error) {
	tmpl, err := loadTemplate("success.html")
	if err != nil {
		return "", fmt.Errorf("failed to load success template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute success template: %w", err)
	}

	return buf.String(), nil
}

// loadTemplate loads a template from the templates directory
func loadTemplate(name string) (*template.Template, error) {
	// Look for the template in the following locations:
	// 1. ./templates/ (relative to current directory)
	// 2. ../templates/ (one level up)
	// 3. ../../templates/ (two levels up)
	possiblePaths := []string{
		filepath.Join("templates", name),
		filepath.Join("..", "templates", name),
		filepath.Join("..", "..", "templates", name),
	}

	var templateContent []byte
	var err error

	for _, path := range possiblePaths {
		templateContent, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read template file %s: %w", name, err)
	}

	// Parse the template with several useful functions
	funcMap := template.FuncMap{
		"title": func(s string) string {
			if len(s) == 0 {
				return s
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
		"slice": func(s string, i, j int) string {
			if i >= len(s) {
				return ""
			}
			if j >= len(s) {
				j = len(s)
			}
			return s[i:j]
		},
	}

	return template.New(name).Funcs(funcMap).Parse(string(templateContent))
}
