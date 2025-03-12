package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"nxpose/internal/config"
	"nxpose/internal/crypto"
	"nxpose/internal/logger"
	"nxpose/internal/server"
	"nxpose/yaml_utils"

	"github.com/spf13/cobra"
)

var (
	// Configuration
	cfg        *config.ServerConfig
	configFile string
	log        *logger.Logger
)

func main() {
	// Initialize with default config
	cfg = config.DefaultServerConfig()

	// Create the root command
	rootCmd := &cobra.Command{
		Use:   "nxpose-server",
		Short: "Run the nxpose tunneling server",
		Long:  `nxpose-server runs the public-facing server component of the nxpose secure tunneling service.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Load configuration
			loadedCfg, err := config.LoadServerConfig(configFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not load configuration: %v\n", err)
			} else {
				cfg = loadedCfg
			}

			// Initialize logger
			log = logger.New(cfg.Verbose)

			if configFile == "" {
				log.Info("No config file specified, using environment variables and defaults")
			} else {
				log.Info("Configuration loaded from file: " + configFile)
			}

			log.Debug("Server configuration loaded successfully")
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Start the server
			runServer()
		},
	}

	// Add check-yaml command for debugging configuration
	checkYamlCmd := &cobra.Command{
		Use:   "check-yaml",
		Short: "Check YAML config file parsing",
		Run: func(cmd *cobra.Command, args []string) {
			if configFile == "" {
				fmt.Println("Error: No config file specified")
				return
			}
			fmt.Printf("===== CHECKING YAML FILE: %s =====\n", configFile)
			fmt.Println("\n=== GENERIC YAML PARSING ===")
			yaml_utils.CheckYAMLFile(configFile)
			fmt.Println("\n=== DIRECT OAUTH2 PARSING ===")
			yaml_utils.DirectUnmarshalOAuth2(configFile)
			fmt.Println("\n=== END OF CHECK ===")
		},
	}
	rootCmd.AddCommand(checkYamlCmd)

	// Add validate-github command
	validateGitHubCmd := &cobra.Command{
		Use:   "validate-github",
		Short: "Validate GitHub OAuth credentials in config",
		Run: func(cmd *cobra.Command, args []string) {
			if configFile == "" {
				fmt.Println("Error: No config file specified")
				return
			}
			fmt.Printf("Validating GitHub credentials in: %s\n", configFile)
			yaml_utils.ValidateGitHubCredentials(configFile)
		},
	}
	rootCmd.AddCommand(validateGitHubCmd)

	// Add fix-yaml command
	fixYamlCmd := &cobra.Command{
		Use:   "fix-yaml",
		Short: "Fix YAML config file structure",
		Run: func(cmd *cobra.Command, args []string) {
			if configFile == "" {
				fmt.Println("Error: No config file specified")
				return
			}

			outputFile := configFile + ".fixed"
			fmt.Printf("Fixing YAML file: %s -> %s\n", configFile, outputFile)
			yaml_utils.FixYAMLFile(configFile, outputFile)
		},
	}
	rootCmd.AddCommand(fixYamlCmd)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is $HOME/.nxpose/server-config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&cfg.BindAddress, "bind", "b", cfg.BindAddress, "Address to bind the server to")
	rootCmd.PersistentFlags().IntVarP(&cfg.Port, "port", "p", cfg.Port, "Port to listen on")
	rootCmd.PersistentFlags().BoolVarP(&cfg.Verbose, "verbose", "v", cfg.Verbose, "Enable verbose output")
	rootCmd.PersistentFlags().StringVar(&cfg.TLSCert, "tls-cert", cfg.TLSCert, "Path to TLS certificate file")
	rootCmd.PersistentFlags().StringVar(&cfg.TLSKey, "tls-key", cfg.TLSKey, "Path to TLS key file")
	rootCmd.PersistentFlags().StringVar(&cfg.BaseDomain, "domain", cfg.BaseDomain, "Base domain for tunnels")

	// Execute the command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runServer() {
	log.WithFields(map[string]interface{}{
		"address": cfg.BindAddress,
		"port":    cfg.Port,
		"domain":  cfg.BaseDomain,
	}).Info("Starting nxpose server")

	// Load or generate TLS certificates
	tlsConfig, err := crypto.LoadOrGenerateServerCertificate(cfg.TLSCert, cfg.TLSKey, log.Logger)
	if err != nil {
		log.WithError(err).Fatal("Failed to set up TLS configuration")
		fmt.Fprintf(os.Stderr, "Error: Failed to set up TLS configuration: %v\n", err)
		os.Exit(1)
	}
	log.Info("TLS configuration loaded successfully")

	// Initialize and start the server
	srv, err := server.NewServer(cfg, tlsConfig, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize server")
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize server: %v\n", err)
		os.Exit(1)
	}

	// Start the server in a goroutine
	go func() {
		if err := srv.Start(); err != nil {
			log.WithError(err).Fatal("Server failed to start")
			fmt.Fprintf(os.Stderr, "Error: Server failed to start: %v\n", err)
			os.Exit(1)
		}
	}()

	fmt.Printf("nxpose server started on %s:%d\n", cfg.BindAddress, cfg.Port)
	fmt.Println("Press Ctrl+C to stop the server")

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down server...")
	if err := srv.Stop(); err != nil {
		log.WithError(err).Error("Error while shutting down server")
		fmt.Fprintf(os.Stderr, "Error while shutting down: %v\n", err)
	}
	log.Info("Server shutdown complete")
}
