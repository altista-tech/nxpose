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

// LoadOrGenerateServerCertificate loads existing TLS certificates or generates new ones for the server
func LoadOrGenerateServerCertificate(certPath, keyPath string) (*tls.Config, error) {
	// If paths are provided, try to load the certificates
	if certPath != "" && keyPath != "" {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		return tlsConfig, nil
	}

	// If paths are not provided, check for default certificates
	homeDir, err := os.UserHomeDir()
	if err == nil {
		defaultCertPath := filepath.Join(homeDir, ".nxpose", "server.crt")
		defaultKeyPath := filepath.Join(homeDir, ".nxpose", "server.key")

		if fileExists(defaultCertPath) && fileExists(defaultKeyPath) {
			cert, err := tls.LoadX509KeyPair(defaultCertPath, defaultKeyPath)
			if err == nil {
				tlsConfig := &tls.Config{
					Certificates: []tls.Certificate{cert},
					MinVersion:   tls.VersionTLS12,
				}
				fmt.Printf("Using existing certificates from %s\n", defaultCertPath)
				return tlsConfig, nil
			}
		}
	}

	// If we couldn't load existing certificates, generate new ones
	fmt.Println("Generating new self-signed certificate for the server...")
	return generateServerCertificate()
}

// generateServerCertificate creates a new self-signed certificate for the server
func generateServerCertificate() (*tls.Config, error) {
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
			Organization: []string{"NXpose Tunnel Server"},
			CommonName:   "nxpose.local",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost", "*.nxpose.local", "nxpose.local"},
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
	homeDir, err := os.UserHomeDir()
	if err == nil {
		configDir := filepath.Join(homeDir, ".nxpose")
		if err := os.MkdirAll(configDir, 0755); err == nil {
			certPath := filepath.Join(configDir, "server.crt")
			keyPath := filepath.Join(configDir, "server.key")

			// Write certificate
			if err := os.WriteFile(certPath, certBuffer, 0644); err != nil {
				fmt.Printf("Warning: Could not save certificate to %s: %v\n", certPath, err)
			} else {
				fmt.Printf("Server certificate saved to: %s\n", certPath)
			}

			// Write private key with restricted permissions
			if err := os.WriteFile(keyPath, keyBuffer, 0600); err != nil {
				fmt.Printf("Warning: Could not save private key to %s: %v\n", keyPath, err)
			} else {
				fmt.Printf("Server private key saved to: %s\n", keyPath)
			}
		}
	}

	// Create a TLS config using the new certificate and key
	cert, err := tls.X509KeyPair(certBuffer, keyBuffer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key pair: %w", err)
	}

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	return tlsConf, nil
}

// SignClientCertificate takes a client CSR and returns a signed certificate
// For a real implementation, this would verify the CSR and sign it with the server's CA
func SignClientCertificate(csrPEM []byte) ([]byte, error) {
	// Parse the CSR
	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("failed to decode CSR PEM")
	}

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSR: %w", err)
	}

	// In a real implementation, validate the CSR here

	// Load the server's CA certificate and key
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home directory: %w", err)
	}

	certPath := filepath.Join(homeDir, ".nxpose", "server.crt")
	keyPath := filepath.Join(homeDir, ".nxpose", "server.key")

	if !fileExists(certPath) || !fileExists(keyPath) {
		return nil, fmt.Errorf("server certificate or key not found")
	}

	// Load the server's certificate
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read server certificate: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("failed to decode server certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server certificate: %w", err)
	}

	// Load the server's private key
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read server key: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode server key PEM")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server private key: %w", err)
	}

	// Create a certificate template based on the CSR
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	// Certificate validity: 30 days
	notBefore := time.Now()
	notAfter := notBefore.Add(30 * 24 * time.Hour)

	template := x509.Certificate{
		SerialNumber:   serialNumber,
		Subject:        csr.Subject,
		NotBefore:      notBefore,
		NotAfter:       notAfter,
		KeyUsage:       x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		IsCA:           false,
		DNSNames:       csr.DNSNames,
		EmailAddresses: csr.EmailAddresses,
		IPAddresses:    csr.IPAddresses,
		URIs:           csr.URIs,
	}

	// Create certificate using the template, server's CA certificate, CSR's public key, and server's private key
	clientCertDER, err := x509.CreateCertificate(rand.Reader, &template, cert, csr.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create client certificate: %w", err)
	}

	// Encode the client certificate to PEM format
	clientCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: clientCertDER,
	})

	return clientCertPEM, nil
}

// For testing purposes, generate a dummy client certificate
func GenerateDummyClientCertificate() ([]byte, error) {
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

	// Certificate validity: 30 days
	notBefore := time.Now()
	notAfter := notBefore.Add(30 * 24 * time.Hour)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"NXpose Tunnel Client"},
			CommonName:   "client.nxpose.local",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Self-sign the certificate for testing purposes
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode certificate to PEM format
	certBuffer := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	return certBuffer, nil
}
