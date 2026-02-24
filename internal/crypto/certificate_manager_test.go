package crypto

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCertificateManager_DefaultStorageDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cm, err := NewCertificateManager(CertificateManagerConfig{
		Email:       "test@example.com",
		Domains:     []string{"example.com"},
		Environment: StagingEnv,
	})
	require.NoError(t, err)
	require.NotNil(t, cm)

	expectedDir := filepath.Join(tmpDir, ".nxpose", "certificates")
	assert.Equal(t, expectedDir, cm.config.StorageDir)
	assert.DirExists(t, expectedDir)
}

func TestNewCertificateManager_CustomStorageDir(t *testing.T) {
	tmpDir := t.TempDir()
	storageDir := filepath.Join(tmpDir, "custom-certs")

	cm, err := NewCertificateManager(CertificateManagerConfig{
		Email:       "test@example.com",
		Domains:     []string{"example.com"},
		Environment: StagingEnv,
		StorageDir:  storageDir,
	})
	require.NoError(t, err)
	require.NotNil(t, cm)

	assert.Equal(t, storageDir, cm.config.StorageDir)
	assert.DirExists(t, storageDir)
}

func TestNewCertificateManager_StagingEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cm, err := NewCertificateManager(CertificateManagerConfig{
		Email:       "test@example.com",
		Domains:     []string{"example.com"},
		Environment: StagingEnv,
	})
	require.NoError(t, err)
	assert.Equal(t, StagingEnv, cm.config.Environment)
}

func TestNewCertificateManager_ProductionEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cm, err := NewCertificateManager(CertificateManagerConfig{
		Email:       "test@example.com",
		Domains:     []string{"example.com"},
		Environment: ProductionEnv,
	})
	require.NoError(t, err)
	assert.Equal(t, ProductionEnv, cm.config.Environment)
}

func TestNewCertificateManager_NilLogger(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cm, err := NewCertificateManager(CertificateManagerConfig{
		Email:       "test@example.com",
		Domains:     []string{"example.com"},
		Environment: StagingEnv,
		Logger:      nil,
	})
	require.NoError(t, err)
	assert.NotNil(t, cm.logger, "should create a default logger")
}

func TestNewCertificateManager_CustomLogger(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	log := logrus.New()

	cm, err := NewCertificateManager(CertificateManagerConfig{
		Email:       "test@example.com",
		Domains:     []string{"example.com"},
		Environment: StagingEnv,
		Logger:      log,
	})
	require.NoError(t, err)
	assert.Equal(t, log, cm.logger)
}

func TestNewCertificateManager_WithDNSProvider(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cm, err := NewCertificateManager(CertificateManagerConfig{
		Email:       "test@example.com",
		Domains:     []string{"example.com"},
		Environment: StagingEnv,
		DNSProvider: "cloudflare",
		DNSCredentials: map[string]string{
			"api_token": "test-token",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, cm)
	assert.Equal(t, "cloudflare", cm.config.DNSProvider)
}

func TestNewCertificateManager_InvalidStorageDir(t *testing.T) {
	// Use a path that can't be created (file as parent)
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blockfile")
	require.NoError(t, os.WriteFile(blockingFile, []byte("data"), 0644))

	_, err := NewCertificateManager(CertificateManagerConfig{
		Email:       "test@example.com",
		Domains:     []string{"example.com"},
		Environment: StagingEnv,
		StorageDir:  filepath.Join(blockingFile, "subdir"),
	})
	assert.Error(t, err)
}

func TestCertificateManager_Stop(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cm, err := NewCertificateManager(CertificateManagerConfig{
		Email:       "test@example.com",
		Domains:     []string{"example.com"},
		Environment: StagingEnv,
	})
	require.NoError(t, err)

	err = cm.Stop()
	assert.NoError(t, err)
}

func TestEnvironmentConstants(t *testing.T) {
	assert.Equal(t, Environment("production"), ProductionEnv)
	assert.Equal(t, Environment("staging"), StagingEnv)
}
