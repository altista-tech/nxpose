package crypto

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// OAuthConfig stores configuration for OAuth2 providers
type OAuthConfig struct {
	// OAuth configuration
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	Provider     string // "google", "github"
}

// StateData represents the information stored in the state parameter
type StateData struct {
	State     string
	ClientID  string
	Timestamp time.Time
}

// GenerateState creates a unique state parameter for CSRF protection
func GenerateState(clientID string) (string, error) {
	// Generate a random byte sequence
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	// Create a unique state by combining random bytes with client ID
	state := hex.EncodeToString(b)

	return state, nil
}

// CreateOAuthConfig creates a new OAuth2 configuration for the specified provider
func CreateOAuthConfig(provider string, clientID, clientSecret, redirectURL string, scopes []string) *oauth2.Config {
	var config *oauth2.Config

	switch provider {
	case "google":
		config = &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       scopes,
			Endpoint:     google.Endpoint,
		}
	case "github":
		config = &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       scopes,
			Endpoint:     github.Endpoint,
		}
	default:
		return nil
	}

	return config
}

// GenerateAuthURL generates the authentication URL for the OAuth2 provider
func GenerateAuthURL(oauthConfig *oauth2.Config, state string) string {
	return oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// ExchangeCodeForToken exchanges the authorization code for an access token
func ExchangeCodeForToken(ctx context.Context, oauthConfig *oauth2.Config, code string) (*oauth2.Token, error) {
	return oauthConfig.Exchange(ctx, code)
}

// GetUserInfo retrieves user information from the OAuth2 provider
func GetUserInfo(ctx context.Context, oauthConfig *oauth2.Config, token *oauth2.Token, provider string) (map[string]interface{}, error) {
	client := oauthConfig.Client(ctx, token)

	var userInfoURL string
	var userInfo map[string]interface{}

	switch provider {
	case "google":
		userInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	case "github":
		userInfoURL = "https://api.github.com/user"
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	resp, err := client.Get(userInfoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got non-200 status code: %d", resp.StatusCode)
	}

	// Parse the response
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return userInfo, nil
}

// GenerateCertificateForUser generates a certificate for the authenticated user
func GenerateCertificateForUser(userInfo map[string]interface{}, provider string) (string, string, string, error) {
	// Extract user identifier (email or username)
	var userIdentifier string

	switch provider {
	case "google":
		if email, ok := userInfo["email"].(string); ok {
			userIdentifier = email
		} else {
			return "", "", "", fmt.Errorf("email not found in Google user info")
		}
	case "github":
		if login, ok := userInfo["login"].(string); ok {
			userIdentifier = login
		} else {
			return "", "", "", fmt.Errorf("login not found in GitHub user info")
		}
	default:
		return "", "", "", fmt.Errorf("unsupported provider: %s", provider)
	}

	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Generate a unique client ID
	clientID := uuid.New().String()

	// Create a certificate template
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   userIdentifier,
			Organization: []string{"NXpose Client"},
		},
		NotBefore:             now,
		NotAfter:              now.Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Create a self-signed certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode certificate to PEM format
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	// Encode private key to PEM format
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return string(certPEM), string(keyPEM), clientID, nil
}
