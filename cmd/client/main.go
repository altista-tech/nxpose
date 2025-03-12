package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
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
	rootCmd.AddCommand(createStatusCommand())

	// Execute the command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// getServerStatus queries the server for its status information
func getServerStatus(serverHost string, serverPort int) (map[string]interface{}, error) {
	// Construct the status API URL
	url := fmt.Sprintf("https://%s:%d/api/status", serverHost, serverPort)

	// Create an HTTP client with TLS configuration
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Send the request
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	// Check response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned non-OK status: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse the JSON response
	var status map[string]interface{}
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return status, nil
}

func createExposeCommand() *cobra.Command {
	var keepAlive bool
	var skipLocalCheck bool

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
			cfg.SkipLocalCheck = skipLocalCheck

			log.WithFields(map[string]interface{}{
				"protocol":         protocol,
				"port":             port,
				"server":           cfg.ServerHost,
				"skip_local_check": skipLocalCheck,
			}).Info("Exposing local service")

			// Check if we have certificate data
			if len(cfg.CertData) == 0 {
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
			if err := tunnel.StartTunnel(protocol, port, publicURL, tunnelID, cfg.CertData, skipLocalCheck); err != nil {
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
	cmd.Flags().BoolVar(&skipLocalCheck, "skip-local-check", false, "Skip checking if the local service is available before creating the tunnel")

	return cmd
}

// createRegisterCommand creates the 'register' subcommand
func createRegisterCommand() *cobra.Command {
	var saveConfig bool
	var saveCert bool
	var forceNewCert bool // Flag to force certificate regeneration
	var skipOAuth bool    // Flag to explicitly skip OAuth when it's enabled on the server

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register with the nxpose server and obtain certificates",
		Long: `Connect to the nxpose server to register and obtain the necessary certificates for secure tunneling.

By default, OAuth2 authentication will be used if the server supports it.
To bypass OAuth authentication (not recommended), use the --skip-oauth flag.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Log connection attempt
			log.WithField("server", fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort)).
				Info("Connecting to server")

			var certificate string
			var err error

			// Use OAuth by default unless explicitly disabled with skipOAuth flag
			if !skipOAuth {
				// Check if server supports OAuth
				if crypto.CheckOAuthSupport(cfg.ServerHost, cfg.ServerPort) {
					// OAuth2 flow for registration
					certificate, err = registerWithOAuth(cfg.ServerHost, cfg.ServerPort, forceNewCert)
					if err != nil {
						log.WithError(err).Error("Failed to register with server using OAuth")
						fmt.Fprintf(os.Stderr, "Error: Failed to register with server using OAuth: %v\n", err)
						os.Exit(1)
					}
				} else {
					// Server doesn't support OAuth, so we can't proceed
					log.Error("Server does not support OAuth authentication, and OAuth is required")
					fmt.Fprintln(os.Stderr, "Error: Server does not support OAuth authentication.")
					fmt.Fprintln(os.Stderr, "OAuth authentication is required for secure registration.")
					fmt.Fprintln(os.Stderr, "Please ensure the server has OAuth configured properly.")
					os.Exit(1)
				}
			} else {
				// Only use traditional registration if OAuth is explicitly skipped
				fmt.Println("Warning: Skipping OAuth authentication as requested.")
				fmt.Println("This is not recommended for production use.")
				certificate, err = crypto.RegisterWithServer(cfg.ServerHost, cfg.ServerPort, forceNewCert)
				if err != nil {
					log.WithError(err).Error("Failed to register with server")
					fmt.Fprintf(os.Stderr, "Error: Failed to register with server: %v\n", err)
					os.Exit(1)
				}
			}

			log.Info("Successfully registered with server and obtained certificate")
			fmt.Println("Successfully registered with server")
			fmt.Println("Certificate snippet:")
			fmt.Println(certificate[:40] + "...")

			// Save certificate to file
			homeDir, err := os.UserHomeDir()
			if err == nil {
				// Create .nxpose directory if it doesn't exist
				configDir := filepath.Join(homeDir, ".nxpose")
				if err := os.MkdirAll(configDir, 0755); err != nil {
					fmt.Printf("Warning: Could not create config directory: %v\n", err)
				} else {
					certPath := filepath.Join(configDir, "client.crt")
					if err := os.WriteFile(certPath, []byte(certificate), 0644); err != nil {
						fmt.Printf("Warning: Could not save certificate: %v\n", err)
					} else {
						log.Info("Certificate saved to file")
						fmt.Println("Certificate saved successfully")
					}
				}
			}

			// Also update the config in memory for future commands in this session
			config.StoreCertificate(certificate)
		},
	}

	// Register-specific flags
	cmd.Flags().BoolVar(&saveConfig, "save-config", true, "Save registration information to config file")
	cmd.Flags().BoolVar(&saveCert, "save-cert", true, "Save certificate and key to disk")
	cmd.Flags().BoolVar(&forceNewCert, "force-new", false, "Force registration of a new certificate even if one exists")
	cmd.Flags().BoolVar(&skipOAuth, "skip-oauth", false, "Skip OAuth authentication (not recommended)")

	return cmd
}

// registerWithOAuth performs registration using OAuth2
func registerWithOAuth(host string, port int, forceNewCert bool) (string, error) {
	// For authentication URLs, use clean domain without port for standard HTTPS port
	var authURL string
	if port == 443 {
		// Don't include port 443 in the URL as it's the default HTTPS port
		authURL = fmt.Sprintf("https://%s/auth/register", host)
	} else {
		authURL = fmt.Sprintf("https://%s:%d/auth/register", host, port)
	}

	// Log the URL we're about to open
	log.WithField("url", authURL).Info("Opening OAuth registration URL in browser")

	// Generate a local state token for security
	stateToken := fmt.Sprintf("%x", rand.Int63())

	// Start a local HTTP server to receive the callback
	// Use a random available port for the callback server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", fmt.Errorf("failed to start local callback server: %w", err)
	}
	defer listener.Close()

	// Get the actual port that was assigned
	callbackPort := listener.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://localhost:%d/callback", callbackPort)

	// Store the certificate from the callback
	var certificate string
	var callbackErr error
	callbackDone := make(chan bool)

	// Set up a simple HTTP server to handle the callback
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		log.Infof("Received callback: %s", r.URL.String())

		// Check state token to prevent CSRF
		receivedState := r.URL.Query().Get("state")
		if receivedState != stateToken {
			log.Errorf("Invalid state token in callback. Expected: %s, Got: %s", stateToken, receivedState)
			callbackErr = fmt.Errorf("invalid state token in callback")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error: Invalid state token. Authentication failed.")
			callbackDone <- true
			return
		}

		// Get the certificate from the callback
		certData := r.URL.Query().Get("certificate")
		if certData == "" {
			log.Info("No certificate data in callback, checking for error message")
			errMsg := r.URL.Query().Get("error")
			if errMsg == "" {
				// Log all query parameters for debugging
				log.Info("Callback URL parameters:")
				for key, values := range r.URL.Query() {
					log.Infof("  %s: %v", key, values)
				}
				errMsg = "no certificate data received"
			}
			log.WithField("error", errMsg).Error("OAuth registration failed")
			callbackErr = fmt.Errorf("OAuth registration failed: %s", errMsg)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error: %s", errMsg)
			callbackDone <- true
			return
		}

		log.Infof("Certificate data received (length: %d characters)", len(certData))

		// Decode the certificate (it's base64 encoded)
		decodedCert, err := base64.StdEncoding.DecodeString(certData)
		if err != nil {
			log.WithError(err).Error("Failed to decode certificate data")
			callbackErr = fmt.Errorf("failed to decode certificate data: %w", err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error decoding certificate data.")
			callbackDone <- true
			return
		}

		log.Infof("Decoded certificate (length: %d bytes)", len(decodedCert))

		// Store the certificate
		certificate = string(decodedCert)

		// Show success page to user
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Registration successful! You can close this window and return to the nxpose client.")

		log.Info("Registration completed successfully")

		// Signal that the callback processing is complete
		callbackDone <- true
	})

	// Open the authentication URL in the browser
	// Build the full URL with callback and state token
	fullAuthURL := fmt.Sprintf("%s?callback=%s&state=%s",
		authURL,
		url.QueryEscape(callbackURL),
		stateToken)

	// Open the URL in the default browser
	log.WithField("url", fullAuthURL).Info("Opening OAuth registration URL")
	if err := openBrowser(fullAuthURL); err != nil {
		return "", fmt.Errorf("failed to open browser: %w", err)
	}

	// Start the callback server
	go func() {
		if err := http.Serve(listener, nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.WithError(err).Error("Callback server error")
		}
	}()

	// Wait for the callback to complete or timeout
	select {
	case <-callbackDone:
		if callbackErr != nil {
			return "", callbackErr
		}
		return certificate, nil
	case <-time.After(5 * time.Minute):
		return "", fmt.Errorf("authentication timed out after 5 minutes")
	}
}

// openBrowser opens the specified URL in the default browser
func openBrowser(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	return err
}

// createStatusCommand creates the 'status' subcommand
func createStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check the status of the nxpose server",
		Long:  `Connect to the nxpose server to check its status, including certificate information.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Log connection attempt
			log.WithField("server", fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort)).
				Info("Connecting to server for status check")

			// Query server status
			status, err := getServerStatus(cfg.ServerHost, cfg.ServerPort)
			if err != nil {
				log.WithError(err).Error("Failed to get server status")
				fmt.Fprintf(os.Stderr, "Error: Failed to get server status: %v\n", err)
				os.Exit(1)
			}

			// Display the status information
			fmt.Println("🟢 NXpose Server Status")
			fmt.Println("====================")

			// Print version and uptime
			fmt.Printf("Version: %s\n", status["version"])
			fmt.Printf("Uptime: %s\n", status["uptime"])
			fmt.Printf("Active tunnels: %d\n", int(status["tunnels"].(float64)))
			fmt.Println()

			// Print TLS information
			tlsInfo := status["tls"].(map[string]interface{})
			fmt.Println("🔒 TLS Configuration")
			fmt.Printf("Provider: %s\n", tlsInfo["provider"])

			// If Let's Encrypt is enabled, show certificate details
			if tlsInfo["provider"] == "Let's Encrypt" && tlsInfo["certificates"] != nil {
				certInfo := tlsInfo["certificates"].(map[string]interface{})

				fmt.Println("Let's Encrypt Configuration:")
				fmt.Printf("  Email: %s\n", certInfo["email"])
				fmt.Printf("  Environment: %s\n", certInfo["environment"])

				// Display certificate information for each domain
				if certs, ok := certInfo["certificates"].(map[string]interface{}); ok {
					fmt.Println("\nCertificates:")
					for domain, info := range certs {
						certDetails := info.(map[string]interface{})

						// Check if there was an error obtaining the certificate
						if errMsg, hasError := certDetails["error"]; hasError {
							fmt.Printf("  %s: Error - %s\n", domain, errMsg)
							continue
						}

						// Format and display certificate information
						fmt.Printf("  %s:\n", domain)
						fmt.Printf("    Issuer: %s\n", certDetails["issuer"])

						// Convert the time strings to Time objects and format them
						if notBefore, ok := certDetails["notBefore"].(string); ok {
							if t, err := time.Parse(time.RFC3339, notBefore); err == nil {
								fmt.Printf("    Valid from: %s\n", t.Format("2006-01-02 15:04:05"))
							}
						}

						if notAfter, ok := certDetails["notAfter"].(string); ok {
							if t, err := time.Parse(time.RFC3339, notAfter); err == nil {
								fmt.Printf("    Valid until: %s\n", t.Format("2006-01-02 15:04:05"))
							}
						}

						// Calculate and display days until expiration
						if notAfter, ok := certDetails["notAfter"].(string); ok {
							if t, err := time.Parse(time.RFC3339, notAfter); err == nil {
								daysLeft := int(time.Until(t).Hours() / 24)
								fmt.Printf("    Days until expiration: %d\n", daysLeft)
							}
						}
					}
				}
			}
		},
	}

	return cmd
}
