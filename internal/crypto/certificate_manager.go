package crypto

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/sirupsen/logrus"
)

// Environment defines the ACME environment to use
type Environment string

const (
	// ProductionEnv uses Let's Encrypt production environment
	ProductionEnv Environment = "production"
	// StagingEnv uses Let's Encrypt staging environment
	StagingEnv Environment = "staging"

	// Default renewal settings
	defaultRenewalDays = 30 // 30 days before expiration
	defaultCheckHours  = 12 // check every 12 hours
)

// CertificateManagerConfig holds the configuration for the certificate manager
type CertificateManagerConfig struct {
	// Existing fields
	Email       string
	Domains     []string
	Environment Environment
	StorageDir  string
	HTTPServer  *http.Server
	Logger      *logrus.Logger

	// New field for DNS config
	DNSProvider    string
	DNSCredentials map[string]string
}

// CertificateManager handles certificate issuance and renewal
type CertificateManager struct {
	config     CertificateManagerConfig
	certmagic  *certmagic.Config
	acmeIssuer *certmagic.ACMEIssuer
	logger     *logrus.Logger
}

// NewCertificateManager creates a new certificate manager
func NewCertificateManager(config CertificateManagerConfig) (*CertificateManager, error) {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	// Set up storage
	storage := &certmagic.FileStorage{Path: config.StorageDir}

	// Create ACME config
	certmagicConfig := certmagic.NewDefault()
	certmagicConfig.Storage = storage

	// Configure renewal settings
	certmagicConfig.RenewalWindowRatio = 0.33 // Renew when 1/3 of the time has passed

	// Configure ACME
	acmeIssuer := certmagic.NewACMEIssuer(certmagicConfig, certmagic.ACMEIssuer{
		Email:  config.Email,
		Agreed: true,
	})

	// Set the directory URL based on the environment
	if config.Environment == StagingEnv {
		acmeIssuer.CA = certmagic.LetsEncryptStagingCA
		config.Logger.Info("Using Let's Encrypt staging environment")
	} else {
		acmeIssuer.CA = certmagic.LetsEncryptProductionCA
		config.Logger.Info("Using Let's Encrypt production environment")
	}

	// Configure DNS-01 challenge solver if provider is specified
	if config.DNSProvider != "" {
		config.Logger.Info("Configuring DNS-01 challenge solver")

		// Use our compatibility wrapper to configure the DNS provider
		if err := configureACMEIssuerWithDNS(acmeIssuer, config.DNSProvider, config.DNSCredentials); err != nil {
			return nil, fmt.Errorf("failed to configure DNS provider: %w", err)
		}
	}

	certmagicConfig.Issuers = []certmagic.Issuer{acmeIssuer}

	return &CertificateManager{
		config:     config,
		certmagic:  certmagicConfig,
		acmeIssuer: acmeIssuer,
		logger:     config.Logger,
	}, nil
}

// Start initializes the certificate manager and begins certificate management
func (cm *CertificateManager) Start(ctx context.Context) error {
	cm.logger.Info("Starting certificate manager")

	// If we have a custom HTTP server for challenges
	if cm.config.HTTPServer != nil {
		// Setup HTTP challenge parameters
		// Extract port from HTTPServer.Addr (format: "host:port")
		parts := strings.Split(cm.config.HTTPServer.Addr, ":")
		var port int
		if len(parts) == 2 {
			portStr := parts[1]
			var err error
			port, err = strconv.Atoi(portStr)
			if err != nil {
				return fmt.Errorf("invalid port in HTTP server address: %w", err)
			}
			cm.acmeIssuer.ListenHost = parts[0]
		} else {
			// Default to port 80 if not specified
			port = 80
		}

		// Set the alternative ports for the challenges
		cm.acmeIssuer.AltHTTPPort = port
		cm.acmeIssuer.AltTLSALPNPort = 0 // Disable TLS-ALPN if using HTTP
	}

	// Manage certificates for all the domains
	err := cm.certmagic.ManageSync(ctx, cm.config.Domains)
	if err != nil {
		return fmt.Errorf("failed to manage certificates: %w", err)
	}

	cm.logger.Info("Certificate manager started")
	return nil
}

// GetCertificate returns a certificate for the specified domain
func (cm *CertificateManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return cm.certmagic.GetCertificate(hello)
}

// GetTLSConfig returns a TLS config with configured certificates
func (cm *CertificateManager) GetTLSConfig() *tls.Config {
	return cm.certmagic.TLSConfig()
}

// Status returns information about managed certificates
func (cm *CertificateManager) Status() map[string]interface{} {
	status := make(map[string]interface{})

	certificates := make(map[string]interface{})

	ctx := context.Background()

	for _, domain := range cm.config.Domains {
		// Use context parameter with CacheManagedCertificate
		cert, err := cm.certmagic.CacheManagedCertificate(ctx, domain)
		if err != nil {
			certificates[domain] = map[string]interface{}{
				"error": err.Error(),
			}
			continue
		}

		// The new certmagic API doesn't expose a direct way to check if a certificate
		// is temporary, so we'll infer it based on the certificate's properties
		isTemp := false
		// Temporary certs usually have a very short lifetime (a few days)
		if time.Until(cert.Leaf.NotAfter) < 7*24*time.Hour {
			isTemp = true
		}

		certificates[domain] = map[string]interface{}{
			"issuer":      cert.Leaf.Issuer.CommonName,
			"notBefore":   cert.Leaf.NotBefore,
			"notAfter":    cert.Leaf.NotAfter,
			"dnsNames":    cert.Leaf.DNSNames,
			"isTemporary": isTemp,
		}
	}

	status["certificates"] = certificates
	status["environment"] = cm.config.Environment
	status["email"] = cm.config.Email

	return status
}

// Stop gracefully shuts down the certificate manager
func (cm *CertificateManager) Stop() error {
	cm.logger.Info("Stopping certificate manager")
	return nil
}
