package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadOrGenerateServerCertificate_WithValidFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	// Generate certs first, then load them via the function
	_, err := generateServerCertificate(log)
	require.NoError(t, err)

	certPath := filepath.Join(tmpDir, ".nxpose", "server.crt")
	keyPath := filepath.Join(tmpDir, ".nxpose", "server.key")

	tlsConfig, err := LoadOrGenerateServerCertificate(certPath, keyPath, log)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)

	assert.Len(t, tlsConfig.Certificates, 1)
	assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
}

func TestLoadOrGenerateServerCertificate_WithMissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	log := logrus.New()

	// Provide paths that don't exist
	tlsConfig, err := LoadOrGenerateServerCertificate(
		filepath.Join(tmpDir, "nonexistent.crt"),
		filepath.Join(tmpDir, "nonexistent.key"),
		log,
	)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)

	// Should have generated new certs
	assert.Len(t, tlsConfig.Certificates, 1)
	assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
}

func TestLoadOrGenerateServerCertificate_WithInvalidFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	log := logrus.New()

	// Create invalid cert files
	certPath := filepath.Join(tmpDir, "bad.crt")
	keyPath := filepath.Join(tmpDir, "bad.key")
	require.NoError(t, os.WriteFile(certPath, []byte("not a cert"), 0644))
	require.NoError(t, os.WriteFile(keyPath, []byte("not a key"), 0600))

	// Should fall through to generating new certs
	tlsConfig, err := LoadOrGenerateServerCertificate(certPath, keyPath, log)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)
	assert.Len(t, tlsConfig.Certificates, 1)
}

func TestLoadOrGenerateServerCertificate_EmptyPaths(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	log := logrus.New()

	// Empty paths should generate new certs
	tlsConfig, err := LoadOrGenerateServerCertificate("", "", log)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)
	assert.Len(t, tlsConfig.Certificates, 1)
	assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
}

func TestLoadOrGenerateServerCertificate_NilLogger(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// nil logger should work (function creates a default)
	tlsConfig, err := LoadOrGenerateServerCertificate("", "", nil)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)
}

func TestLoadOrGenerateServerCertificate_LoadsDefaultLocation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	log := logrus.New()

	// Generate certs at default location
	_, err := generateServerCertificate(log)
	require.NoError(t, err)

	// Call with empty paths - should find certs at default location
	tlsConfig, err := LoadOrGenerateServerCertificate("", "", log)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)
	assert.Len(t, tlsConfig.Certificates, 1)
}

func TestGenerateServerCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	log := logrus.New()

	tlsConfig, err := generateServerCertificate(log)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)

	assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
	require.Len(t, tlsConfig.Certificates, 1)

	// Parse the generated certificate
	x509Cert, err := x509.ParseCertificate(tlsConfig.Certificates[0].Certificate[0])
	require.NoError(t, err)

	// Verify certificate properties
	assert.Equal(t, "nxpose.local", x509Cert.Subject.CommonName)
	assert.Equal(t, []string{"NXpose Tunnel Server"}, x509Cert.Subject.Organization)
	assert.True(t, x509Cert.IsCA)
	assert.True(t, x509Cert.BasicConstraintsValid)
	assert.Contains(t, x509Cert.DNSNames, "localhost")
	assert.Contains(t, x509Cert.DNSNames, "*.nxpose.local")
	assert.Contains(t, x509Cert.DNSNames, "nxpose.local")
	assert.Contains(t, x509Cert.IPAddresses, net.IPv4(127, 0, 0, 1).To4())

	// Verify key usage
	assert.True(t, x509Cert.KeyUsage&x509.KeyUsageCertSign != 0)
	assert.True(t, x509Cert.KeyUsage&x509.KeyUsageKeyEncipherment != 0)
	assert.True(t, x509Cert.KeyUsage&x509.KeyUsageDigitalSignature != 0)
	assert.Contains(t, x509Cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)

	// Verify validity
	validity := x509Cert.NotAfter.Sub(x509Cert.NotBefore)
	assert.InDelta(t, 365*24, validity.Hours(), 1)
}

func TestGenerateServerCertificate_SavesToDisk(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	log := logrus.New()

	_, err := generateServerCertificate(log)
	require.NoError(t, err)

	certPath := filepath.Join(tmpDir, ".nxpose", "server.crt")
	keyPath := filepath.Join(tmpDir, ".nxpose", "server.key")

	assert.FileExists(t, certPath)
	assert.FileExists(t, keyPath)

	// Verify key file permissions
	keyInfo, err := os.Stat(keyPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), keyInfo.Mode().Perm())
}

func TestGenerateDummyClientCertificate(t *testing.T) {
	certPEM, err := GenerateDummyClientCertificate()
	require.NoError(t, err)
	require.NotEmpty(t, certPEM)

	// Decode PEM
	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block)
	assert.Equal(t, "CERTIFICATE", block.Type)

	// Parse certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.Equal(t, "client.nxpose.local", cert.Subject.CommonName)
	assert.Equal(t, []string{"NXpose Tunnel Client"}, cert.Subject.Organization)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)

	// Should be valid for 30 days
	validity := cert.NotAfter.Sub(cert.NotBefore)
	assert.InDelta(t, 30*24, validity.Hours(), 1)
}

func TestSignClientCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	log := logrus.New()

	// Generate server certificate (acts as CA)
	_, err := generateServerCertificate(log)
	require.NoError(t, err)

	// Generate a CSR
	csrPEM := generateTestCSR(t)

	// Sign the CSR
	signedCertPEM, err := SignClientCertificate(csrPEM)
	require.NoError(t, err)
	require.NotEmpty(t, signedCertPEM)

	// Parse the signed certificate
	block, _ := pem.Decode(signedCertPEM)
	require.NotNil(t, block)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// Verify it's a client certificate
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)
	assert.False(t, cert.IsCA)

	// Verify validity (30 days)
	validity := cert.NotAfter.Sub(cert.NotBefore)
	assert.InDelta(t, 30*24, validity.Hours(), 1)

	// Verify issuer is the server cert
	assert.Equal(t, "nxpose.local", cert.Issuer.CommonName)
}

func TestSignClientCertificate_InvalidCSR(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	log := logrus.New()

	_, err := generateServerCertificate(log)
	require.NoError(t, err)

	// Try with invalid PEM
	_, err = SignClientCertificate([]byte("not a csr"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode CSR PEM")
}

func TestSignClientCertificate_NoServerCert(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	csrPEM := generateTestCSR(t)

	// No server cert exists
	_, err := SignClientCertificate(csrPEM)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server certificate or key not found")
}

// generateTestRSAKey generates a 2048-bit RSA key for testing
func generateTestRSAKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// generateTestCSR creates a test CSR for use in signing tests
func generateTestCSR(t *testing.T) []byte {
	t.Helper()

	key, err := generateTestRSAKey()
	require.NoError(t, err)

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   "test-client",
			Organization: []string{"Test"},
		},
		DNSNames: []string{"test.local"},
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	})
}
