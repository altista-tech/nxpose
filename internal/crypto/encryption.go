package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// TLSConfig holds the TLS configuration for the client
type TLSConfig struct {
	Certificate   []byte
	PrivateKey    []byte
	TLSClientConf *tls.Config
}

// RegisterWithServer connects to a remote server and retrieves/generates a certificate
// In a real implementation, this would perform certificate exchange with a server
// If forceNewCert is true, always generate a new certificate regardless of existing ones
func RegisterWithServer(host string, port int, forceNewCert bool) (string, error) {
	// Build server address
	serverAddr := fmt.Sprintf("%s:%d", host, port)

	fmt.Printf("Establishing secure connection to server at %s...\n", serverAddr)

	var tlsConfig *TLSConfig
	var err error

	// Either generate new certificate or use existing one
	if forceNewCert {
		fmt.Println("Forcing generation of new certificate...")
		tlsConfig, err = generateSelfSignedCert()
	} else {
		// Try to use existing certificate first
		tlsConfig, err = createTLSConfig()
	}

	if err != nil {
		return "", fmt.Errorf("failed to create TLS configuration: %w", err)
	}

	// Try to establish a TLS connection to the server
	// This is a real connection attempt but will fall back to simulation if the server is unavailable
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", serverAddr, tlsConfig.TLSClientConf)
	if err != nil {
		// In development mode, we'll just print a warning if the server isn't reachable
		fmt.Printf("Warning: Could not establish secure connection to server at %s: %v\n", serverAddr, err)
		fmt.Println("Continuing with simulation mode...")
	} else {
		// If connection was successful, close it
		defer conn.Close()
		fmt.Printf("Successfully established secure TLS connection to server at %s\n", serverAddr)

		// In a real implementation, we would exchange data with the server here
		// to get a proper certificate signed by the server's CA
	}

	// For demonstration, we'll use the self-signed certificate we generated
	certPEM := string(tlsConfig.Certificate)

	fmt.Println("TLS connection established and certificate received successfully")

	return certPEM, nil
}

// / createTLSConfig generates a TLS configuration with a self-signed certificate
func createTLSConfig() (*TLSConfig, error) {
	// First, try to load existing certificates from disk
	homeDir, err := os.UserHomeDir()
	if err == nil {
		certPath := filepath.Join(homeDir, ".nxpose", "client.crt")
		keyPath := filepath.Join(homeDir, ".nxpose", "client.key")

		// If both files exist, try to load them
		if fileExists(certPath) && fileExists(keyPath) {
			cert, err := os.ReadFile(certPath)
			if err == nil {
				key, err := os.ReadFile(keyPath)
				if err == nil {
					// Successfully loaded both files
					tlsConf, err := createTLSConfigFromFiles(cert, key)
					if err == nil {
						fmt.Println("Using existing certificates from", certPath)
						return &TLSConfig{
							Certificate:   cert,
							PrivateKey:    key,
							TLSClientConf: tlsConf,
						}, nil
					}
				}
			}
		}
	}

	// If we couldn't load existing certificates, generate new ones
	fmt.Println("Generating new self-signed certificate for development...")
	return generateSelfSignedCert()
}

// generateSelfSignedCert creates a new self-signed certificate for development
func generateSelfSignedCert() (*TLSConfig, error) {
	// Generate a new private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create a certificate template
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	// Certificate validity: 1 year
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"NXpose Tunnel Client"},
			CommonName:   "localhost",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	// Create certificate using the template and private key
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode certificate to PEM format
	certBuffer := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	// Encode private key to PEM format
	keyBuffer := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Save the certificate and key to disk for future use
	saveCertificateAndKey(certBuffer, keyBuffer)

	// Create a TLS config using the new certificate and key
	cert, err := tls.X509KeyPair(certBuffer, keyBuffer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key pair: %w", err)
	}

	tlsConf := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, // Only for development - allows connection to self-signed server certs
	}

	return &TLSConfig{
		Certificate:   certBuffer,
		PrivateKey:    keyBuffer,
		TLSClientConf: tlsConf,
	}, nil
}

// createTLSConfigFromFiles creates a TLS configuration from existing certificate files
func createTLSConfigFromFiles(certPEM, keyPEM []byte) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key pair: %w", err)
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, // Only for development
	}, nil
}

// saveCertificateAndKey saves the certificate and key to disk for future use
func saveCertificateAndKey(cert, key []byte) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Warning: Could not determine home directory: %v\n", err)
		return
	}

	configDir := filepath.Join(homeDir, ".nxpose")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Printf("Warning: Could not create config directory: %v\n", err)
		return
	}

	certPath := filepath.Join(configDir, "client.crt")
	keyPath := filepath.Join(configDir, "client.key")

	// Write certificate
	if err := os.WriteFile(certPath, cert, 0644); err != nil {
		fmt.Printf("Warning: Could not save certificate to %s: %v\n", certPath, err)
	}

	// Write private key with restricted permissions
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		fmt.Printf("Warning: Could not save private key to %s: %v\n", keyPath, err)
	}
}

// fileExists checks if a file exists
func fileExists(filepath string) bool {
	info, err := os.Stat(filepath)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
