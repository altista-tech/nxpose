package crypto

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	// Use temp dir to avoid writing to real ~/.nxpose
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	tlsConfig, err := GenerateSelfSignedCert()
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)

	// Verify certificate bytes are valid PEM
	certBlock, _ := pem.Decode(tlsConfig.Certificate)
	require.NotNil(t, certBlock, "certificate should be valid PEM")
	assert.Equal(t, "CERTIFICATE", certBlock.Type)

	// Verify private key bytes are valid PEM
	keyBlock, _ := pem.Decode(tlsConfig.PrivateKey)
	require.NotNil(t, keyBlock, "private key should be valid PEM")
	assert.Equal(t, "RSA PRIVATE KEY", keyBlock.Type)

	// Parse and validate the certificate
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	require.NoError(t, err)

	assert.Equal(t, "localhost", cert.Subject.CommonName)
	assert.Equal(t, []string{"NXpose Tunnel Client"}, cert.Subject.Organization)
	assert.Contains(t, cert.DNSNames, "localhost")
	assert.Contains(t, cert.IPAddresses, net.IPv4(127, 0, 0, 1).To4())

	// Verify key usage
	assert.True(t, cert.KeyUsage&x509.KeyUsageKeyEncipherment != 0)
	assert.True(t, cert.KeyUsage&x509.KeyUsageDigitalSignature != 0)

	// Verify extended key usage
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)

	// Verify validity period (approximately 1 year)
	validity := cert.NotAfter.Sub(cert.NotBefore)
	assert.InDelta(t, 365*24, validity.Hours(), 1)

	// Verify TLS config is usable
	require.NotNil(t, tlsConfig.TLSClientConf)
	assert.Len(t, tlsConfig.TLSClientConf.Certificates, 1)
	assert.True(t, tlsConfig.TLSClientConf.InsecureSkipVerify)
}

func TestGenerateSelfSignedCert_SavesToDisk(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	_, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	// Check files were saved
	certPath := filepath.Join(tmpDir, ".nxpose", "client.crt")
	keyPath := filepath.Join(tmpDir, ".nxpose", "client.key")

	assert.FileExists(t, certPath)
	assert.FileExists(t, keyPath)

	// Verify key file permissions (0600)
	keyInfo, err := os.Stat(keyPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), keyInfo.Mode().Perm())
}

func TestGenerateSelfSignedCert_UniqueSerialNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tlsConfig1, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	tlsConfig2, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	// Parse both certificates
	block1, _ := pem.Decode(tlsConfig1.Certificate)
	cert1, err := x509.ParseCertificate(block1.Bytes)
	require.NoError(t, err)

	block2, _ := pem.Decode(tlsConfig2.Certificate)
	cert2, err := x509.ParseCertificate(block2.Bytes)
	require.NoError(t, err)

	// Serial numbers should differ
	assert.NotEqual(t, cert1.SerialNumber, cert2.SerialNumber)
}

func TestGenerateSelfSignedCert_ValidKeyPair(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tlsConfig, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	// Verify the cert and key form a valid pair by loading them
	_, err = tls.X509KeyPair(tlsConfig.Certificate, tlsConfig.PrivateKey)
	assert.NoError(t, err)
}

func TestCreateTLSConfigFromFiles_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Generate a cert to get valid PEM bytes
	tlsConfig, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	conf, err := createTLSConfigFromFiles(tlsConfig.Certificate, tlsConfig.PrivateKey)
	require.NoError(t, err)
	require.NotNil(t, conf)

	assert.Len(t, conf.Certificates, 1)
	assert.True(t, conf.InsecureSkipVerify)
}

func TestCreateTLSConfigFromFiles_InvalidCert(t *testing.T) {
	conf, err := createTLSConfigFromFiles([]byte("not a cert"), []byte("not a key"))
	assert.Error(t, err)
	assert.Nil(t, conf)
}

func TestCreateTLSConfigFromFiles_MismatchedKeyPair(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Generate two separate certs
	config1, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	config2, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	// Try to create config with cert from one and key from another
	conf, err := createTLSConfigFromFiles(config1.Certificate, config2.PrivateKey)
	assert.Error(t, err)
	assert.Nil(t, conf)
}

func TestCreateTLSConfig_LoadsExistingCerts(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// First generate and save certs
	original, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	// Now createTLSConfig should load the saved certs
	loaded, err := createTLSConfig()
	require.NoError(t, err)
	require.NotNil(t, loaded)

	// The loaded cert bytes should match what was saved
	assert.Equal(t, original.Certificate, loaded.Certificate)
	assert.Equal(t, original.PrivateKey, loaded.PrivateKey)
}

func TestCreateTLSConfig_GeneratesNewWhenNoneExist(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// No existing certs - should generate new ones
	tlsConfig, err := createTLSConfig()
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)
	assert.NotEmpty(t, tlsConfig.Certificate)
	assert.NotEmpty(t, tlsConfig.PrivateKey)
	assert.NotNil(t, tlsConfig.TLSClientConf)
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file
	testFile := filepath.Join(tmpDir, "testfile")
	err := os.WriteFile(testFile, []byte("data"), 0644)
	require.NoError(t, err)

	assert.True(t, fileExists(testFile))
	assert.False(t, fileExists(filepath.Join(tmpDir, "nonexistent")))
}
