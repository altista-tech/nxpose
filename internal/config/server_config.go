package config

import (
	"fmt"
	"os"
	"path/filepath"
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

// RedisConfig holds the configuration for Redis
type RedisConfig struct {
	Enabled   bool
	Host      string
	Port      int
	Password  string
	DB        int
	KeyPrefix string
	Timeout   time.Duration
}

// TunnelLimitsConfig holds the configuration for tunnel limits
type TunnelLimitsConfig struct {
	MaxPerUser    int
	MaxConnection string // Format: "10s", "5m", "2h", etc.
}

// AdminConfig holds configuration for the admin panel
type AdminConfig struct {
	Enabled    bool
	PathPrefix string
	AuthMethod string // "basic" or "none"
	Username   string // For basic auth
	Password   string // For basic auth
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

	// Redis settings
	Redis RedisConfig

	// Tunnel limits
	TunnelLimits TunnelLimitsConfig

	// Let's Encrypt settings
	LetsEncrypt LetsEncryptConfig

	// Admin panel settings
	Admin AdminConfig
}

// OAuth2Config holds the configuration for OAuth2 providers
type OAuth2Config struct {
	Enabled      bool
	Providers    []OAuth2ProviderConfig
	RedirectURL  string
	SessionKey   string
	SessionStore string // "memory", "mongo", or "redis"
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
		Redis: RedisConfig{
			Enabled:   false,
			Host:      "localhost",
			Port:      6379,
			Password:  "",
			DB:        0,
			KeyPrefix: "nxpose:",
			Timeout:   10 * time.Second,
		},
		TunnelLimits: TunnelLimitsConfig{
			MaxPerUser:    5,
			MaxConnection: "",
		},
		LetsEncrypt: LetsEncryptConfig{
			Enabled:        false,
			Email:          "",
			Environment:    crypto.ProductionEnv,
			StorageDir:     storageDir,
			DNSProvider:    "",
			DNSCredentials: make(map[string]string),
		},
		Admin: AdminConfig{
			Enabled:    false,
			PathPrefix: "/admin",
			AuthMethod: "basic",
			Username:   "admin",
			Password:   "",
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

	// Redis settings
	viper.BindEnv("redis.enabled", "NXPOSE_REDIS_ENABLED")
	viper.BindEnv("redis.host", "NXPOSE_REDIS_HOST")
	viper.BindEnv("redis.port", "NXPOSE_REDIS_PORT")
	viper.BindEnv("redis.password", "NXPOSE_REDIS_PASSWORD")
	viper.BindEnv("redis.db", "NXPOSE_REDIS_DB")
	viper.BindEnv("redis.key_prefix", "NXPOSE_REDIS_KEY_PREFIX")
	viper.BindEnv("redis.timeout", "NXPOSE_REDIS_TIMEOUT")

	// Tunnel limits
	viper.BindEnv("tunnels.max_per_user", "NXPOSE_TUNNELS_MAX_PER_USER")
	viper.BindEnv("tunnels.max_connection", "NXPOSE_TUNNELS_MAX_CONNECTION")

	// Let's Encrypt settings
	viper.BindEnv("letsencrypt.enabled", "NXPOSE_LETSENCRYPT_ENABLED")
	viper.BindEnv("letsencrypt.email", "NXPOSE_LETSENCRYPT_EMAIL")
	viper.BindEnv("letsencrypt.environment", "NXPOSE_LETSENCRYPT_ENVIRONMENT")
	viper.BindEnv("letsencrypt.storage_dir", "NXPOSE_LETSENCRYPT_STORAGE_DIR")
	viper.BindEnv("letsencrypt.dns.provider", "NXPOSE_LETSENCRYPT_DNS_PROVIDER")

	// Admin panel settings
	viper.BindEnv("admin.enabled", "NXPOSE_ADMIN_ENABLED")
	viper.BindEnv("admin.path_prefix", "NXPOSE_ADMIN_PATH_PREFIX")
	viper.BindEnv("admin.auth_method", "NXPOSE_ADMIN_AUTH_METHOD")
	viper.BindEnv("admin.username", "NXPOSE_ADMIN_USERNAME")
	viper.BindEnv("admin.password", "NXPOSE_ADMIN_PASSWORD")

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
					fmt.Printf("DEBUG:     - ClientSecret: [REDACTED] (length: %d)\n",
						len(p.ClientSecret))
					fmt.Printf("DEBUG:     - Scopes: %v\n", p.Scopes)
				}
			}
			config.OAuth2.Providers = providers
		} else {
			fmt.Printf("ERROR: Failed to unmarshal OAuth2 providers: %v\n", err)
		}
	} else if viper.GetBool("verbose") {
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

	// Redis settings
	if viper.IsSet("redis.enabled") {
		config.Redis.Enabled = viper.GetBool("redis.enabled")
	}
	if viper.IsSet("redis.host") {
		config.Redis.Host = viper.GetString("redis.host")
	}
	if viper.IsSet("redis.port") {
		config.Redis.Port = viper.GetInt("redis.port")
	}
	if viper.IsSet("redis.password") {
		config.Redis.Password = viper.GetString("redis.password")
	}
	if viper.IsSet("redis.db") {
		config.Redis.DB = viper.GetInt("redis.db")
	}
	if viper.IsSet("redis.key_prefix") {
		config.Redis.KeyPrefix = viper.GetString("redis.key_prefix")
	}
	if viper.IsSet("redis.timeout") {
		timeout, err := time.ParseDuration(viper.GetString("redis.timeout"))
		if err == nil {
			config.Redis.Timeout = timeout
		}
	}

	// Tunnel limits
	if viper.IsSet("tunnels.max_per_user") {
		config.TunnelLimits.MaxPerUser = viper.GetInt("tunnels.max_per_user")
	}
	if viper.IsSet("tunnels.max_connection") {
		config.TunnelLimits.MaxConnection = viper.GetString("tunnels.max_connection")
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

	// Admin panel settings
	if viper.IsSet("admin.enabled") {
		config.Admin.Enabled = viper.GetBool("admin.enabled")
	}
	if viper.IsSet("admin.path_prefix") {
		config.Admin.PathPrefix = viper.GetString("admin.path_prefix")
	}
	if viper.IsSet("admin.auth_method") {
		config.Admin.AuthMethod = viper.GetString("admin.auth_method")
	}
	if viper.IsSet("admin.username") {
		config.Admin.Username = viper.GetString("admin.username")
	}
	if viper.IsSet("admin.password") {
		config.Admin.Password = viper.GetString("admin.password")
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

	// Redis settings
	viper.Set("redis.enabled", config.Redis.Enabled)
	viper.Set("redis.host", config.Redis.Host)
	viper.Set("redis.port", config.Redis.Port)
	viper.Set("redis.password", config.Redis.Password)
	viper.Set("redis.db", config.Redis.DB)
	viper.Set("redis.key_prefix", config.Redis.KeyPrefix)
	viper.Set("redis.timeout", config.Redis.Timeout.String())

	// Tunnel limits
	viper.Set("tunnels.max_per_user", config.TunnelLimits.MaxPerUser)
	viper.Set("tunnels.max_connection", config.TunnelLimits.MaxConnection)

	// Set Let's Encrypt settings
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

	// Admin panel settings
	viper.Set("admin.enabled", config.Admin.Enabled)
	viper.Set("admin.path_prefix", config.Admin.PathPrefix)
	viper.Set("admin.auth_method", config.Admin.AuthMethod)
	viper.Set("admin.username", config.Admin.Username)
	viper.Set("admin.password", config.Admin.Password)

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
