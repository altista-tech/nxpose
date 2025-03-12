package crypto

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/cloudflare"
	"github.com/libdns/digitalocean"
	"github.com/libdns/libdns"
)

// DNSProviderWrapper is an adapter to make libdns providers work with certmagic
// This is needed to handle compatibility issues between different versions of certmagic
type DNSProviderWrapper struct {
	provider interface{} // Use interface{} instead of libdns.Provider
}

// GetRecords gets DNS records from the provider
func (d *DNSProviderWrapper) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	if getter, ok := d.provider.(interface {
		GetRecords(ctx context.Context, zone string) ([]libdns.Record, error)
	}); ok {
		return getter.GetRecords(ctx, zone)
	}
	return nil, nil
}

// AppendRecords adds new DNS records to the provider
func (d *DNSProviderWrapper) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	if appender, ok := d.provider.(interface {
		AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error)
	}); ok {
		return appender.AppendRecords(ctx, zone, records)
	}
	return nil, nil
}

// DeleteRecords removes DNS records from the provider
func (d *DNSProviderWrapper) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	if deleter, ok := d.provider.(interface {
		DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error)
	}); ok {
		return deleter.DeleteRecords(ctx, zone, records)
	}
	return nil, nil
}

// SetTimeout sets the timeout for DNS operations
func (d *DNSProviderWrapper) SetTimeout(timeout time.Duration) {
	if setter, ok := d.provider.(interface{ SetTimeout(time.Duration) }); ok {
		setter.SetTimeout(timeout)
	}
}

// getDNSProvider returns a DNS provider interface compatible with certmagic
func getDNSProvider(providerName string, credentials map[string]string) (interface{}, error) {
	var provider interface{}

	switch providerName {
	case "cloudflare":
		cfProvider := &cloudflare.Provider{}
		if apiToken, ok := credentials["api_token"]; ok {
			cfProvider.APIToken = apiToken
		}
		provider = cfProvider

	case "digitalocean":
		doProvider := &digitalocean.Provider{}
		if apiToken, ok := credentials["api_token"]; ok {
			doProvider.APIToken = apiToken
		}
		provider = doProvider

	default:
		return nil, nil
	}

	return &DNSProviderWrapper{provider: provider}, nil
}

// configureACMEIssuerWithDNS configures the ACME issuer with DNS-01 challenge support
func configureACMEIssuerWithDNS(issuer *certmagic.ACMEIssuer, providerName string, credentials map[string]string) error {
	provider, err := getDNSProvider(providerName, credentials)
	if err != nil {
		return err
	}

	if provider != nil {
		// Cast to the specific type that certmagic expects
		dnsProvider, ok := provider.(*DNSProviderWrapper)
		if !ok {
			return nil
		}

		// In certmagic v0.22.0, need to use setDNSProvider with reflection
		// This is because the field names have changed between versions
		solver := &certmagic.DNS01Solver{}

		// Use reflection to set the DNS provider
		// Try known field names from different certmagic versions
		solverVal := reflect.ValueOf(solver).Elem()
		providerField := solverVal.FieldByName("DNSProvider")

		if providerField.IsValid() && providerField.CanSet() {
			providerField.Set(reflect.ValueOf(dnsProvider))
		} else {
			// Try alternative field name
			providerField = solverVal.FieldByName("Provider")
			if providerField.IsValid() && providerField.CanSet() {
				providerField.Set(reflect.ValueOf(dnsProvider))
			} else {
				return fmt.Errorf("unable to set DNS provider: field not found")
			}
		}

		issuer.DNS01Solver = solver
	}

	return nil
}
