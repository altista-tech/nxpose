package yaml_utils

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// CheckYAMLFile is a debugging utility to check YAML parsing
func CheckYAMLFile(filename string) {
	// Read the file
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		return
	}

	// Unmarshal into a generic structure
	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing YAML: %v\n", err)
		return
	}

	// Check for oauth2 section
	oauth2, ok := config["oauth2"].(map[string]interface{})
	if !ok {
		fmt.Println("oauth2 section not found or not a map")
		return
	}

	// Check for providers
	providers, ok := oauth2["providers"].([]interface{})
	if !ok {
		fmt.Println("providers section not found or not an array")
		return
	}

	fmt.Printf("Found %d providers\n", len(providers))

	// Inspect each provider
	for i, p := range providers {
		provider, ok := p.(map[string]interface{})
		if !ok {
			fmt.Printf("Provider #%d is not a map\n", i+1)
			continue
		}

		name, _ := provider["name"].(string)
		clientID, _ := provider["client_id"].(string)
		clientSecret, _ := provider["client_secret"].(string)

		fmt.Printf("Provider #%d: %s\n", i+1, name)
		fmt.Printf("  client_id = '%s' (type: %T, len: %d)\n", clientID, provider["client_id"], len(clientID))
		fmt.Printf("  client_secret = '%s' (type: %T, len: %d)\n",
			func() string {
				if len(clientSecret) > 8 {
					return clientSecret[:4] + "..." + clientSecret[len(clientSecret)-4:]
				}
				return clientSecret
			}(),
			provider["client_secret"],
			len(clientSecret))

		// Check scopes
		scopes, ok := provider["scopes"].([]interface{})
		if !ok {
			fmt.Printf("  scopes is not an array (type: %T)\n", provider["scopes"])
		} else {
			fmt.Printf("  scopes (%d): %v\n", len(scopes), scopes)
		}
	}
}

// DirectUnmarshalOAuth2 directly unmarshals only the oauth2 section
func DirectUnmarshalOAuth2(filename string) {
	// Read the file
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		return
	}

	// Define a struct that matches the OAuth2 configuration
	type OAuth2Provider struct {
		Name         string   `yaml:"name"`
		ClientID     string   `yaml:"client_id"`
		ClientSecret string   `yaml:"client_secret"`
		Scopes       []string `yaml:"scopes"`
	}

	type OAuth2Config struct {
		Enabled        bool             `yaml:"enabled"`
		RedirectURL    string           `yaml:"redirect_url"`
		SessionKey     string           `yaml:"session_key"`
		TokenDuration  string           `yaml:"token_duration"`
		CookieDuration string           `yaml:"cookie_duration"`
		Providers      []OAuth2Provider `yaml:"providers"`
	}

	type Config struct {
		OAuth2 OAuth2Config `yaml:"oauth2"`
	}

	// Unmarshal into our specific structure
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing YAML: %v\n", err)
		return
	}

	// Print out the parsed OAuth2 configuration
	fmt.Println("OAuth2 Configuration:")
	fmt.Printf("  Enabled: %v\n", config.OAuth2.Enabled)
	fmt.Printf("  RedirectURL: %s\n", config.OAuth2.RedirectURL)
	fmt.Printf("  SessionKey: %s\n", config.OAuth2.SessionKey)
	fmt.Printf("  TokenDuration: %s\n", config.OAuth2.TokenDuration)
	fmt.Printf("  CookieDuration: %s\n", config.OAuth2.CookieDuration)
	fmt.Printf("  Providers: %d\n", len(config.OAuth2.Providers))

	// Inspect each provider
	for i, p := range config.OAuth2.Providers {
		fmt.Printf("Provider #%d: %s\n", i+1, p.Name)
		fmt.Printf("  ClientID: '%s' (len: %d)\n", p.ClientID, len(p.ClientID))
		fmt.Printf("  ClientSecret: '%s' (len: %d)\n",
			func() string {
				if len(p.ClientSecret) > 8 {
					return p.ClientSecret[:4] + "..." + p.ClientSecret[len(p.ClientSecret)-4:]
				}
				return p.ClientSecret
			}(),
			len(p.ClientSecret))
		fmt.Printf("  Scopes: %v (count: %d)\n", p.Scopes, len(p.Scopes))
	}
}
