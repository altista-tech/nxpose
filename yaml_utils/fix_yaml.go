package yaml_utils

import (
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v3"
)

// FixYAMLFile attempts to fix a YAML configuration file by reading it
// and rewriting it with proper structure
func FixYAMLFile(inputFile, outputFile string) {
	// Read the file
	data, err := ioutil.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		return
	}

	// Try to unmarshal to check validity
	var anyMap map[string]interface{}
	err = yaml.Unmarshal(data, &anyMap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing YAML: %v\n", err)
		return
	}

	// Find oauth2 section
	oauth2Section, hasOAuth2 := anyMap["oauth2"].(map[string]interface{})
	if !hasOAuth2 {
		fmt.Println("No oauth2 section found in input file")
		return
	}

	// Rebuild oauth2 section with proper structure
	oauth2 := map[string]interface{}{
		"enabled": oauth2Section["enabled"],
	}

	// Handle redirect_url
	if redirectURL, ok := oauth2Section["redirect_url"].(string); ok {
		oauth2["redirect_url"] = redirectURL
	}

	// Handle session_key
	if sessionKey, ok := oauth2Section["session_key"].(string); ok {
		oauth2["session_key"] = sessionKey
	}

	// Handle token_duration
	if tokenDuration, ok := oauth2Section["token_duration"].(string); ok {
		oauth2["token_duration"] = tokenDuration
	}

	// Handle cookie_duration
	if cookieDuration, ok := oauth2Section["cookie_duration"].(string); ok {
		oauth2["cookie_duration"] = cookieDuration
	}

	// Rebuild providers with explicit field names
	if providers, ok := oauth2Section["providers"].([]interface{}); ok {
		var newProviders []map[string]interface{}

		for _, p := range providers {
			if providerMap, ok := p.(map[string]interface{}); ok {
				newProvider := map[string]interface{}{}

				// Copy name
				if name, ok := providerMap["name"].(string); ok {
					newProvider["name"] = name
				}

				// Copy client_id
				if clientID, ok := providerMap["client_id"].(string); ok {
					newProvider["client_id"] = clientID
				}

				// Copy client_secret
				if clientSecret, ok := providerMap["client_secret"].(string); ok {
					newProvider["client_secret"] = clientSecret
				}

				// Copy scopes
				if scopes, ok := providerMap["scopes"].([]interface{}); ok {
					var newScopes []string
					for _, s := range scopes {
						if scope, ok := s.(string); ok {
							newScopes = append(newScopes, scope)
						}
					}
					newProvider["scopes"] = newScopes
				}

				newProviders = append(newProviders, newProvider)
			}
		}

		oauth2["providers"] = newProviders
	}

	// Create the new structure
	newConfig := map[string]interface{}{
		"oauth2": oauth2,
	}

	// Add other sections from original file
	for key, value := range anyMap {
		if key != "oauth2" {
			newConfig[key] = value
		}
	}

	// Marshal back to YAML
	newData, err := yaml.Marshal(newConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating YAML: %v\n", err)
		return
	}

	// Write to output file
	err = ioutil.WriteFile(outputFile, newData, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
		return
	}

	fmt.Printf("Successfully fixed YAML file. Output written to: %s\n", outputFile)
}
