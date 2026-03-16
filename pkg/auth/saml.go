package auth

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/crewjam/saml"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
)

type samlProvider struct {
	sp *saml.ServiceProvider
}

// NewSAMLProvider creates a SAML 2.0 Service Provider that validates assertions from an IdP.
func NewSAMLProvider(cfg *config.Config) (Provider, error) {
	spCert, spKey, err := loadSPKeyPair(cfg.SAMLSPCertFile, cfg.SAMLSPKeyFile)
	if err != nil {
		return nil, fmt.Errorf("saml sp keypair: %w", err)
	}

	idpMetadata, err := resolveIDPMetadata(cfg)
	if err != nil {
		return nil, fmt.Errorf("saml idp metadata: %w", err)
	}

	acsURL, err := url.Parse(cfg.SAMLSPACS)
	if err != nil {
		return nil, fmt.Errorf("parse SAML_SP_ACS_URL %q: %w", cfg.SAMLSPACS, err)
	}

	entityID, err := url.Parse(cfg.SAMLSPEntityID)
	if err != nil {
		return nil, fmt.Errorf("parse SAML_SP_ENTITY_ID %q: %w", cfg.SAMLSPEntityID, err)
	}

	sp := &saml.ServiceProvider{
		Key:         spKey,
		Certificate: spCert,
		AcsURL:      *acsURL,
		MetadataURL: *entityID,
		IDPMetadata: idpMetadata,
	}

	return &samlProvider{sp: sp}, nil
}

func (p *samlProvider) ValidateToken(_ context.Context, samlResponse string) (*Claims, error) {
	assertion, err := p.sp.ParseXMLResponse([]byte(samlResponse), []string{""}, p.sp.AcsURL)
	if err != nil {
		return nil, fmt.Errorf("saml validate assertion: %w", err)
	}

	claims := &Claims{
		Metadata: make(map[string]any),
	}

	if assertion.Subject != nil && assertion.Subject.NameID != nil {
		claims.UserID = assertion.Subject.NameID.Value
	}

	for _, stmt := range assertion.AttributeStatements {
		for _, attr := range stmt.Attributes {
			if len(attr.Values) == 0 {
				continue
			}
			val := attr.Values[0].Value
			claims.Metadata[attr.Name] = val

			switch attr.Name {
			case "email", "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress":
				claims.Email = val
			case "role", "http://schemas.microsoft.com/ws/2008/06/identity/claims/role":
				claims.Role = val
			case "name", "displayName":
				if claims.UserID == "" {
					claims.UserID = val
				}
			}
		}
	}

	return claims, nil
}

func (p *samlProvider) GenerateToken(_, _, _ string) (string, error) {
	return "", fmt.Errorf("token generation not supported with SAML provider; use IdP login flow")
}

func (p *samlProvider) RefreshToken(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("token refresh not supported with SAML provider; re-authenticate via IdP")
}

func loadSPKeyPair(certFile, keyFile string) (*x509.Certificate, *rsa.PrivateKey, error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read cert %s: %w", certFile, err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("no PEM block found in %s", certFile)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse cert: %w", err)
	}

	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read key %s: %w", keyFile, err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("no PEM block found in %s", keyFile)
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		pkcs8Key, err2 := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err2 != nil {
			return nil, nil, fmt.Errorf("parse private key (tried PKCS1 and PKCS8): %w", err)
		}
		rsaKey, ok := pkcs8Key.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, fmt.Errorf("PKCS8 key is not RSA")
		}
		key = rsaKey
	}

	return cert, key, nil
}

func resolveIDPMetadata(cfg *config.Config) (*saml.EntityDescriptor, error) {
	if cfg.SAMLIDPMetadataURL != "" {
		return fetchIDPMetadata(cfg.SAMLIDPMetadataURL)
	}
	if cfg.SAMLIDPMetadataFile != "" {
		return loadIDPMetadataFile(cfg.SAMLIDPMetadataFile)
	}
	return nil, fmt.Errorf("either SAML_IDP_METADATA_URL or SAML_IDP_METADATA_FILE is required")
}

func fetchIDPMetadata(metadataURL string) (*saml.EntityDescriptor, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
	resp, err := client.Get(metadataURL)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", metadataURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", metadataURL, resp.StatusCode)
	}

	var metadata saml.EntityDescriptor
	if err := xml.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decode idp metadata: %w", err)
	}
	return &metadata, nil
}

func loadIDPMetadataFile(path string) (*saml.EntityDescriptor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var metadata saml.EntityDescriptor
	if err := xml.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("parse idp metadata: %w", err)
	}
	return &metadata, nil
}
