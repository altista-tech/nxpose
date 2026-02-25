package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// Helper functions for secure logging
func maskString(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}

func firstChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func lastChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// OAuthConfig represents the configuration for authentication
type OAuthConfig struct {
	Enabled        bool
	RedirectURL    string
	SessionKey     string
	TokenDuration  time.Duration
	CookieDuration time.Duration
	Providers      []ProviderConfig
}

// ProviderConfig represents an OAuth provider configuration
type ProviderConfig struct {
	Name         string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// OAuthService handles authentication using golang.org/x/oauth2
type OAuthService struct {
	logger       *logrus.Logger
	baseURL      string
	mongo        *MongoClient
	config       OAuthConfig
	store        sessions.Store
	providers    map[string]*oauth2.Config
	cookieName   string
	secureCookie bool
}

// NewOAuthService creates a new OAuth service
func NewOAuthService(config OAuthConfig, logger *logrus.Logger, baseURL string, mongo *MongoClient) (*OAuthService, error) {
	if !config.Enabled {
		return nil, nil
	}

	// Set default durations if not specified
	if config.TokenDuration == 0 {
		config.TokenDuration = 5 * time.Minute
	}

	if config.CookieDuration == 0 {
		config.CookieDuration = 24 * time.Hour
	}

	// Extract domain from baseURL to set cookies properly
	domain := ""
	if parsedURL, err := url.Parse(baseURL); err == nil {
		domain = parsedURL.Hostname()
		logger.Infof("Extracted domain for cookies: %s", domain)
	} else {
		logger.Warnf("Failed to parse base URL: %v", err)
	}

	// Create session store
	store := sessions.NewCookieStore([]byte(config.SessionKey))
	store.Options = &sessions.Options{
		Path:     "/",
		Domain:   domain,
		MaxAge:   int(config.CookieDuration.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	}

	service := &OAuthService{
		logger:       logger,
		baseURL:      baseURL,
		mongo:        mongo,
		config:       config,
		store:        store,
		providers:    make(map[string]*oauth2.Config),
		cookieName:   "nxpose_session",
		secureCookie: true,
	}

	// Add OAuth providers
	for _, p := range config.Providers {
		// Skip providers with missing required credentials
		if p.ClientID == "" || p.ClientSecret == "" {
			if logger.Level >= logrus.DebugLevel {
				// Detailed diagnostic logging
				missingFields := []string{}
				if p.ClientID == "" {
					missingFields = append(missingFields, "client_id")
				}
				if p.ClientSecret == "" {
					missingFields = append(missingFields, "client_secret")
				}

				logger.Debugf("OAuth provider '%s' configuration details:", p.Name)
				logger.Debugf("  - Missing fields: %v", missingFields)
				logger.Debugf("  - ClientID present: %v (length: %d)", p.ClientID != "", len(p.ClientID))
				logger.Debugf("  - ClientSecret present: %v (length: %d)", p.ClientSecret != "", len(p.ClientSecret))
				if len(p.Scopes) == 0 {
					logger.Debugf("  - Warning: No scopes defined")
				} else {
					logger.Debugf("  - Scopes: %v", p.Scopes)
				}
			}

			logger.Warnf("Skipping OAuth provider '%s' due to missing credentials", p.Name)
			continue
		}

		// Debug logging for enabled providers
		if logger.Level >= logrus.DebugLevel {
			logger.Debugf("Adding OAuth provider '%s':", p.Name)
			logger.Debugf("  - ClientID: %s (first/last 4 chars: %s...%s)",
				maskString(p.ClientID),
				firstChars(p.ClientID, 4),
				lastChars(p.ClientID, 4))
			logger.Debugf("  - ClientSecret: (first/last 4 chars: %s...%s)",
				firstChars(p.ClientSecret, 4),
				lastChars(p.ClientSecret, 4))
			logger.Debugf("  - Scopes: %v", p.Scopes)
		}

		var oauth2Config *oauth2.Config
		callbackPath := "/auth/callback/" + p.Name

		switch p.Name {
		case "github":
			// Use prebuilt GitHub endpoint
			oauth2Config = &oauth2.Config{
				ClientID:     p.ClientID,
				ClientSecret: p.ClientSecret,
				RedirectURL:  baseURL + callbackPath,
				Scopes:       p.Scopes,
				Endpoint:     github.Endpoint,
			}
			logger.Infof("Added GitHub OAuth provider")
		case "google":
			// Use prebuilt Google endpoint
			oauth2Config = &oauth2.Config{
				ClientID:     p.ClientID,
				ClientSecret: p.ClientSecret,
				RedirectURL:  baseURL + callbackPath,
				Scopes:       p.Scopes,
				Endpoint:     google.Endpoint,
			}
			logger.Infof("Added Google OAuth provider")
		default:
			logger.Warnf("Unsupported OAuth provider: %s", p.Name)
			continue
		}

		// Add the provider to our map
		service.providers[p.Name] = oauth2Config
	}

	if len(service.providers) == 0 {
		logger.Warn("No OAuth providers configured properly")
	}

	return service, nil
}

// RegisterRoutes registers OAuth routes with the router
func (s *OAuthService) RegisterRoutes(router *mux.Router) {
	// Registration page
	router.HandleFunc("/auth/register", s.handleRegister).Methods("GET")

	// OAuth login initiation for each provider
	for provider := range s.providers {
		router.HandleFunc("/auth/login/"+provider, s.handleLogin(provider)).Methods("GET")
	}

	// OAuth callback handler
	router.HandleFunc("/auth/callback/{provider}", s.handleCallback).Methods("GET")

	// OAuth completion handler
	router.HandleFunc("/auth/oauth-done", s.handleOAuthDone).Methods("GET")
}

// handleRegister renders the registration page with available OAuth providers
func (s *OAuthService) handleRegister(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Registration page requested")
	s.logger.Infof("Registration request full URL: %s", r.URL.String())

	// Store the callback URL in a session if provided
	callback := r.URL.Query().Get("callback")
	state := r.URL.Query().Get("state")

	if callback != "" {
		// Store callback and state in session
		session, _ := s.store.Get(r, s.cookieName)
		session.Values["callback_url"] = callback
		if state != "" {
			session.Values["callback_state"] = state
		}
		session.Save(r, w)

		s.logger.Infof("Stored callback URL in session: %s", callback)
		if state != "" {
			s.logger.Infof("Stored state in session: %s", state)
		}
	}

	// Initialize with standard CSS and HTML
	w.Header().Set("Content-Type", "text/html")
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>NXpose - Register with OAuth</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 600px;
            margin: 40px auto;
            padding: 20px;
            text-align: center;
        }
        h1 {
            color: #333;
            margin-bottom: 30px;
        }
        .provider-button {
            display: inline-block;
            padding: 12px 24px;
            margin: 10px;
            border-radius: 4px;
            color: white;
            text-decoration: none;
            font-weight: bold;
            transition: background-color 0.3s;
            cursor: pointer;
        }
        .google {
            background-color: #DB4437;
        }
        .google:hover {
            background-color: #C1351D;
        }
        .github {
            background-color: #333;
        }
        .github:hover {
            background-color: #000;
        }
        .info {
            margin-top: 30px;
            font-size: 14px;
            color: #666;
        }
    </style>
</head>
<body>
    <h1>Register with NXpose</h1>
    <p>Choose a provider to authenticate and register your client:</p>
    
    <div>
`

	// Track if we have any valid providers
	hasProviders := false

	// Add buttons for all configured providers
	for provider := range s.providers {
		hasProviders = true

		var buttonClass, providerName string
		switch provider {
		case "github":
			buttonClass = "github"
			providerName = "GitHub"
		case "google":
			buttonClass = "google"
			providerName = "Google"
		default:
			buttonClass = provider
			providerName = strings.ToUpper(provider[:1]) + provider[1:]
		}

		html += fmt.Sprintf(`        <a href="/auth/login/%s" class="provider-button %s">Sign in with %s</a>
`, provider, buttonClass, providerName)
	}

	// If no providers were configured with valid credentials
	if !hasProviders {
		html += `        <p>No OAuth providers are properly configured. Please contact the administrator.</p>
`
	}

	html += `    </div>
    
    <div class="info">
        <p>After authenticating, a secure certificate will be generated for your client.</p>
    </div>
</body>
</html>
`
	w.Write([]byte(html))
}

// handleLogin returns a handler function for initiating OAuth login with a specific provider
func (s *OAuthService) handleLogin(provider string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get OAuth config for requested provider
		config, exists := s.providers[provider]
		if !exists {
			http.Error(w, "Unsupported OAuth provider", http.StatusBadRequest)
			return
		}

		// Generate a random state parameter for security
		b := make([]byte, 32)
		_, err := rand.Read(b)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			s.logger.Errorf("Failed to generate random state: %v", err)
			return
		}
		state := base64.StdEncoding.EncodeToString(b)

		// Store state in session for verification during callback
		session, _ := s.store.Get(r, s.cookieName)
		session.Values["oauth_state"] = state
		session.Values["oauth_provider"] = provider
		session.Save(r, w)

		// Redirect user to OAuth provider
		url := config.AuthCodeURL(state)
		s.logger.Infof("Redirecting to OAuth provider %s: %s", provider, url)
		http.Redirect(w, r, url, http.StatusFound)
	}
}

// handleCallback processes OAuth callback from providers
func (s *OAuthService) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Extract provider from URL path
	vars := mux.Vars(r)
	provider := vars["provider"]

	s.logger.Infof("Received callback from provider: %s", provider)
	s.logger.Infof("Full callback URL: %s", r.URL.String())

	// Get the code and state from the request
	code := r.URL.Query().Get("code")
	receivedState := r.URL.Query().Get("state")

	if code == "" || receivedState == "" {
		s.logger.Errorf("Missing code or state in callback")
		http.Error(w, "Invalid callback request", http.StatusBadRequest)
		return
	}

	// Verify state from session
	session, err := s.store.Get(r, s.cookieName)
	if err != nil {
		s.logger.Errorf("Failed to get session: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	expectedState, ok := session.Values["oauth_state"].(string)
	if !ok || expectedState == "" || expectedState != receivedState {
		s.logger.Errorf("Invalid OAuth state, expected %s, got %s", expectedState, receivedState)
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Get OAuth config for this provider
	config, exists := s.providers[provider]
	if !exists {
		s.logger.Errorf("Provider not found: %s", provider)
		http.Error(w, "Invalid provider", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	ctx := context.Background()
	token, err := config.Exchange(ctx, code)
	if err != nil {
		s.logger.Errorf("Failed to exchange token: %v", err)
		http.Error(w, "Failed to authenticate", http.StatusInternalServerError)
		return
	}

	// Get user info from provider
	client := config.Client(ctx, token)
	userInfo, err := s.getUserInfo(client, provider)
	if err != nil {
		s.logger.Errorf("Failed to get user info: %v", err)
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}

	// Extract user details
	userID := fmt.Sprintf("%s_%x", provider, sha1.Sum([]byte(fmt.Sprintf("%v", userInfo["id"]))))
	userName := s.extractUserName(userInfo, provider)

	// Handle avatar if available
	avatarHash := ""
	if avatarURL, ok := userInfo["avatar_url"].(string); ok && avatarURL != "" {
		avatarHash = s.saveAvatar(avatarURL)
	}

	// Get client callback and state from session
	clientCallback, _ := session.Values["callback_url"].(string)
	clientState, _ := session.Values["callback_state"].(string)

	// Redirect to OAuth done page with user info
	redirectParams := url.Values{}
	redirectParams.Set("user_id", url.QueryEscape(userID))
	redirectParams.Set("user_name", url.QueryEscape(userName))
	if avatarHash != "" {
		redirectParams.Set("avatar", url.QueryEscape(avatarHash))
	}

	// Add client callback if we have it
	if clientCallback != "" {
		redirectParams.Set("client_callback", url.QueryEscape(clientCallback))
		s.logger.Infof("Adding client_callback to redirect: %s", clientCallback)
		if clientState != "" {
			redirectParams.Set("client_state", url.QueryEscape(clientState))
			s.logger.Infof("Adding client_state to redirect: %s", clientState)
		}
	}

	redirectURL := fmt.Sprintf("/auth/oauth-done?%s", redirectParams.Encode())
	s.logger.Infof("Authentication successful for user %s, redirecting to %s", userName, redirectURL)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// getUserInfo retrieves user information from the OAuth provider
func (s *OAuthService) getUserInfo(client *http.Client, provider string) (map[string]interface{}, error) {
	var apiURL string

	switch provider {
	case "github":
		apiURL = "https://api.github.com/user"
	case "google":
		apiURL = "https://www.googleapis.com/oauth2/v3/userinfo"
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	// Make the API request
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse the JSON response
	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	s.logger.Debugf("User info from %s: %v", provider, userInfo)
	return userInfo, nil
}

// extractUserName extracts the user's name from provider-specific user info
func (s *OAuthService) extractUserName(userInfo map[string]interface{}, provider string) string {
	switch provider {
	case "github":
		// Try name first, then login
		if name, ok := userInfo["name"].(string); ok && name != "" {
			return name
		}
		if login, ok := userInfo["login"].(string); ok {
			return login
		}
	case "google":
		// Try name first, then given_name + family_name, then email
		if name, ok := userInfo["name"].(string); ok && name != "" {
			return name
		}

		var parts []string
		if given, ok := userInfo["given_name"].(string); ok && given != "" {
			parts = append(parts, given)
		}
		if family, ok := userInfo["family_name"].(string); ok && family != "" {
			parts = append(parts, family)
		}
		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}

		if email, ok := userInfo["email"].(string); ok {
			return email
		}
	}

	// Fallback to a default name with provider prefix
	return provider + "_user"
}

// saveAvatar downloads and saves the avatar image
func (s *OAuthService) saveAvatar(avatarURL string) string {
	avatarHash := fmt.Sprintf("%x", sha1.Sum([]byte(avatarURL)))

	// Download the avatar
	resp, err := http.Get(avatarURL)
	if err != nil {
		s.logger.Warnf("Failed to download avatar: %v", err)
		return ""
	}
	defer resp.Body.Close()

	// Read avatar data
	avatarData, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Warnf("Failed to read avatar data: %v", err)
		return ""
	}

	// Ensure avatar directory exists
	os.MkdirAll("/tmp/nxpose-avatars", 0755)

	// Save avatar file
	err = os.WriteFile(fmt.Sprintf("/tmp/nxpose-avatars/%s.image", avatarHash), avatarData, 0644)
	if err != nil {
		s.logger.Warnf("Failed to save avatar: %v", err)
		return ""
	}

	s.logger.Debugf("Saved avatar from %s to %s.image", avatarURL, avatarHash)
	return avatarHash
}

// handleOAuthDone handles OAuth completion and redirects to the client
func (s *OAuthService) handleOAuthDone(w http.ResponseWriter, r *http.Request) {
	s.logger.Infof("OAuth done handler called with URL: %s", r.URL.String())

	// Get user details from URL parameters
	userID := r.URL.Query().Get("user_id")
	userName := r.URL.Query().Get("user_name")
	avatarHash := r.URL.Query().Get("avatar")

	if userID == "" || userName == "" {
		s.logger.Error("Missing user ID or name in oauth-done handler")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get client callback and state from query params
	clientCallbackEncoded := r.URL.Query().Get("client_callback")
	s.logger.Infof("Raw encoded client callback: %s", clientCallbackEncoded)

	clientCallback, err := url.QueryUnescape(clientCallbackEncoded)
	if err != nil {
		s.logger.Errorf("Failed to unescape client callback URL: %v", err)
		clientCallback = clientCallbackEncoded // Use as-is if unescaping fails
	}

	clientState := r.URL.Query().Get("client_state")
	if clientState != "" {
		s.logger.Infof("Raw encoded client state: %s", clientState)
		clientState, err = url.QueryUnescape(clientState)
		if err != nil {
			s.logger.Warnf("Failed to unescape client state: %v", err)
		}
	}

	s.logger.Infof("Client callback URL: %s, state: %s", clientCallback, clientState)

	// Generate a certificate for the user
	cert, err := s.GenerateCertificate(userID, userName)
	if err != nil {
		s.logger.Errorf("Failed to generate certificate: %v", err)
		http.Error(w, "Failed to generate certificate", http.StatusInternalServerError)
		return
	}

	// Check if we need to redirect back to a client callback
	if clientCallback != "" {
		s.logger.Infof("Client callback found, preparing certificate for: %s", clientCallback)

		// Make sure it's a valid URL before proceeding
		parsedURL, err := url.Parse(clientCallback)
		if err != nil {
			s.logger.Errorf("Invalid client callback URL: %v", err)
			http.Error(w, "Invalid callback URL", http.StatusBadRequest)
			return
		}

		// Ensure it's an absolute URL with scheme
		if !parsedURL.IsAbs() {
			s.logger.Warnf("Client callback URL is not absolute, adding http:// scheme: %s", clientCallback)
			// Add http:// as scheme if not present
			if !strings.HasPrefix(clientCallback, "http://") && !strings.HasPrefix(clientCallback, "https://") {
				clientCallback = "http://" + clientCallback
				s.logger.Infof("Modified client callback URL: %s", clientCallback)
			}
		}

		// Encode certificate data for the callback
		certData := base64.StdEncoding.EncodeToString([]byte(cert["certificate"].(string) + "\n" + cert["private_key"].(string)))

		// Build the callback URL with certificate data directly
		var stateParam string
		if clientState != "" {
			stateParam = "&state=" + url.QueryEscape(clientState)
		}

		responseHTML := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Registration Complete</title>
    <script>
        window.location.href = "%s?certificate=%s%s";
    </script>
</head>
<body>
    <h1>Registration Complete</h1>
    <p>Redirecting to client...</p>
    <p>If you are not redirected automatically, <a href="%s?certificate=%s%s">click here</a>.</p>
</body>
</html>
`,
			clientCallback,
			url.QueryEscape(certData),
			stateParam,
			clientCallback,
			url.QueryEscape(certData),
			stateParam)

		// Set content type and write the HTML response directly
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(responseHTML))
		s.logger.Infof("Sending HTML with JavaScript redirect to: %s", clientCallback)
		return
	}

	// If no client callback, show a success page with user details
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":    userName,
		"id":      userID,
		"picture": fmt.Sprintf("%s/avatar/%s.image", s.baseURL, avatarHash),
	})
}

// GenerateCertificate creates a new certificate for a user
func (s *OAuthService) GenerateCertificate(userID, userName string) (map[string]interface{}, error) {
	s.logger.Infof("Generating certificate for user %s (ID: %s)", userName, userID)

	// Generate a certificate ID based on user ID
	certID := fmt.Sprintf("cert-%s-%d", userID[:8], time.Now().Unix())

	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create a certificate template
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   userName,
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
		return nil, fmt.Errorf("failed to create certificate: %w", err)
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

	cert := map[string]interface{}{
		"cert_id":     certID,
		"user_id":     userID,
		"user_name":   userName,
		"issued_at":   time.Now().Format(time.RFC3339),
		"expires_at":  time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
		"certificate": string(certPEM),
		"private_key": string(keyPEM),
	}

	// Store the certificate in the database if MongoDB is available
	if s.mongo != nil {
		// Store certificate in MongoDB for future reference
		collection := s.mongo.database.Collection("certificates")
		_, err = collection.InsertOne(context.Background(), cert)
		if err != nil {
			s.logger.Warnf("Failed to store certificate in MongoDB: %v", err)
		} else {
			s.logger.Infof("Certificate %s stored in database", certID)
		}
	} else {
		s.logger.Warnf("MongoDB not available, certificate %s for user %s will not be persisted", certID, userID)
	}

	return cert, nil
}

// ValidateOAuthConfig validates OAuth2 configuration and returns detailed diagnostic information
func ValidateOAuthConfig(config OAuthConfig, logger *logrus.Logger) map[string]interface{} {
	result := map[string]interface{}{
		"valid":     true,
		"enabled":   config.Enabled,
		"issues":    []string{},
		"providers": []map[string]interface{}{},
	}

	// If OAuth2 is not enabled, no need to validate further
	if !config.Enabled {
		return result
	}

	issues := []string{}

	// Check redirect URL
	if config.RedirectURL == "" {
		issues = append(issues, "Missing redirect URL")
		result["valid"] = false
	} else if !strings.HasPrefix(config.RedirectURL, "https://") {
		issues = append(issues, "Redirect URL should use HTTPS protocol")
		result["valid"] = false
	}

	// Check session key
	if config.SessionKey == "" {
		issues = append(issues, "Missing session key")
		result["valid"] = false
	} else if config.SessionKey == "change-this-to-a-random-secret-key" {
		issues = append(issues, "Using default session key; should be changed to a random value")
		result["valid"] = false
	} else if len(config.SessionKey) < 16 {
		issues = append(issues, "Session key is too short (< 16 characters)")
		result["valid"] = false
	}

	// Check providers
	if len(config.Providers) == 0 {
		issues = append(issues, "No OAuth providers configured")
		result["valid"] = false
	} else {
		hasValidProvider := false

		for _, p := range config.Providers {
			providerInfo := map[string]interface{}{
				"name":   p.Name,
				"valid":  true,
				"issues": []string{},
			}

			providerIssues := []string{}

			// Check required fields
			if p.ClientID == "" {
				providerIssues = append(providerIssues, "Missing client_id")
				providerInfo["valid"] = false
			}

			if p.ClientSecret == "" {
				providerIssues = append(providerIssues, "Missing client_secret")
				providerInfo["valid"] = false
			}

			if len(p.Scopes) == 0 {
				providerIssues = append(providerIssues, "No scopes defined")
				providerInfo["valid"] = false
			}

			// Check provider type
			switch p.Name {
			case "github", "google", "facebook", "microsoft", "yandex", "dev":
				// Valid provider type
			default:
				providerIssues = append(providerIssues, "Unsupported provider type")
				providerInfo["valid"] = false
			}

			if providerInfo["valid"].(bool) {
				hasValidProvider = true
			}

			providerInfo["issues"] = providerIssues
			result["providers"] = append(result["providers"].([]map[string]interface{}), providerInfo)
		}

		if !hasValidProvider {
			issues = append(issues, "No valid OAuth providers configured")
			result["valid"] = false
		}
	}

	result["issues"] = issues
	return result
}
