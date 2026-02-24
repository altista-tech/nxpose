package crypto

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

func TestCreateOAuthConfig_Google(t *testing.T) {
	config := CreateOAuthConfig(
		"google",
		"google-client-id",
		"google-client-secret",
		"https://example.com/callback",
		[]string{"openid", "email", "profile"},
	)
	require.NotNil(t, config)

	assert.Equal(t, "google-client-id", config.ClientID)
	assert.Equal(t, "google-client-secret", config.ClientSecret)
	assert.Equal(t, "https://example.com/callback", config.RedirectURL)
	assert.Equal(t, []string{"openid", "email", "profile"}, config.Scopes)
	assert.Equal(t, google.Endpoint, config.Endpoint)
}

func TestCreateOAuthConfig_GitHub(t *testing.T) {
	config := CreateOAuthConfig(
		"github",
		"github-client-id",
		"github-client-secret",
		"https://example.com/callback",
		[]string{"user", "repo"},
	)
	require.NotNil(t, config)

	assert.Equal(t, "github-client-id", config.ClientID)
	assert.Equal(t, "github-client-secret", config.ClientSecret)
	assert.Equal(t, "https://example.com/callback", config.RedirectURL)
	assert.Equal(t, []string{"user", "repo"}, config.Scopes)
	assert.Equal(t, github.Endpoint, config.Endpoint)
}

func TestCreateOAuthConfig_UnsupportedProvider(t *testing.T) {
	config := CreateOAuthConfig(
		"unsupported",
		"id", "secret",
		"https://example.com/callback",
		[]string{"scope"},
	)
	assert.Nil(t, config)
}

func TestCreateOAuthConfig_EmptyProvider(t *testing.T) {
	config := CreateOAuthConfig("", "id", "secret", "url", nil)
	assert.Nil(t, config)
}

func TestGenerateState(t *testing.T) {
	state, err := GenerateState("test-client")
	require.NoError(t, err)
	require.NotEmpty(t, state)

	// State should be hex-encoded 16 bytes = 32 hex characters
	assert.Len(t, state, 32)

	// Should be valid hex
	_, err = hex.DecodeString(state)
	assert.NoError(t, err)
}

func TestGenerateState_UniqueValues(t *testing.T) {
	states := make(map[string]bool)
	for i := 0; i < 100; i++ {
		state, err := GenerateState("client-id")
		require.NoError(t, err)
		assert.False(t, states[state], "state should be unique")
		states[state] = true
	}
}

func TestGenerateAuthURL(t *testing.T) {
	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURL:  "https://example.com/callback",
		Scopes:       []string{"openid"},
		Endpoint:     google.Endpoint,
	}

	url := GenerateAuthURL(config, "test-state-value")

	assert.Contains(t, url, "client_id=test-client-id")
	assert.Contains(t, url, "state=test-state-value")
	assert.Contains(t, url, "access_type=offline")
	assert.Contains(t, url, "response_type=code")
	assert.Contains(t, url, "redirect_uri=")
}

func TestGenerateAuthURL_GitHub(t *testing.T) {
	config := &oauth2.Config{
		ClientID:     "gh-client-id",
		ClientSecret: "gh-secret",
		RedirectURL:  "https://example.com/callback",
		Scopes:       []string{"user"},
		Endpoint:     github.Endpoint,
	}

	url := GenerateAuthURL(config, "my-state")

	assert.Contains(t, url, "github.com")
	assert.Contains(t, url, "client_id=gh-client-id")
	assert.Contains(t, url, "state=my-state")
}

func TestGenerateCertificateForUser_Google(t *testing.T) {
	userInfo := map[string]interface{}{
		"email": "user@example.com",
		"name":  "Test User",
	}

	certPEM, keyPEM, clientID, err := GenerateCertificateForUser(userInfo, "google")
	require.NoError(t, err)
	assert.NotEmpty(t, certPEM)
	assert.NotEmpty(t, keyPEM)
	assert.NotEmpty(t, clientID)

	// Parse cert to verify subject
	block, _ := pem.Decode([]byte(certPEM))
	require.NotNil(t, block)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.Equal(t, "user@example.com", cert.Subject.CommonName)
	assert.Equal(t, []string{"NXpose Client"}, cert.Subject.Organization)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)
}

func TestGenerateCertificateForUser_GitHub(t *testing.T) {
	userInfo := map[string]interface{}{
		"login": "testuser",
		"id":    float64(12345),
	}

	certPEM, keyPEM, clientID, err := GenerateCertificateForUser(userInfo, "github")
	require.NoError(t, err)
	assert.NotEmpty(t, certPEM)
	assert.NotEmpty(t, keyPEM)
	assert.NotEmpty(t, clientID)

	block, _ := pem.Decode([]byte(certPEM))
	require.NotNil(t, block)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.Equal(t, "testuser", cert.Subject.CommonName)
}

func TestGenerateCertificateForUser_GoogleMissingEmail(t *testing.T) {
	userInfo := map[string]interface{}{
		"name": "Test User",
	}

	_, _, _, err := GenerateCertificateForUser(userInfo, "google")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email not found")
}

func TestGenerateCertificateForUser_GitHubMissingLogin(t *testing.T) {
	userInfo := map[string]interface{}{
		"id": float64(12345),
	}

	_, _, _, err := GenerateCertificateForUser(userInfo, "github")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "login not found")
}

func TestGenerateCertificateForUser_UnsupportedProvider(t *testing.T) {
	userInfo := map[string]interface{}{"email": "user@example.com"}

	_, _, _, err := GenerateCertificateForUser(userInfo, "unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}

func TestGenerateCertificateForUser_UniqueClientIDs(t *testing.T) {
	userInfo := map[string]interface{}{"email": "user@example.com"}

	_, _, id1, err := GenerateCertificateForUser(userInfo, "google")
	require.NoError(t, err)

	_, _, id2, err := GenerateCertificateForUser(userInfo, "google")
	require.NoError(t, err)

	assert.NotEqual(t, id1, id2, "each call should produce a unique client ID")
}

func TestGenerateCertificateForUser_ValidKeyPair(t *testing.T) {
	userInfo := map[string]interface{}{"email": "user@example.com"}

	certPEM, keyPEM, _, err := GenerateCertificateForUser(userInfo, "google")
	require.NoError(t, err)

	// Verify the cert and key form a valid TLS pair
	_, err = tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	assert.NoError(t, err)
}
