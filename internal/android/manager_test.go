package android

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type phone struct {
	proxy   string
	reverse bool
	offline bool
	reject  bool
	calls   []string
}

func (p *phone) run(args ...string) (string, error) {
	s := strings.Join(args, " ")
	p.calls = append(p.calls, s)
	if p.offline {
		return "", errors.New("device offline")
	}
	switch {
	case s == "devices -l":
		return "List of devices attached\nabc device product:test model:Pixel_8\nxyz unauthorized\noff offline", nil
	case strings.Contains(s, "settings get"):
		return p.proxy, nil
	case strings.Contains(s, "settings delete"):
		p.proxy = "null"
		return "", nil
	case strings.Contains(s, "settings put"):
		if p.reject && args[len(args)-1] == Proxy {
			return "", errors.New("permission denied")
		}
		p.proxy = strings.Trim(args[len(args)-1], "'")
		return "", nil
	case strings.Contains(s, "reverse --list"):
		if p.reverse {
			return "UsbFfs tcp:9081 tcp:9081", nil
		}
		return "", nil
	case strings.Contains(s, "reverse --no-rebind"):
		p.reverse = true
		return "", nil
	case strings.Contains(s, "reverse --remove"):
		p.reverse = false
		return "", nil
	case strings.Contains(s, "am start -n"):
		return "Error: Activity class does not exist", nil
	}
	return "", nil
}
func setup(t *testing.T, p *phone) *Manager {
	t.Helper()
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.run = p.run
	return m
}
func TestProxyRestoreSurvivesRestart(t *testing.T) {
	for _, original := range []string{"null", ":0", "10.0.0.2:8888"} {
		t.Run(original, func(t *testing.T) {
			p := &phone{proxy: original}
			m := setup(t, p)
			if err := m.Enable("abc"); err != nil {
				t.Fatal(err)
			}
			if p.proxy != Proxy || !p.reverse {
				t.Fatal("routing not enabled")
			}
			recovered, err := New(filepath.Dir(m.path))
			if err != nil {
				t.Fatal(err)
			}
			recovered.run = p.run
			p.offline = true
			if err = recovered.Restore("abc"); err == nil || len(recovered.Pending()) != 1 {
				t.Fatal("lost disconnected device backup")
			}
			p.offline = false
			// USB reconnect can discard the old reverse mapping.
			p.reverse = false
			if err = recovered.Restore("abc"); err != nil {
				t.Fatal(err)
			}
			if p.proxy != original || p.reverse || len(recovered.Pending()) != 0 {
				t.Fatal("routing not restored")
			}
		})
	}
}
func TestEnableFailureRollsBack(t *testing.T) {
	p := &phone{proxy: "null", reject: true}
	m := setup(t, p)
	if m.Enable("abc") == nil {
		t.Fatal("accepted rejected setting")
	}
	if p.proxy != "null" || p.reverse || len(m.Pending()) != 0 {
		t.Fatal("rollback failed")
	}
}
func TestExistingTunnelNotReplaced(t *testing.T) {
	p := &phone{proxy: ":0", reverse: true}
	m := setup(t, p)
	if m.Enable("abc") == nil {
		t.Fatal("replaced tunnel")
	}
	if len(m.Pending()) != 0 || p.proxy != ":0" {
		t.Fatal("mutated device")
	}
}
func TestBackupMustBeSavedBeforeMutation(t *testing.T) {
	p := &phone{proxy: "null"}
	m := setup(t, p)
	m.path = filepath.Join(t.TempDir(), "missing", "backup.json")
	if m.Enable("abc") == nil {
		t.Fatal("ignored backup error")
	}
	if p.proxy != "null" || p.reverse {
		t.Fatal("changed routing before backup")
	}
}
func TestDevicesAndCertificateFallback(t *testing.T) {
	p := &phone{}
	m := setup(t, p)
	devices, err := m.Devices()
	if err != nil || len(devices) != 3 || devices[0].Model != "Pixel 8" || devices[1].State != "unauthorized" {
		t.Fatal(devices, err)
	}
	message, err := m.InstallCertificate("abc", "public.crt")
	if err != nil || !strings.Contains(message, "Download/mitmAny-ca.crt") {
		t.Fatal(message, err)
	}
	all := strings.Join(p.calls, "\n")
	if !strings.Contains(all, "push public.crt "+CertificatePath) || !strings.Contains(all, "android.settings.SECURITY_SETTINGS") {
		t.Fatal(all)
	}
}
func TestCorruptBackupNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "android-proxies.json"), []byte("bad json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir); err == nil {
		t.Fatal("ignored corrupt backup")
	}
}

func TestRestoreAllContinuesAfterOfflineDevice(t *testing.T) {
	first := &phone{proxy: "null"}
	second := &phone{proxy: "null"}
	manager := setup(t, first)
	manager.run = func(args ...string) (string, error) {
		if len(args) > 1 && args[1] == "bbb" {
			return second.run(args...)
		}
		return first.run(args...)
	}
	if err := manager.Enable("aaa"); err != nil {
		t.Fatal(err)
	}
	if err := manager.Enable("bbb"); err != nil {
		t.Fatal(err)
	}
	first.offline = true
	if err := manager.RestoreAll(); err == nil {
		t.Fatal("offline device must report a failure")
	}
	if pending := manager.Pending(); len(pending) != 1 || pending[0] != "aaa" {
		t.Fatal("wrong pending devices", pending)
	}
	if second.proxy != "null" || second.reverse {
		t.Fatal("connected second device was not restored")
	}
	first.offline = false
	if err := manager.RestoreAll(); err != nil {
		t.Fatal(err)
	}
	if len(manager.Pending()) != 0 || first.proxy != "null" || first.reverse {
		t.Fatal("retry failed")
	}
}
