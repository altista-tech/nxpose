package crypto

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDNSProvider_Cloudflare(t *testing.T) {
	credentials := map[string]string{
		"api_token": "test-cf-token",
	}

	provider, err := getDNSProvider("cloudflare", credentials)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Should be wrapped in DNSProviderWrapper
	wrapper, ok := provider.(*DNSProviderWrapper)
	assert.True(t, ok)
	assert.NotNil(t, wrapper.provider)
}

func TestGetDNSProvider_DigitalOcean(t *testing.T) {
	credentials := map[string]string{
		"api_token": "test-do-token",
	}

	provider, err := getDNSProvider("digitalocean", credentials)
	require.NoError(t, err)
	require.NotNil(t, provider)

	wrapper, ok := provider.(*DNSProviderWrapper)
	assert.True(t, ok)
	assert.NotNil(t, wrapper.provider)
}

func TestGetDNSProvider_UnsupportedProvider(t *testing.T) {
	provider, err := getDNSProvider("route53", map[string]string{})
	assert.NoError(t, err) // Returns nil, nil for unsupported
	assert.Nil(t, provider)
}

func TestGetDNSProvider_EmptyProvider(t *testing.T) {
	provider, err := getDNSProvider("", map[string]string{})
	assert.NoError(t, err)
	assert.Nil(t, provider)
}

func TestGetDNSProvider_EmptyCredentials(t *testing.T) {
	// Should still create provider, just with empty token
	provider, err := getDNSProvider("cloudflare", map[string]string{})
	require.NoError(t, err)
	require.NotNil(t, provider)
}

func TestGetDNSProvider_NilCredentials(t *testing.T) {
	provider, err := getDNSProvider("cloudflare", nil)
	require.NoError(t, err)
	require.NotNil(t, provider)
}

func TestDNSProviderWrapper_MethodsWithNonImplementingProvider(t *testing.T) {
	// Wrap a simple struct that doesn't implement any DNS methods
	wrapper := &DNSProviderWrapper{provider: struct{}{}}

	// All methods should return nil gracefully
	records, err := wrapper.GetRecords(context.TODO(), "example.com.")
	assert.NoError(t, err)
	assert.Nil(t, records)

	records, err = wrapper.AppendRecords(context.TODO(), "example.com.", nil)
	assert.NoError(t, err)
	assert.Nil(t, records)

	records, err = wrapper.DeleteRecords(context.TODO(), "example.com.", nil)
	assert.NoError(t, err)
	assert.Nil(t, records)

	// SetTimeout should not panic
	assert.NotPanics(t, func() {
		wrapper.SetTimeout(0)
	})
}

func TestConfigureACMEIssuerWithDNS_UnsupportedProvider(t *testing.T) {
	// For unsupported provider, getDNSProvider returns nil, nil
	// so configureACMEIssuerWithDNS should return nil (no-op)
	err := configureACMEIssuerWithDNS(nil, "unsupported", map[string]string{})
	assert.NoError(t, err)
}
