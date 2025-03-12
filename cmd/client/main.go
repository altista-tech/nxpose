package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"nxpose/internal/config"
	"nxpose/internal/crypto"
	"nxpose/internal/logger"
	"nxpose/internal/tunnel"
)

var (
	// Configuration
	cfg        *config.Config
	configFile string
	log        *logger.Logger
)

func main() {
	// Initialize with default config
	cfg = config.DefaultConfig()

	// Create the root command
	rootCmd := &cobra.Command{
		Use:   "nxpose",
		Short: "A secure tunneling service to expose local services to the internet",
		Long: `nxpose is a Go-based secure tunneling service that allows exposing
local services to the internet through secure tunnels.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Load configuration
			loadedCfg, err := config.LoadConfig(configFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not load configuration: %v\n", err)
			} else {
				cfg = loadedCfg
			}

			// Initialize logger
			log = logger.New(cfg.Verbose)
			log.Debug("Configuration loaded successfully")
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is $HOME/.nxpose/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&cfg.ServerHost, "server", "s", cfg.ServerHost, "Server hostname or IP address")
	rootCmd.PersistentFlags().IntVarP(&cfg.ServerPort, "port", "p", cfg.ServerPort, "Server port")
	rootCmd.PersistentFlags().BoolVarP(&cfg.Verbose, "verbose", "v", cfg.Verbose, "Enable verbose output")
	rootCmd.PersistentFlags().StringVar(&cfg.TLSCert, "tls-cert", cfg.TLSCert, "Path to TLS certificate file")
	rootCmd.PersistentFlags().StringVar(&cfg.TLSKey, "tls-key", cfg.TLSKey, "Path to TLS key file")

	// Add subcommands
	rootCmd.AddCommand(createRegisterCommand())
	rootCmd.AddCommand(createExposeCommand())

	// Execute the command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func createExposeCommand() *cobra.Command {
	var keepAlive bool

	cmd := &cobra.Command{
		Use:   "expose [protocol] [port]",
		Short: "Expose a local service to the internet",
		Long:  `Expose a local service running on a specified port to the internet through a secure tunnel.`,
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			// Parse arguments
			protocol := args[0]
			port, err := strconv.Atoi(args[1])
			if err != nil {
				log.WithError(err).Error("Invalid port number")
				fmt.Fprintf(os.Stderr, "Error: Invalid port number '%s'\n", args[1])
				os.Exit(1)
			}

			// Update config with command line arguments
			cfg.Protocol = protocol
			cfg.LocalPort = port

			log.WithFields(map[string]interface{}{
				"protocol": protocol,
				"port":     port,
				"server":   cfg.ServerHost,
			}).Info("Exposing local service")

			// Check if we have certificate data
			if cfg.CertData == nil || len(cfg.CertData) == 0 {
				// Try to load from file
				certData, err := config.LoadCertificateData("")
				if err != nil {
					log.WithError(err).Error("No certificate data available. Please run 'nxpose register' first")
					fmt.Fprintln(os.Stderr, "Error: No certificate data available. Please run 'nxpose register' first")
					os.Exit(1)
				}
				cfg.CertData = certData
				log.Debug("Loaded certificate data from file")
			}

			// Call the tunnel service to expose the local service
			// Using the updated ExposeLocalService function that returns both URL and ID
			publicURL, tunnelID, err := tunnel.ExposeLocalService(protocol, port, cfg.CertData, cfg.ServerHost, cfg.ServerPort)
			if err != nil {
				log.WithError(err).Error("Failed to expose local service")
				fmt.Fprintf(os.Stderr, "Error: Failed to expose local service: %v\n", err)
				os.Exit(1)
			}

			log.WithField("publicURL", publicURL).Info("Local service exposed successfully")
			fmt.Printf("Local service exposed successfully at: %s\n", publicURL)

			// Start the tunnel with the server-provided tunnel ID
			if err := tunnel.StartTunnel(protocol, port, publicURL, tunnelID, cfg.CertData); err != nil {
				log.WithError(err).Error("Failed to start tunnel")
				fmt.Fprintf(os.Stderr, "Error: Failed to start tunnel: %v\n", err)
				os.Exit(1)
			}

			keepAlive = true

			// If keepAlive is true, keep the process running
			if keepAlive {
				fmt.Println("Tunnel active. Press Ctrl+C to stop.")
				// Wait indefinitely or until interrupted
				c := make(chan os.Signal, 1)
				signal.Notify(c, os.Interrupt, syscall.SIGTERM)
				<-c
				fmt.Println("\nShutting down tunnel...")
			} else {
				// For demo purposes, just wait a bit to see some activity
				time.Sleep(15 * time.Second)
				fmt.Println("Tunnel demo completed. In a real implementation, the tunnel would remain active.")
			}
		},
	}

	// Expose-specific flags
	cmd.Flags().BoolVar(&keepAlive, "keep-alive", false, "Keep the tunnel running until interrupted")

	return cmd
}

// createRegisterCommand creates the 'register' subcommand
func createRegisterCommand() *cobra.Command {
	var saveConfig bool
	var saveCert bool
	var forceNewCert bool // New flag to force certificate regeneration

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register with the nxpose server and obtain certificates",
		Long:  `Connect to the nxpose server to register and obtain the necessary certificates for secure tunneling.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Log connection attempt
			log.WithField("server", fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort)).
				Info("Connecting to server")

			// Call the RegisterWithServer function to get a certificate
			// Now passing the forceNewCert flag
			certificate, err := crypto.RegisterWithServer(cfg.ServerHost, cfg.ServerPort, forceNewCert)
			if err != nil {
				log.WithError(err).Error("Failed to register with server")
				fmt.Fprintf(os.Stderr, "Error: Failed to register with server: %v\n", err)
				os.Exit(1)
			}

			// Store certificate in config if successful
			// In the "register" command handler
			if certificate != "" {
				log.Info("Successfully registered with server and obtained certificate")

				// Store the certificate data in the config
				cfg.CertData = []byte(certificate)

				// Display success message with certificate snippet
				fmt.Println("Successfully registered with server")
				fmt.Println("Certificate snippet:")

				// Print just the first line of the certificate for brevity
				certLines := strings.Split(certificate, "\n")
				if len(certLines) > 0 {
					fmt.Println(certLines[0] + "...")
				}

				// Always save certificate to file - no need for a flag
				err := config.SaveCertificateData([]byte(certificate), "")
				if err != nil {
					log.WithError(err).Error("Failed to save certificate")
					fmt.Fprintf(os.Stderr, "Error: Failed to save certificate: %v\n", err)
				} else {
					log.Info("Certificate saved to file")
					fmt.Println("Certificate saved successfully")
				}
			}

			// If save config flag is set, save the configuration
			if saveConfig {
				if err := config.SaveConfig(cfg, ""); err != nil {
					log.WithError(err).Error("Failed to save configuration")
				} else {
					log.Info("Configuration saved successfully")
					fmt.Println("Configuration saved successfully")
				}
			}
		},
	}

	// Register-specific flags
	cmd.Flags().BoolVar(&saveConfig, "save-config", false, "Save configuration after registration")
	cmd.Flags().BoolVar(&saveCert, "save-cert", false, "Save certificate to file after registration")
	cmd.Flags().BoolVar(&forceNewCert, "force-new-cert", false, "Force generation of a new certificate even if one exists")

	return cmd
}
