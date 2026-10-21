// Package android manages ADB routing and durable restoration of device proxies.
package android

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const Endpoint = "tcp:9081"
const Proxy = "127.0.0.1:9081"
const CertificatePath = "/sdcard/Download/mitmAny-ca.crt"

type Device struct {
	Serial string `json:"serial"`
	State  string `json:"state"`
	Model  string `json:"model"`
}
type Backup struct {
	Proxy   string `json:"proxy"`
	Reverse bool   `json:"reverse"`
}
type Manager struct {
	path    string
	backups map[string]Backup
	run     func(...string) (string, error)
}

func New(dir string) (*Manager, error) {
	m := &Manager{path: filepath.Join(dir, "android-proxies.json"), backups: map[string]Backup{}, run: command}
	data, err := os.ReadFile(m.path)
	if err == nil {
		err = json.Unmarshal(data, &m.backups)
		if m.backups == nil {
			m.backups = map[string]Backup{}
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取 Android 代理备份: %w", err)
	}
	return m, nil
}
func adbPath() (string, error) {
	if path, err := exec.LookPath("adb"); err == nil {
		return path, nil
	}
	home, _ := os.UserHomeDir()
	executable := "adb"
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	candidates := []string{
		filepath.Join(home, "Library/Android/sdk/platform-tools", executable),
		filepath.Join(home, "Android/Sdk/platform-tools", executable),
		"/opt/homebrew/bin/adb", "/usr/local/bin/adb",
	}
	for _, key := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if root := os.Getenv(key); root != "" {
			candidates = append(candidates, filepath.Join(root, "platform-tools", executable))
		}
	}
	if root := os.Getenv("LOCALAPPDATA"); root != "" {
		candidates = append(candidates, filepath.Join(root, "Android/Sdk/platform-tools", executable))
	}
	for _, p := range candidates {
		if s, e := os.Stat(p); e == nil && !s.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("未找到 ADB，请安装 Android SDK Platform-Tools，并将 adb 加入 PATH")
}
func command(args ...string) (string, error) {
	path, err := adbPath()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ADB: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}
func (m *Manager) device(serial string, args ...string) (string, error) {
	if serial == "" || strings.HasPrefix(serial, "-") || strings.ContainsAny(serial, "\r\n\x00") {
		return "", errors.New("请选择有效的 Android 设备")
	}
	return m.run(append([]string{"-s", serial}, args...)...)
}
func (m *Manager) Devices() ([]Device, error) {
	out, err := m.run("devices", "-l")
	if err != nil {
		return nil, err
	}
	devices := []Device{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || strings.HasPrefix(line, "List ") || strings.HasPrefix(line, "*") {
			continue
		}
		d := Device{Serial: f[0], State: f[1]}
		for _, v := range f[2:] {
			if strings.HasPrefix(v, "model:") {
				d.Model = strings.ReplaceAll(strings.TrimPrefix(v, "model:"), "_", " ")
			}
		}
		devices = append(devices, d)
	}
	return devices, nil
}
func (m *Manager) Pending() []string {
	out := []string{}
	for serial := range m.backups {
		out = append(out, serial)
	}
	sort.Strings(out)
	return out
}
func (m *Manager) save() error {
	data, err := json.Marshal(m.backups)
	if err != nil {
		return err
	}
	if err = os.WriteFile(m.path+".tmp", data, 0600); err != nil {
		return err
	}
	return os.Rename(m.path+".tmp", m.path)
}
func (m *Manager) Enable(serial string) error {
	if _, ok := m.backups[serial]; ok {
		return errors.New("此设备有待恢复的代理，请先恢复再设置")
	}
	original, err := m.device(serial, "shell", "settings", "get", "global", "http_proxy")
	if err != nil {
		return err
	}
	// Refuse an existing reverse mapping rather than replacing another tool's tunnel.
	out, err := m.device(serial, "reverse", "--list")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(out, "\n") {
		for _, f := range strings.Fields(line) {
			if f == Endpoint {
				return errors.New("手机端口 9081 已被 ADB 隧道占用，请先移除已有映射")
			}
		}
	}
	m.backups[serial] = Backup{Proxy: original, Reverse: true}
	if err = m.save(); err != nil {
		delete(m.backups, serial)
		return err
	}
	if _, err = m.device(serial, "reverse", "--no-rebind", Endpoint, Endpoint); err != nil {
		m.backups[serial] = Backup{Proxy: original}
		_ = m.save()
		return err
	}
	_, err = m.device(serial, "shell", "settings", "put", "global", "http_proxy", Proxy)
	if err == nil {
		var got string
		got, err = m.device(serial, "shell", "settings", "get", "global", "http_proxy")
		if err == nil && got != Proxy {
			err = errors.New("手机未接受代理设置，请检查 USB 调试的安全授权")
		}
	}
	if err != nil {
		restoreErr := m.Restore(serial)
		return errors.Join(err, restoreErr)
	}
	return nil
}

// quote keeps a saved setting literal when adb shell joins its arguments.
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func (m *Manager) Restore(serial string) error {
	b, ok := m.backups[serial]
	if !ok {
		return nil
	}
	var err error
	if b.Proxy == "null" {
		_, err = m.device(serial, "shell", "settings", "delete", "global", "http_proxy")
	} else {
		_, err = m.device(serial, "shell", "settings", "put", "global", "http_proxy", quote(b.Proxy))
	}
	if err != nil {
		return err
	}
	got, err := m.device(serial, "shell", "settings", "get", "global", "http_proxy")
	if err != nil {
		return err
	}
	if got != b.Proxy {
		return errors.New("手机代理恢复校验失败")
	}
	if b.Reverse {
		mappings, listErr := m.device(serial, "reverse", "--list")
		if listErr != nil {
			return listErr
		}
		for _, line := range strings.Split(mappings, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 3 && fields[1] == Endpoint && fields[2] == Endpoint {
				if _, err = m.device(serial, "reverse", "--remove", Endpoint); err != nil {
					return err
				}
			}
		}
	}
	delete(m.backups, serial)
	if err = m.save(); err != nil {
		m.backups[serial] = b
		return err
	}
	return nil
}

// RestoreAll attempts every saved device even if another is disconnected.
// Failed devices keep their durable backups for a later retry.
func (m *Manager) RestoreAll() error {
	var failures []error
	for _, serial := range m.Pending() {
		if err := m.Restore(serial); err != nil {
			failures = append(failures, fmt.Errorf("请连接手机 %s 后恢复代理: %w", serial, err))
		}
	}
	return errors.Join(failures...)
}

func (m *Manager) InstallCertificate(serial, local string) (string, error) {
	if _, err := m.device(serial, "push", local, CertificatePath); err != nil {
		return "", err
	}
	// Modern Android requires choosing the CA file from Settings, with user confirmation.
	targets := [][]string{
		{"-n", quote("com.android.settings/.Settings$CredentialStorageActivity")},
		{"-a", "android.settings.SECURITY_SETTINGS"},
		{"-a", "android.settings.SETTINGS"},
	}
	for _, target := range targets {
		out, err := m.device(serial, append([]string{"shell", "am", "start"}, target...)...)
		if err == nil && !strings.Contains(out, "Error") && !strings.Contains(out, "Exception") {
			return "证书已复制到 Download/mitmAny-ca.crt，已打开证书/安全设置。请选择「安装证书 → CA 证书」并选取该文件，按手机提示确认安装。", nil
		}
	}
	return "", errors.New("证书已复制到 Download/mitmAny-ca.crt，但无法打开设置；请在手机手动进入「安全 → 安装证书 → CA 证书」")
}
