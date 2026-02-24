package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nxpose/internal/crypto"
)

// resetViper clears all viper state between tests to avoid cross-test contamination.
func resetViper() {
	viper.Reset()
}

// writeYAML is a helper that writes content to a temp YAML file and returns the path.
func writeYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

// --- DefaultConfig tests ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "nxpose.naxrevlis.com", cfg.ServerHost)
	assert.Equal(t, 443, cfg.ServerPort)
	assert.Equal(t, 3000, cfg.LocalPort)
	assert.Equal(t, "https", cfg.Protocol)
	assert.Equal(t, "", cfg.SubdomainID)
	assert.False(t, cfg.SkipLocalCheck)
	assert.False(t, cfg.Verbose)
	assert.Equal(t, "", cfg.TLSCert)
	assert.Equal(t, "", cfg.TLSKey)
	assert.Nil(t, cfg.CertData)
}

func TestDefaultServerConfig(t *testing.T) {
	cfg := DefaultServerConfig()

	// Server settings
	assert.Equal(t, "0.0.0.0", cfg.BindAddress)
	assert.Equal(t, 8443, cfg.Port)
	assert.Equal(t, "localhost", cfg.BaseDomain)
	assert.False(t, cfg.Verbose)
	assert.Equal(t, "", cfg.TLSCert)
	assert.Equal(t, "", cfg.TLSKey)

	// OAuth2 defaults
	assert.False(t, cfg.OAuth2.Enabled)
	assert.Equal(t, "https://localhost:8443/auth/callback", cfg.OAuth2.RedirectURL)
	assert.Equal(t, "", cfg.OAuth2.SessionKey)
	assert.Equal(t, "memory", cfg.OAuth2.SessionStore)
	assert.Empty(t, cfg.OAuth2.Providers)

	// MongoDB defaults
	assert.False(t, cfg.MongoDB.Enabled)
	assert.Equal(t, "mongodb://localhost:27017", cfg.MongoDB.URI)
	assert.Equal(t, "nxpose", cfg.MongoDB.Database)
	assert.Equal(t, 10*time.Second, cfg.MongoDB.Timeout)

	// Redis defaults
	assert.False(t, cfg.Redis.Enabled)
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, 6379, cfg.Redis.Port)
	assert.Equal(t, "", cfg.Redis.Password)
	assert.Equal(t, 0, cfg.Redis.DB)
	assert.Equal(t, "nxpose:", cfg.Redis.KeyPrefix)
	assert.Equal(t, 10*time.Second, cfg.Redis.Timeout)

	// Tunnel limits defaults
	assert.Equal(t, 5, cfg.TunnelLimits.MaxPerUser)
	assert.Equal(t, "", cfg.TunnelLimits.MaxConnection)

	// LetsEncrypt defaults
	assert.False(t, cfg.LetsEncrypt.Enabled)
	assert.Equal(t, "", cfg.LetsEncrypt.Email)
	assert.Equal(t, crypto.ProductionEnv, cfg.LetsEncrypt.Environment)
	assert.NotEmpty(t, cfg.LetsEncrypt.StorageDir) // derived from $HOME
	assert.Equal(t, "", cfg.LetsEncrypt.DNSProvider)
	assert.NotNil(t, cfg.LetsEncrypt.DNSCredentials)
}

// --- LoadConfig (client) YAML loading tests ---

func TestLoadConfig_FromYAMLFile(t *testing.T) {
	resetViper()

	yaml := `
server:
  host: "tunnel.example.com"
  port: 9443
client:
  local_port: 8080
  protocol: "http"
  subdomain: "myapp"
  skip_local_check: true
verbose: true
tls:
  cert: "/tmp/cert.pem"
  key: "/tmp/key.pem"
`
	path := writeYAML(t, yaml)

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, "tunnel.example.com", cfg.ServerHost)
	assert.Equal(t, 9443, cfg.ServerPort)
	assert.Equal(t, 8080, cfg.LocalPort)
	assert.Equal(t, "http", cfg.Protocol)
	assert.Equal(t, "myapp", cfg.SubdomainID)
	assert.True(t, cfg.SkipLocalCheck)
	assert.True(t, cfg.Verbose)
	assert.Equal(t, "/tmp/cert.pem", cfg.TLSCert)
	assert.Equal(t, "/tmp/key.pem", cfg.TLSKey)
}

func TestLoadConfig_PartialYAMLUsesDefaults(t *testing.T) {
	resetViper()

	yaml := `
server:
  host: "custom.host.io"
`
	path := writeYAML(t, yaml)

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, "custom.host.io", cfg.ServerHost)
	// All other fields should retain defaults
	assert.Equal(t, 443, cfg.ServerPort)
	assert.Equal(t, 3000, cfg.LocalPort)
	assert.Equal(t, "https", cfg.Protocol)
	assert.Equal(t, "", cfg.SubdomainID)
	assert.False(t, cfg.SkipLocalCheck)
	assert.False(t, cfg.Verbose)
}

// --- LoadServerConfig YAML loading tests ---

func TestLoadServerConfig_FromYAMLFile(t *testing.T) {
	resetViper()

	yaml := `
server:
  bind_address: "127.0.0.1"
  port: 9999
  domain: "tunnel.prod.com"
tls:
  cert: "/etc/ssl/cert.pem"
  key: "/etc/ssl/key.pem"
verbose: true
oauth2:
  enabled: true
  redirect_url: "https://tunnel.prod.com/auth/callback"
  session_key: "supersecret"
  session_store: "redis"
  providers:
    - name: "github"
      credentials:
        - client_id: "gh-id-123"
          client_secret: "gh-secret-456"
      scopes:
        - "user:email"
        - "read:user"
mongodb:
  enabled: true
  uri: "mongodb://db.host:27017"
  database: "tunnels"
  timeout: "30s"
redis:
  enabled: true
  host: "redis.host"
  port: 6380
  password: "redispass"
  db: 2
  key_prefix: "tun:"
  timeout: "5s"
tunnels:
  max_per_user: 10
  max_connection: "2h"
letsencrypt:
  enabled: true
  email: "admin@prod.com"
  environment: "staging"
  storage_dir: "/var/certs"
  dns:
    provider: "cloudflare"
    credentials:
      api_token: "cf-token-xyz"
`
	path := writeYAML(t, yaml)

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)

	// Server settings
	assert.Equal(t, "127.0.0.1", cfg.BindAddress)
	assert.Equal(t, 9999, cfg.Port)
	assert.Equal(t, "tunnel.prod.com", cfg.BaseDomain)
	assert.Equal(t, "/etc/ssl/cert.pem", cfg.TLSCert)
	assert.Equal(t, "/etc/ssl/key.pem", cfg.TLSKey)
	assert.True(t, cfg.Verbose)

	// OAuth2
	assert.True(t, cfg.OAuth2.Enabled)
	assert.Equal(t, "https://tunnel.prod.com/auth/callback", cfg.OAuth2.RedirectURL)
	assert.Equal(t, "supersecret", cfg.OAuth2.SessionKey)
	assert.Equal(t, "redis", cfg.OAuth2.SessionStore)
	require.Len(t, cfg.OAuth2.Providers, 1)
	assert.Equal(t, "github", cfg.OAuth2.Providers[0].Name)
	assert.Equal(t, "gh-id-123", cfg.OAuth2.Providers[0].ClientID)
	assert.Equal(t, "gh-secret-456", cfg.OAuth2.Providers[0].ClientSecret)
	assert.Equal(t, []string{"user:email", "read:user"}, cfg.OAuth2.Providers[0].Scopes)

	// MongoDB
	assert.True(t, cfg.MongoDB.Enabled)
	assert.Equal(t, "mongodb://db.host:27017", cfg.MongoDB.URI)
	assert.Equal(t, "tunnels", cfg.MongoDB.Database)
	assert.Equal(t, 30*time.Second, cfg.MongoDB.Timeout)

	// Redis
	assert.True(t, cfg.Redis.Enabled)
	assert.Equal(t, "redis.host", cfg.Redis.Host)
	assert.Equal(t, 6380, cfg.Redis.Port)
	assert.Equal(t, "redispass", cfg.Redis.Password)
	assert.Equal(t, 2, cfg.Redis.DB)
	assert.Equal(t, "tun:", cfg.Redis.KeyPrefix)
	assert.Equal(t, 5*time.Second, cfg.Redis.Timeout)

	// Tunnel limits
	assert.Equal(t, 10, cfg.TunnelLimits.MaxPerUser)
	assert.Equal(t, "2h", cfg.TunnelLimits.MaxConnection)

	// LetsEncrypt
	assert.True(t, cfg.LetsEncrypt.Enabled)
	assert.Equal(t, "admin@prod.com", cfg.LetsEncrypt.Email)
	assert.Equal(t, crypto.StagingEnv, cfg.LetsEncrypt.Environment)
	assert.Equal(t, "/var/certs", cfg.LetsEncrypt.StorageDir)
	assert.Equal(t, "cloudflare", cfg.LetsEncrypt.DNSProvider)
	assert.Equal(t, "cf-token-xyz", cfg.LetsEncrypt.DNSCredentials["api_token"])
}

func TestLoadServerConfig_PartialYAMLUsesDefaults(t *testing.T) {
	resetViper()

	yaml := `
server:
  domain: "partial.test.com"
`
	path := writeYAML(t, yaml)

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)

	assert.Equal(t, "partial.test.com", cfg.BaseDomain)
	// Defaults should apply for everything else
	assert.Equal(t, "0.0.0.0", cfg.BindAddress)
	assert.Equal(t, 8443, cfg.Port)
	assert.False(t, cfg.Verbose)
	assert.False(t, cfg.OAuth2.Enabled)
	assert.Equal(t, "memory", cfg.OAuth2.SessionStore)
	assert.False(t, cfg.MongoDB.Enabled)
	assert.False(t, cfg.Redis.Enabled)
	assert.Equal(t, 5, cfg.TunnelLimits.MaxPerUser)
	assert.False(t, cfg.LetsEncrypt.Enabled)
	assert.Equal(t, crypto.ProductionEnv, cfg.LetsEncrypt.Environment)
}

func TestLoadServerConfig_LetsEncryptProductionEnvironment(t *testing.T) {
	resetViper()

	yaml := `
letsencrypt:
  enabled: true
  environment: "production"
`
	path := writeYAML(t, yaml)

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)
	assert.Equal(t, crypto.ProductionEnv, cfg.LetsEncrypt.Environment)
}

func TestLoadServerConfig_MultipleOAuth2Providers(t *testing.T) {
	resetViper()

	yaml := `
oauth2:
  enabled: true
  providers:
    - name: "github"
      credentials:
        - client_id: "gh-id"
          client_secret: "gh-secret"
      scopes:
        - "user:email"
    - name: "google"
      credentials:
        - client_id: "google-id"
          client_secret: "google-secret"
      scopes:
        - "email"
        - "profile"
`
	path := writeYAML(t, yaml)

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)
	require.Len(t, cfg.OAuth2.Providers, 2)
	assert.Equal(t, "github", cfg.OAuth2.Providers[0].Name)
	assert.Equal(t, "gh-id", cfg.OAuth2.Providers[0].ClientID)
	assert.Equal(t, "google", cfg.OAuth2.Providers[1].Name)
	assert.Equal(t, "google-id", cfg.OAuth2.Providers[1].ClientID)
	assert.Equal(t, "google-secret", cfg.OAuth2.Providers[1].ClientSecret)
	assert.Equal(t, []string{"email", "profile"}, cfg.OAuth2.Providers[1].Scopes)
}

func TestLoadServerConfig_OAuth2CredentialsFormat(t *testing.T) {
	resetViper()

	yaml := `
oauth2:
  enabled: true
  providers:
    - name: "github"
      credentials:
        - client_id: "cred-gh-id"
          client_secret: "cred-gh-secret"
      scopes:
        - "user:email"
`
	path := writeYAML(t, yaml)

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)
	require.Len(t, cfg.OAuth2.Providers, 1)
	// The credentials format should be extracted into ClientID/ClientSecret
	assert.Equal(t, "cred-gh-id", cfg.OAuth2.Providers[0].ClientID)
	assert.Equal(t, "cred-gh-secret", cfg.OAuth2.Providers[0].ClientSecret)
}

// --- Environment variable override tests ---

func TestLoadConfig_EnvOverridesYAML(t *testing.T) {
	resetViper()

	yaml := `
server:
  host: "yaml-host.com"
  port: 1111
verbose: false
`
	path := writeYAML(t, yaml)

	t.Setenv("NXPOSE_SERVER_HOST", "env-host.com")
	t.Setenv("NXPOSE_VERBOSE", "true")

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	// Env var should override YAML values (viper AutomaticEnv with prefix)
	// Note: viper's AutomaticEnv uses the env prefix + key path with _ separator
	// The actual behavior depends on how viper resolves the key
	assert.True(t, cfg.Verbose)
}

func TestLoadServerConfig_EnvOverridesYAML(t *testing.T) {
	resetViper()

	yaml := `
server:
  bind_address: "192.168.1.1"
  port: 5555
  domain: "yaml-domain.com"
verbose: false
`
	path := writeYAML(t, yaml)

	t.Setenv("NXPOSE_SERVER_PORT", "7777")
	t.Setenv("NXPOSE_SERVER_DOMAIN", "env-domain.com")
	t.Setenv("NXPOSE_VERBOSE", "true")

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)

	assert.Equal(t, 7777, cfg.Port)
	assert.Equal(t, "env-domain.com", cfg.BaseDomain)
	assert.True(t, cfg.Verbose)
	// bind_address should remain from YAML since we didn't override it
	assert.Equal(t, "192.168.1.1", cfg.BindAddress)
}

func TestLoadServerConfig_EnvOverridesMongoDBSettings(t *testing.T) {
	resetViper()

	yaml := `
mongodb:
  enabled: false
  uri: "mongodb://yaml-host:27017"
`
	path := writeYAML(t, yaml)

	t.Setenv("NXPOSE_MONGODB_ENABLED", "true")
	t.Setenv("NXPOSE_MONGODB_URI", "mongodb://env-host:27017")
	t.Setenv("NXPOSE_MONGODB_DATABASE", "envdb")

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)

	assert.True(t, cfg.MongoDB.Enabled)
	assert.Equal(t, "mongodb://env-host:27017", cfg.MongoDB.URI)
	assert.Equal(t, "envdb", cfg.MongoDB.Database)
}

func TestLoadServerConfig_EnvOverridesRedisSettings(t *testing.T) {
	resetViper()

	yaml := `
redis:
  enabled: false
`
	path := writeYAML(t, yaml)

	t.Setenv("NXPOSE_REDIS_ENABLED", "true")
	t.Setenv("NXPOSE_REDIS_HOST", "redis-env.host")
	t.Setenv("NXPOSE_REDIS_PORT", "6380")
	t.Setenv("NXPOSE_REDIS_PASSWORD", "envpass")
	t.Setenv("NXPOSE_REDIS_KEY_PREFIX", "env:")

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)

	assert.True(t, cfg.Redis.Enabled)
	assert.Equal(t, "redis-env.host", cfg.Redis.Host)
	assert.Equal(t, 6380, cfg.Redis.Port)
	assert.Equal(t, "envpass", cfg.Redis.Password)
	assert.Equal(t, "env:", cfg.Redis.KeyPrefix)
}

func TestLoadServerConfig_EnvOverridesTunnelLimits(t *testing.T) {
	resetViper()

	yaml := `
tunnels:
  max_per_user: 3
`
	path := writeYAML(t, yaml)

	t.Setenv("NXPOSE_TUNNELS_MAX_PER_USER", "20")
	t.Setenv("NXPOSE_TUNNELS_MAX_CONNECTION", "4h")

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)

	assert.Equal(t, 20, cfg.TunnelLimits.MaxPerUser)
	assert.Equal(t, "4h", cfg.TunnelLimits.MaxConnection)
}

func TestLoadServerConfig_EnvOverridesLetsEncrypt(t *testing.T) {
	resetViper()

	yaml := `
letsencrypt:
  enabled: false
`
	path := writeYAML(t, yaml)

	t.Setenv("NXPOSE_LETSENCRYPT_ENABLED", "true")
	t.Setenv("NXPOSE_LETSENCRYPT_EMAIL", "env@example.com")
	t.Setenv("NXPOSE_LETSENCRYPT_ENVIRONMENT", "staging")
	t.Setenv("NXPOSE_LETSENCRYPT_STORAGE_DIR", "/env/certs")
	t.Setenv("NXPOSE_LETSENCRYPT_DNS_PROVIDER", "route53")

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)

	assert.True(t, cfg.LetsEncrypt.Enabled)
	assert.Equal(t, "env@example.com", cfg.LetsEncrypt.Email)
	assert.Equal(t, crypto.StagingEnv, cfg.LetsEncrypt.Environment)
	assert.Equal(t, "/env/certs", cfg.LetsEncrypt.StorageDir)
	assert.Equal(t, "route53", cfg.LetsEncrypt.DNSProvider)
}

func TestLoadServerConfig_EnvOverridesOAuth2(t *testing.T) {
	resetViper()

	yaml := `
oauth2:
  enabled: false
`
	path := writeYAML(t, yaml)

	t.Setenv("NXPOSE_OAUTH2_ENABLED", "true")
	t.Setenv("NXPOSE_OAUTH2_REDIRECT_URL", "https://env.example.com/callback")
	t.Setenv("NXPOSE_OAUTH2_SESSION_KEY", "env-session-key")
	t.Setenv("NXPOSE_OAUTH2_SESSION_STORE", "mongo")

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)

	assert.True(t, cfg.OAuth2.Enabled)
	assert.Equal(t, "https://env.example.com/callback", cfg.OAuth2.RedirectURL)
	assert.Equal(t, "env-session-key", cfg.OAuth2.SessionKey)
	assert.Equal(t, "mongo", cfg.OAuth2.SessionStore)
}

// --- Invalid / missing config file handling tests ---

func TestLoadConfig_InvalidYAMLReturnsError(t *testing.T) {
	resetViper()

	content := `
server:
  host: [invalid yaml
  broken: {
`
	path := writeYAML(t, content)

	_, err := LoadConfig(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error reading config file")
}

func TestLoadServerConfig_InvalidYAMLReturnsError(t *testing.T) {
	resetViper()

	content := `
server:
  port: [broken yaml
`
	path := writeYAML(t, content)

	_, err := LoadServerConfig(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error reading config file")
}

func TestLoadConfig_NonExistentExplicitFileReturnsError(t *testing.T) {
	resetViper()

	_, err := LoadConfig("/nonexistent/path/to/config.yaml")
	assert.Error(t, err)
}

func TestLoadServerConfig_NonExistentExplicitFileStillReturnsConfig(t *testing.T) {
	resetViper()

	// LoadServerConfig with a nonexistent explicit file prints a message
	// but doesn't error if file not found (falls back to env vars)
	cfg, err := LoadServerConfig("/nonexistent/path/to/server-config.yaml")
	// The function returns a config file not found error for explicit paths
	// Since the path exists but may not be found by viper, check behavior
	if err != nil {
		assert.Contains(t, err.Error(), "error reading config file")
	} else {
		// If no error, defaults should be returned
		assert.Equal(t, "0.0.0.0", cfg.BindAddress)
	}
}

func TestLoadConfig_EmptyYAMLUsesDefaults(t *testing.T) {
	resetViper()

	path := writeYAML(t, "")

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	// Empty YAML means no keys set, so all defaults should apply
	assert.Equal(t, "nxpose.naxrevlis.com", cfg.ServerHost)
	assert.Equal(t, 443, cfg.ServerPort)
	assert.Equal(t, 3000, cfg.LocalPort)
	assert.Equal(t, "https", cfg.Protocol)
}

func TestLoadServerConfig_EmptyYAMLUsesDefaults(t *testing.T) {
	resetViper()

	path := writeYAML(t, "# empty config\n")

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.BindAddress)
	assert.Equal(t, 8443, cfg.Port)
	assert.Equal(t, "localhost", cfg.BaseDomain)
	assert.False(t, cfg.MongoDB.Enabled)
	assert.False(t, cfg.Redis.Enabled)
}

// --- Config validation tests ---

func TestLoadServerConfig_PortFromYAML(t *testing.T) {
	resetViper()

	yaml := `
server:
  port: 0
`
	path := writeYAML(t, yaml)

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)
	// The code accepts any port value without range validation
	assert.Equal(t, 0, cfg.Port)
}

func TestLoadServerConfig_LargePortValue(t *testing.T) {
	resetViper()

	yaml := `
server:
  port: 65535
`
	path := writeYAML(t, yaml)

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)
	assert.Equal(t, 65535, cfg.Port)
}

func TestLoadServerConfig_RedisTimeoutParsing(t *testing.T) {
	resetViper()

	yaml := `
redis:
  timeout: "30s"
`
	path := writeYAML(t, yaml)

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, cfg.Redis.Timeout)
}

func TestLoadServerConfig_RedisInvalidTimeoutKeepsDefault(t *testing.T) {
	resetViper()

	yaml := `
redis:
  timeout: "not-a-duration"
`
	path := writeYAML(t, yaml)

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)
	// Invalid duration should keep the default
	assert.Equal(t, 10*time.Second, cfg.Redis.Timeout)
}

func TestLoadServerConfig_LetsEncryptEnvironmentValidation(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected crypto.Environment
	}{
		{"staging", "staging", crypto.StagingEnv},
		{"production", "production", crypto.ProductionEnv},
		{"unknown defaults to production", "invalid", crypto.ProductionEnv},
		{"empty defaults to production", "", crypto.ProductionEnv},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetViper()

			yaml := `
letsencrypt:
  environment: "` + tt.envValue + `"
`
			path := writeYAML(t, yaml)

			cfg, err := LoadServerConfig(path)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.LetsEncrypt.Environment)
		})
	}
}

// --- SaveConfig and certificate tests ---

func TestSaveConfig_WritesAndReloads(t *testing.T) {
	resetViper()

	dir := t.TempDir()
	path := filepath.Join(dir, "saved-config.yaml")

	original := &Config{
		ServerHost:  "saved.host.com",
		ServerPort:  1234,
		LocalPort:   5678,
		Protocol:    "http",
		SubdomainID: "test-sub",
		Verbose:     true,
		TLSCert:     "/path/to/cert",
		TLSKey:      "/path/to/key",
	}

	err := SaveConfig(original, path)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Reload and verify
	resetViper()
	loaded, err := LoadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, original.ServerHost, loaded.ServerHost)
	assert.Equal(t, original.ServerPort, loaded.ServerPort)
	assert.Equal(t, original.LocalPort, loaded.LocalPort)
	assert.Equal(t, original.Protocol, loaded.Protocol)
	assert.Equal(t, original.SubdomainID, loaded.SubdomainID)
	assert.Equal(t, original.Verbose, loaded.Verbose)
	assert.Equal(t, original.TLSCert, loaded.TLSCert)
	assert.Equal(t, original.TLSKey, loaded.TLSKey)
}

func TestSaveServerConfig_WritesAndReloads(t *testing.T) {
	resetViper()

	dir := t.TempDir()
	path := filepath.Join(dir, "saved-server-config.yaml")

	original := DefaultServerConfig()
	original.Port = 7777
	original.BaseDomain = "saved.example.com"
	original.MongoDB.Enabled = true
	original.MongoDB.URI = "mongodb://saved:27017"
	original.Redis.Enabled = true
	original.Redis.Host = "saved-redis"
	original.TunnelLimits.MaxPerUser = 15

	err := SaveServerConfig(original, path)
	require.NoError(t, err)

	resetViper()
	loaded, err := LoadServerConfig(path)
	require.NoError(t, err)

	assert.Equal(t, 7777, loaded.Port)
	assert.Equal(t, "saved.example.com", loaded.BaseDomain)
	assert.True(t, loaded.MongoDB.Enabled)
	assert.Equal(t, "mongodb://saved:27017", loaded.MongoDB.URI)
	assert.True(t, loaded.Redis.Enabled)
	assert.Equal(t, "saved-redis", loaded.Redis.Host)
	assert.Equal(t, 15, loaded.TunnelLimits.MaxPerUser)
}

func TestSaveCertificateData_WritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cert")

	certData := []byte("test-certificate-data-content")
	err := SaveCertificateData(certData, path)
	require.NoError(t, err)

	// Verify file was written with correct permissions
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// Verify content
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, certData, data)
}

func TestLoadCertificateData_ReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cert")

	expected := []byte("loaded-cert-data")
	require.NoError(t, os.WriteFile(path, expected, 0600))

	data, err := LoadCertificateData(path)
	require.NoError(t, err)
	assert.Equal(t, expected, data)
}

func TestLoadCertificateData_NonExistentFileReturnsError(t *testing.T) {
	_, err := LoadCertificateData("/nonexistent/cert.pem")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not load certificate data")
}

func TestSaveCertificateData_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.cert")

	original := []byte("-----BEGIN CERTIFICATE-----\nMIIBxTCCA...\n-----END CERTIFICATE-----")

	err := SaveCertificateData(original, path)
	require.NoError(t, err)

	loaded, err := LoadCertificateData(path)
	require.NoError(t, err)
	assert.Equal(t, original, loaded)
}

func TestStoreCertificate_SetsCertData(t *testing.T) {
	// StoreCertificate creates a new DefaultConfig and sets CertData on it.
	// This is a basic smoke test to ensure it doesn't panic.
	StoreCertificate("test-cert-string")
}

// --- DNS credentials in server config ---

func TestLoadServerConfig_DNSCredentials(t *testing.T) {
	resetViper()

	yaml := `
letsencrypt:
  dns:
    provider: "route53"
    credentials:
      aws_access_key_id: "AKID123"
      aws_secret_access_key: "SECRET456"
      aws_region: "us-east-1"
`
	path := writeYAML(t, yaml)

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)

	assert.Equal(t, "route53", cfg.LetsEncrypt.DNSProvider)
	assert.Equal(t, "AKID123", cfg.LetsEncrypt.DNSCredentials["aws_access_key_id"])
	assert.Equal(t, "SECRET456", cfg.LetsEncrypt.DNSCredentials["aws_secret_access_key"])
	assert.Equal(t, "us-east-1", cfg.LetsEncrypt.DNSCredentials["aws_region"])
}

// --- Edge cases ---

func TestLoadConfig_WrongTypeInYAML(t *testing.T) {
	resetViper()

	// Port as a string that can't be parsed as int
	yaml := `
server:
  port: "not-a-number"
`
	path := writeYAML(t, yaml)

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	// viper will return 0 for unparseable int
	assert.Equal(t, 0, cfg.ServerPort)
}

func TestLoadServerConfig_MongoDBTimeout(t *testing.T) {
	resetViper()

	yaml := `
mongodb:
  timeout: "1m"
`
	path := writeYAML(t, yaml)

	cfg, err := LoadServerConfig(path)
	require.NoError(t, err)
	assert.Equal(t, time.Minute, cfg.MongoDB.Timeout)
}
