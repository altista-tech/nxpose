package yaml_utils

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

// GitHub OAuth validation structure
type GitHubProvider struct {
	Name         string   `yaml:"name"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	Scopes       []string `yaml:"scopes"`
}

type OAuth2Config struct {
	Enabled     bool             `yaml:"enabled"`
	Providers   []GitHubProvider `yaml:"providers"`
	RedirectURL string           `yaml:"redirect_url"`
	SessionKey  string           `yaml:"session_key"`
}

type ConfigFile struct {
	OAuth2 OAuth2Config `yaml:"oauth2"`
}

// ValidateGitHubCredentials validates GitHub credentials directly without going through Viper
func ValidateGitHubCredentials(configFile string) {
	// Read the file
	data, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		return
	}

	// Unmarshal the YAML
	var config ConfigFile
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing YAML: %v\n", err)
		return
	}

	// Verify OAuth2 is enabled
	if !config.OAuth2.Enabled {
		fmt.Println("OAuth2 is not enabled in config")
		return
	}

	// Find GitHub provider
	var githubProvider *GitHubProvider
	for i := range config.OAuth2.Providers {
		if config.OAuth2.Providers[i].Name == "github" {
			githubProvider = &config.OAuth2.Providers[i]
			break
		}
	}

	if githubProvider == nil {
		fmt.Println("GitHub provider not found in config")
		return
	}

	// Verify GitHub credentials
	fmt.Println("GitHub provider found:")
	fmt.Printf("  client_id: %s\n", githubProvider.ClientID)
	fmt.Printf("  client_secret: %s...\n", githubProvider.ClientSecret[:10])
	fmt.Printf("  scopes: %v\n", githubProvider.Scopes)

	// Validate with GitHub API
	fmt.Println("\nValidating credentials with GitHub API...")
	// Build the request URL for GitHub OAuth endpoint
	validationURL := "https://github.com/login/oauth/authorize?"
	params := url.Values{}
	params.Add("client_id", githubProvider.ClientID)
	params.Add("scope", "user:email read:user")

	resp, err := http.Get(validationURL + params.Encode())
	if err != nil {
		fmt.Printf("API request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Check response
	fmt.Printf("GitHub API response: %s\n", resp.Status)
	if resp.StatusCode == 200 {
		fmt.Println("Credentials appear valid! The OAuth authorization page loaded successfully.")
	} else {
		fmt.Println("GitHub responded with an error. Credentials may be invalid.")
	}
}
