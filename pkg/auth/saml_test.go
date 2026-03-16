package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
)

func generateTestCert(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-sp"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	certFile = filepath.Join(dir, "sp.crt")
	keyFile = filepath.Join(dir, "sp.key")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NoError(t, os.WriteFile(certFile, certPEM, 0600))

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	require.NoError(t, os.WriteFile(keyFile, keyPEM, 0600))

	return certFile, keyFile
}

func writeIDPMetadata(t *testing.T, dir string) string {
	t.Helper()
	// Minimal IdP metadata with an SSO descriptor
	metadata := `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata"
    entityID="https://idp.example.com">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <SingleSignOnService
      Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"
      Location="https://idp.example.com/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`

	path := filepath.Join(dir, "idp-metadata.xml")
	require.NoError(t, os.WriteFile(path, []byte(metadata), 0600))
	return path
}

func TestSAML_NewProvider_Success(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateTestCert(t, dir)
	metadataFile := writeIDPMetadata(t, dir)

	p, err := NewSAMLProvider(&config.Config{
		SAMLSPEntityID:      "https://sp.example.com",
		SAMLSPACS:           "https://sp.example.com/saml/acs",
		SAMLSPCertFile:      certFile,
		SAMLSPKeyFile:       keyFile,
		SAMLIDPMetadataFile: metadataFile,
	})

	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestSAML_NewProvider_MissingKeyPair(t *testing.T) {
	dir := t.TempDir()
	metadataFile := writeIDPMetadata(t, dir)

	_, err := NewSAMLProvider(&config.Config{
		SAMLSPEntityID:      "https://sp.example.com",
		SAMLSPACS:           "https://sp.example.com/saml/acs",
		SAMLSPCertFile:      "/nonexistent/cert.pem",
		SAMLSPKeyFile:       "/nonexistent/key.pem",
		SAMLIDPMetadataFile: metadataFile,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "saml sp keypair")
}

func TestSAML_NewProvider_MissingMetadata(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateTestCert(t, dir)

	_, err := NewSAMLProvider(&config.Config{
		SAMLSPEntityID: "https://sp.example.com",
		SAMLSPACS:      "https://sp.example.com/saml/acs",
		SAMLSPCertFile: certFile,
		SAMLSPKeyFile:  keyFile,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "saml idp metadata")
}

func TestSAML_GenerateToken_Unsupported(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateTestCert(t, dir)
	metadataFile := writeIDPMetadata(t, dir)

	p, err := NewSAMLProvider(&config.Config{
		SAMLSPEntityID:      "https://sp.example.com",
		SAMLSPACS:           "https://sp.example.com/saml/acs",
		SAMLSPCertFile:      certFile,
		SAMLSPKeyFile:       keyFile,
		SAMLIDPMetadataFile: metadataFile,
	})
	require.NoError(t, err)

	token, err := p.GenerateToken("user-1", "a@b.com", "admin")

	assert.Empty(t, token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestSAML_RefreshToken_Unsupported(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateTestCert(t, dir)
	metadataFile := writeIDPMetadata(t, dir)

	p, err := NewSAMLProvider(&config.Config{
		SAMLSPEntityID:      "https://sp.example.com",
		SAMLSPACS:           "https://sp.example.com/saml/acs",
		SAMLSPCertFile:      certFile,
		SAMLSPKeyFile:       keyFile,
		SAMLIDPMetadataFile: metadataFile,
	})
	require.NoError(t, err)

	token, err := p.RefreshToken(context.Background(), "some-token")

	assert.Empty(t, token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestSAML_ValidateToken_InvalidXML(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateTestCert(t, dir)
	metadataFile := writeIDPMetadata(t, dir)

	p, err := NewSAMLProvider(&config.Config{
		SAMLSPEntityID:      "https://sp.example.com",
		SAMLSPACS:           "https://sp.example.com/saml/acs",
		SAMLSPCertFile:      certFile,
		SAMLSPKeyFile:       keyFile,
		SAMLIDPMetadataFile: metadataFile,
	})
	require.NoError(t, err)

	claims, err := p.ValidateToken(context.Background(), "not-valid-saml-xml")

	assert.Nil(t, claims)
	assert.Error(t, err)
}
