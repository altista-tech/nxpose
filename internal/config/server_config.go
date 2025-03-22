package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"nxpose/internal/crypto"
)

// LetsEncryptConfig holds configuration for Let's Encrypt integration
type LetsEncryptConfig struct {
	// Existing fields
	Enabled     bool
	Email       string
	Environment crypto.Environment
	StorageDir  string

	// New DNS fields
	DNSProvider    string
	DNSCredentials map[string]string
}

// ServerConfig holds all configuration for the server
type ServerConfig struct {
	// Server settings
	BindAddress string
	Port        int
	BaseDomain  string

	// TLS settings
	TLSCert string
	TLSKey  string

	// Common settings
	Verbose bool

	// OAuth2 settings
	OAuth2 OAuth2Config

	// MongoDB settings
	MongoDB MongoDBConfig

	// Let's Encrypt settings
	LetsEncrypt LetsEncryptConfig
}

// OAuth2Config holds the configuration for OAuth2 providers
type OAuth2Config struct {
	Enabled      bool
	Providers    []OAuth2ProviderConfig
	RedirectURL  string
	SessionKey   string
	SessionStore string
}

// OAuth2ProviderConfig holds configuration for a specific OAuth2 provider
type OAuth2ProviderConfig struct {
	Name         string
	ClientID     string
	ClientSecret string
	Credentials  []map[string]string
	Scopes       []string
}

// MongoDBConfig holds the configuration for MongoDB
type MongoDBConfig struct {
	Enabled  bool
	URI      string
	Database string
	Timeout  time.Duration
}

// DefaultServerConfig returns a config with default values
func DefaultServerConfig() *ServerConfig {
	// Get home directory for default storage
	homeDir, err := os.UserHomeDir()
	storageDir := ""
	if err == nil {
		storageDir = filepath.Join(homeDir, ".nxpose", "certificates")
	}

	return &ServerConfig{
		BindAddress: "0.0.0.0",
		Port:        8443,
		BaseDomain:  "localhost",
		Verbose:     false,
		TLSCert:     "",
		TLSKey:      "",
		OAuth2: OAuth2Config{
			Enabled:      false,
			RedirectURL:  "https://localhost:8443/auth/callback",
			SessionKey:   "",
			SessionStore: "memory",
			Providers:    []OAuth2ProviderConfig{},
		},
		MongoDB: MongoDBConfig{
			Enabled:  false,
			URI:      "mongodb://localhost:27017",
			Database: "nxpose",
			Timeout:  10 * time.Second,
		},
		LetsEncrypt: LetsEncryptConfig{
			Enabled:        false,
			Email:          "",
			Environment:    crypto.ProductionEnv,
			StorageDir:     storageDir,
			DNSProvider:    "",
			DNSCredentials: make(map[string]string),
		},
	}
}

// LoadServerConfig loads configuration from config files and environment variables
func LoadServerConfig(configFile string) (*ServerConfig, error) {
	config := DefaultServerConfig()

	// Setup viper to read environment variables
	viper.SetEnvPrefix("NXPOSE")
	viper.AutomaticEnv()

	// Map environment variables to config keys for server-specific settings
	viper.BindEnv("server.bind_address", "NXPOSE_SERVER_BIND_ADDRESS")
	viper.BindEnv("server.port", "NXPOSE_SERVER_PORT")
	viper.BindEnv("server.domain", "NXPOSE_SERVER_DOMAIN")
	viper.BindEnv("tls.cert", "NXPOSE_TLS_CERT")
	viper.BindEnv("tls.key", "NXPOSE_TLS_KEY")
	viper.BindEnv("verbose", "NXPOSE_VERBOSE")

	// MongoDB settings
	viper.BindEnv("mongodb.enabled", "NXPOSE_MONGODB_ENABLED")
	viper.BindEnv("mongodb.uri", "NXPOSE_MONGODB_URI")
	viper.BindEnv("mongodb.database", "NXPOSE_MONGODB_DATABASE")
	viper.BindEnv("mongodb.timeout", "NXPOSE_MONGODB_TIMEOUT")

	// Let's Encrypt settings
	viper.BindEnv("letsencrypt.enabled", "NXPOSE_LETSENCRYPT_ENABLED")
	viper.BindEnv("letsencrypt.email", "NXPOSE_LETSENCRYPT_EMAIL")
	viper.BindEnv("letsencrypt.environment", "NXPOSE_LETSENCRYPT_ENVIRONMENT")
	viper.BindEnv("letsencrypt.storage_dir", "NXPOSE_LETSENCRYPT_STORAGE_DIR")
	viper.BindEnv("letsencrypt.dns.provider", "NXPOSE_LETSENCRYPT_DNS_PROVIDER")

	// OAuth2 settings
	viper.BindEnv("oauth2.enabled", "NXPOSE_OAUTH2_ENABLED")
	viper.BindEnv("oauth2.redirect_url", "NXPOSE_OAUTH2_REDIRECT_URL")
	viper.BindEnv("oauth2.session_key", "NXPOSE_OAUTH2_SESSION_KEY")
	viper.BindEnv("oauth2.session_store", "NXPOSE_OAUTH2_SESSION_STORE")

	// If configFile is provided, use it directly
	if configFile != "" {
		viper.SetConfigFile(configFile)

		// Try to read config from file
		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("error reading config file: %w", err)
			}
			// If file not found, we'll still use env vars
			fmt.Println("Config file not found, using environment variables only")
		} else {
			// Print raw OAuth2 provider configuration for debugging
			if viper.GetBool("verbose") {
				fmt.Printf("DEBUG: Config file loaded: %s\n", viper.ConfigFileUsed())

				// Try to check for raw OAuth2 providers configuration
				providersKey := "oauth2.providers"
				if !viper.IsSet(providersKey) {
					fmt.Printf("DEBUG: Key '%s' not found in config\n", providersKey)
				} else {
					rawValue := viper.Get(providersKey)
					fmt.Printf("DEBUG: Raw value for '%s': %v (type: %T)\n", providersKey, rawValue, rawValue)

					// Try to print the raw YAML representation
					if yamlBytes, err := yaml.Marshal(rawValue); err == nil {
						fmt.Printf("DEBUG: YAML representation of providers:\n---\n%s---\n", string(yamlBytes))
					}
				}
			}
		}
	} else {
		// No config file specified, try to find one in standard locations
		// Look for config in the following places:
		// 1. Current directory
		// 2. $HOME/.nxpose
		// 3. /etc/nxpose
		viper.AddConfigPath(".")

		homeDir, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(filepath.Join(homeDir, ".nxpose"))
		}

		viper.AddConfigPath("/etc/nxpose")
		viper.SetConfigName("server-config")
		viper.SetConfigType("yaml")

		// Try to read config from file (doesn't error if file doesn't exist)
		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("error reading config file: %w", err)
			}
			fmt.Println("No config file found, using environment variables only")
		}
	}

	// Map config values from Viper to our Config struct
	// Server settings
	if viper.IsSet("server.bind_address") {
		config.BindAddress = viper.GetString("server.bind_address")
	}
	if viper.IsSet("server.port") {
		config.Port = viper.GetInt("server.port")
	}
	if viper.IsSet("server.domain") {
		config.BaseDomain = viper.GetString("server.domain")
	}

	// TLS settings
	if viper.IsSet("tls.cert") {
		config.TLSCert = viper.GetString("tls.cert")
	}
	if viper.IsSet("tls.key") {
		config.TLSKey = viper.GetString("tls.key")
	}

	// Verbose flag
	if viper.IsSet("verbose") {
		config.Verbose = viper.GetBool("verbose")
	}

	// OAuth2 settings
	if viper.IsSet("oauth2.enabled") {
		config.OAuth2.Enabled = viper.GetBool("oauth2.enabled")
	}
	if viper.IsSet("oauth2.redirect_url") {
		config.OAuth2.RedirectURL = viper.GetString("oauth2.redirect_url")
	}
	if viper.IsSet("oauth2.session_key") {
		config.OAuth2.SessionKey = viper.GetString("oauth2.session_key")
	}
	if viper.IsSet("oauth2.session_store") {
		config.OAuth2.SessionStore = viper.GetString("oauth2.session_store")
	}

	// OAuth2 providers
	if viper.IsSet("oauth2.providers") {
		var providers []OAuth2ProviderConfig
		err := viper.UnmarshalKey("oauth2.providers", &providers)
		if err == nil {
			// Process the new credentials format
			for i := range providers {
				// Check if we need to extract from credentials array
				if len(providers[i].Credentials) > 0 {
					// Extract client_id and client_secret from credentials
					for _, cred := range providers[i].Credentials {
						if id, ok := cred["client_id"]; ok && id != "" {
							providers[i].ClientID = id
						}
						if secret, ok := cred["client_secret"]; ok && secret != "" {
							providers[i].ClientSecret = secret
						}
					}
				}
			}

			// Add debug logging to show what was loaded
			if viper.GetBool("verbose") {
				fmt.Printf("DEBUG: OAuth2 providers loaded from config: %d\n", len(providers))
				for i, p := range providers {
					fmt.Printf("DEBUG:   Provider #%d: %s\n", i+1, p.Name)
					fmt.Printf("DEBUG:     - ClientID: '%s' (length: %d)\n", p.ClientID, len(p.ClientID))
					fmt.Printf("DEBUG:     - ClientSecret: '%s' (length: %d)\n",
						func() string {
							if len(p.ClientSecret) > 8 {
								return p.ClientSecret[:4] + "..." + p.ClientSecret[len(p.ClientSecret)-4:]
							}
							return strings.Repeat("*", len(p.ClientSecret))
						}(),
						len(p.ClientSecret))
					fmt.Printf("DEBUG:     - Scopes: %v\n", p.Scopes)
				}
			}
			config.OAuth2.Providers = providers
		} else {
			fmt.Printf("ERROR: Failed to unmarshal OAuth2 providers: %v\n", err)
		}
	} else {
		fmt.Printf("DEBUG: oauth2.providers not set in configuration\n")
	}

	// MongoDB settings
	if viper.IsSet("mongodb.enabled") {
		config.MongoDB.Enabled = viper.GetBool("mongodb.enabled")
	}
	if viper.IsSet("mongodb.uri") {
		config.MongoDB.URI = viper.GetString("mongodb.uri")
	}
	if viper.IsSet("mongodb.database") {
		config.MongoDB.Database = viper.GetString("mongodb.database")
	}
	if viper.IsSet("mongodb.timeout") {
		config.MongoDB.Timeout = viper.GetDuration("mongodb.timeout")
	}

	// Let's Encrypt settings
	if viper.IsSet("letsencrypt.enabled") {
		config.LetsEncrypt.Enabled = viper.GetBool("letsencrypt.enabled")
	}
	if viper.IsSet("letsencrypt.email") {
		config.LetsEncrypt.Email = viper.GetString("letsencrypt.email")
	}
	if viper.IsSet("letsencrypt.environment") {
		env := viper.GetString("letsencrypt.environment")
		if env == "staging" {
			config.LetsEncrypt.Environment = crypto.StagingEnv
		} else {
			config.LetsEncrypt.Environment = crypto.ProductionEnv
		}
	}
	if viper.IsSet("letsencrypt.storage_dir") {
		config.LetsEncrypt.StorageDir = viper.GetString("letsencrypt.storage_dir")
	}

	// Load DNS provider
	if viper.IsSet("letsencrypt.dns.provider") {
		config.LetsEncrypt.DNSProvider = viper.GetString("letsencrypt.dns.provider")
	}

	// Load DNS credentials
	if viper.IsSet("letsencrypt.dns.credentials") {
		credentials := viper.GetStringMapString("letsencrypt.dns.credentials")
		config.LetsEncrypt.DNSCredentials = credentials
	}

	return config, nil
}

// SaveServerConfig saves the configuration to a file
func SaveServerConfig(config *ServerConfig, filePath string) error {
	// Set server values in viper
	viper.Set("server.bind_address", config.BindAddress)
	viper.Set("server.port", config.Port)
	viper.Set("server.domain", config.BaseDomain)

	// Set TLS values in viper
	viper.Set("tls.cert", config.TLSCert)
	viper.Set("tls.key", config.TLSKey)

	// Set verbose flag
	viper.Set("verbose", config.Verbose)

	// Set OAuth2 settings
	viper.Set("oauth2.enabled", config.OAuth2.Enabled)
	viper.Set("oauth2.redirect_url", config.OAuth2.RedirectURL)
	viper.Set("oauth2.session_key", config.OAuth2.SessionKey)
	viper.Set("oauth2.session_store", config.OAuth2.SessionStore)
	viper.Set("oauth2.providers", config.OAuth2.Providers)

	// Set MongoDB settings
	viper.Set("mongodb.enabled", config.MongoDB.Enabled)
	viper.Set("mongodb.uri", config.MongoDB.URI)
	viper.Set("mongodb.database", config.MongoDB.Database)
	viper.Set("mongodb.timeout", config.MongoDB.Timeout)

	// Let's Encrypt settings
	viper.Set("letsencrypt.enabled", config.LetsEncrypt.Enabled)
	viper.Set("letsencrypt.email", config.LetsEncrypt.Email)
	if config.LetsEncrypt.Environment == crypto.StagingEnv {
		viper.Set("letsencrypt.environment", "staging")
	} else {
		viper.Set("letsencrypt.environment", "production")
	}
	viper.Set("letsencrypt.storage_dir", config.LetsEncrypt.StorageDir)

	// Load DNS provider
	viper.Set("letsencrypt.dns.provider", config.LetsEncrypt.DNSProvider)

	// Load DNS credentials
	viper.Set("letsencrypt.dns.credentials", config.LetsEncrypt.DNSCredentials)

	// If no file path provided, use default
	if filePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home directory: %w", err)
		}

		configDir := filepath.Join(homeDir, ".nxpose")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("could not create config directory: %w", err)
		}

		filePath = filepath.Join(configDir, "server-config.yaml")
	}

	// Write config to file
	if err := viper.WriteConfigAs(filePath); err != nil {
		return fmt.Errorf("could not save config: %w", err)
	}

	return nil
}
