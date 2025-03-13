package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

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
	// Server binding
	BindAddress string
	Port        int
	BaseDomain  string

	// TLS settings
	TLSCert string
	TLSKey  string

	// Let's Encrypt settings
	LetsEncrypt LetsEncryptConfig

	// Common settings
	Verbose bool
}

// DefaultServerConfig returns a server config with default values
func DefaultServerConfig() *ServerConfig {
	// Get home directory for default storage
	homeDir, err := os.UserHomeDir()
	storageDir := ""
	if err == nil {
		storageDir = filepath.Join(homeDir, ".nxpose", "certificates")
	}

	return &ServerConfig{
		BindAddress: "0.0.0.0",
		Port:        8080,
		BaseDomain:  "nxpose.local",
		TLSCert:     "",
		TLSKey:      "",
		LetsEncrypt: LetsEncryptConfig{
			Enabled:        false,
			Email:          "",
			Environment:    crypto.ProductionEnv,
			StorageDir:     storageDir,
			DNSProvider:    "",
			DNSCredentials: make(map[string]string),
		},
		Verbose: false,
	}
}

// LoadServerConfig loads server configuration from config files and environment variables
func LoadServerConfig(configFile string) (*ServerConfig, error) {
	config := DefaultServerConfig()

	// If configFile is provided, use it directly
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
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
	}

	// Enable environment variables to override config files
	viper.SetEnvPrefix("NXPOSE_SERVER")
	viper.AutomaticEnv()

	// Try to read config from file (doesn't error if file doesn't exist)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Map config values from Viper to our Config struct
	if viper.IsSet("server.bind_address") {
		config.BindAddress = viper.GetString("server.bind_address")
	}
	if viper.IsSet("server.port") {
		config.Port = viper.GetInt("server.port")
	}
	if viper.IsSet("server.domain") {
		config.BaseDomain = viper.GetString("server.domain")
	}
	if viper.IsSet("tls.cert") {
		config.TLSCert = viper.GetString("tls.cert")
	}
	if viper.IsSet("tls.key") {
		config.TLSKey = viper.GetString("tls.key")
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

	if viper.IsSet("verbose") {
		config.Verbose = viper.GetBool("verbose")
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

// SaveServerConfig saves the server configuration to a file
func SaveServerConfig(config *ServerConfig, filePath string) error {
	// Set values in viper
	viper.Set("server.bind_address", config.BindAddress)
	viper.Set("server.port", config.Port)
	viper.Set("server.domain", config.BaseDomain)
	viper.Set("tls.cert", config.TLSCert)
	viper.Set("tls.key", config.TLSKey)

	// Let's Encrypt settings
	viper.Set("letsencrypt.enabled", config.LetsEncrypt.Enabled)
	viper.Set("letsencrypt.email", config.LetsEncrypt.Email)
	if config.LetsEncrypt.Environment == crypto.StagingEnv {
		viper.Set("letsencrypt.environment", "staging")
	} else {
		viper.Set("letsencrypt.environment", "production")
	}
	viper.Set("letsencrypt.storage_dir", config.LetsEncrypt.StorageDir)

	viper.Set("verbose", config.Verbose)

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
