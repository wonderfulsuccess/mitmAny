package certificates

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"github.com/lqqyt2423/go-mitmproxy/cert"
	"os"
	"path/filepath"
	"time"
)

type Status struct {
	Exists      bool   `json:"exists"`
	Trusted     bool   `json:"trusted"`
	Valid       bool   `json:"valid"`
	Subject     string `json:"subject"`
	Fingerprint string `json:"fingerprint"`
	Expires     string `json:"expires"`
	Path        string `json:"path"`
	Error       string `json:"error,omitempty"`
}
type Manager struct {
	Dir string
	CA  cert.CA
}

func New(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	ca, err := cert.NewSelfSignCA(dir)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(filepath.Join(dir, "mitmproxy-ca.pem"), 0600)
	return &Manager{dir, ca}, nil
}
func (m *Manager) Path() string            { return filepath.Join(m.Dir, "mitmproxy-ca-cert.pem") }
func (m *Manager) Root() *x509.Certificate { return m.CA.GetRootCA() }
func (m *Manager) SHA1() string            { sum := sha1.Sum(m.Root().Raw); return hex.EncodeToString(sum[:]) }
func (m *Manager) Status() Status {
	c := m.Root()
	sum := sha256.Sum256(c.Raw)
	s := Status{Exists: true, Valid: time.Now().After(c.NotBefore) && time.Now().Before(c.NotAfter), Subject: c.Subject.CommonName, Fingerprint: hex.EncodeToString(sum[:]), Expires: c.NotAfter.Format(time.RFC3339), Path: m.Path()}
	err := checkTrust(m)
	_, statErr := os.Stat(m.Path())
	s.Exists = statErr == nil
	s.Trusted = err == nil && s.Valid
	if err != nil {
		s.Error = err.Error()
	}
	return s
}
func (m *Manager) Export(path string) error {
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: m.Root().Raw})
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("export certificate: %w", err)
	}
	return nil
}
func (m *Manager) Install() error {
	if err := m.Export(m.Path()); err != nil {
		return err
	}
	if err := install(m); err != nil {
		return err
	}
	if !m.Status().Trusted {
		return fmt.Errorf("证书已提交安装，但尚未通过系统信任检查")
	}
	return nil
}
func (m *Manager) Uninstall() error { return uninstall(m) }
