package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	// Server related
	ServerHost string
	ServerPort int

	// Client related
	LocalPort      int
	Protocol       string
	SubdomainID    string
	SkipLocalCheck bool

	// Common settings
	Verbose  bool
	TLSCert  string
	TLSKey   string
	CertData []byte // For storing certificate data after registration
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	return &Config{
		ServerHost:     "nxpose.naxrevlis.com",
		ServerPort:     443,
		LocalPort:      3000,
		Protocol:       "https",
		SubdomainID:    "",
		SkipLocalCheck: false,
		Verbose:        false,
		TLSCert:        "",
		TLSKey:         "",
		CertData:       nil,
	}
}

// LoadConfig loads configuration from config files and environment variables
func LoadConfig(configFile string) (*Config, error) {
	config := DefaultConfig()

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
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Enable environment variables to override config files
	viper.SetEnvPrefix("NXPOSE")
	viper.AutomaticEnv()

	// Try to read config from file
	configReadError := viper.ReadInConfig()
	configNotFound := false

	if configReadError != nil {
		if _, ok := configReadError.(viper.ConfigFileNotFoundError); ok {
			configNotFound = true
		} else {
			return nil, fmt.Errorf("error reading config file: %w", configReadError)
		}
	}

	// If config is not found, create a default one
	if configNotFound {
		// Save the default config
		if err := SaveConfig(config, ""); err != nil {
			// Just log the error but continue with default config
			fmt.Fprintf(os.Stderr, "Warning: Could not save default configuration: %v\n", err)
		} else {
			fmt.Println("Created default configuration file with server: https://nxpose.naxrevlsi.com:8443")
		}
	}

	// Map config values from Viper to our Config struct
	if viper.IsSet("server.host") {
		config.ServerHost = viper.GetString("server.host")
	}
	if viper.IsSet("server.port") {
		config.ServerPort = viper.GetInt("server.port")
	}
	if viper.IsSet("client.local_port") {
		config.LocalPort = viper.GetInt("client.local_port")
	}
	if viper.IsSet("client.protocol") {
		config.Protocol = viper.GetString("client.protocol")
	}
	if viper.IsSet("client.subdomain") {
		config.SubdomainID = viper.GetString("client.subdomain")
	}
	if viper.IsSet("client.skip_local_check") {
		config.SkipLocalCheck = viper.GetBool("client.skip_local_check")
	}
	if viper.IsSet("verbose") {
		config.Verbose = viper.GetBool("verbose")
	}
	if viper.IsSet("tls.cert") {
		config.TLSCert = viper.GetString("tls.cert")
	}
	if viper.IsSet("tls.key") {
		config.TLSKey = viper.GetString("tls.key")
	}

	return config, nil
}

// SaveConfig saves the configuration to a file
func SaveConfig(config *Config, filePath string) error {
	// Set values in viper
	viper.Set("server.host", config.ServerHost)
	viper.Set("server.port", config.ServerPort)
	viper.Set("client.local_port", config.LocalPort)
	viper.Set("client.protocol", config.Protocol)
	viper.Set("client.subdomain", config.SubdomainID)
	viper.Set("verbose", config.Verbose)
	viper.Set("tls.cert", config.TLSCert)
	viper.Set("tls.key", config.TLSKey)

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

		filePath = filepath.Join(configDir, "config.yaml")
	}

	// Write config to file
	if err := viper.WriteConfigAs(filePath); err != nil {
		return fmt.Errorf("could not save config: %w", err)
	}

	return nil
}

// SaveCertificateData saves the certificate data to a separate file
func SaveCertificateData(certData []byte, filePath string) error {
	if filePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home directory: %w", err)
		}

		configDir := filepath.Join(homeDir, ".nxpose")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("could not create config directory: %w", err)
		}

		filePath = filepath.Join(configDir, "nxpose.cert")
	}

	// Write certificate data to file
	if err := os.WriteFile(filePath, certData, 0600); err != nil {
		return fmt.Errorf("could not save certificate data: %w", err)
	}

	return nil
}

// StoreCertificate stores the certificate string in the global configuration
func StoreCertificate(certStr string) {
	// Get the current configuration from the DefaultConfig
	cfg := DefaultConfig()

	// Store the certificate data
	cfg.CertData = []byte(certStr)
}

// LoadCertificateData loads the certificate data from a file
func LoadCertificateData(filePath string) ([]byte, error) {
	if filePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not determine home directory: %w", err)
		}

		filePath = filepath.Join(homeDir, ".nxpose", "nxpose.cert")
	}

	// Read certificate data from file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not load certificate data: %w", err)
	}

	return data, nil
}
