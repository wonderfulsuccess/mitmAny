package certificates

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func privileged(command string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	script := "do shell script " + strconv.Quote(command) + " with administrator privileges"
	out, err := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("系统授权或证书操作失败: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}
func checkTrust(m *Manager) error {
	out, err := exec.Command("/usr/bin/security", "verify-cert", "-c", m.Path(), "-p", "basic").CombinedOutput()
	if err != nil {
		return fmt.Errorf("证书尚未受系统信任: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
func install(m *Manager) error {
	return nativeCertificate(m.Root().Raw, false)
}
func uninstall(m *Manager) error {
	if err := nativeCertificate(m.Root().Raw, true); err != nil {
		return err
	}
	// Earlier builds may have left an untrusted copy in the System keychain.
	out, err := exec.Command("/usr/bin/security", "find-certificate", "-a", "-Z", "/Library/Keychains/System.keychain").Output()
	if err != nil {
		return fmt.Errorf("当前用户证书已卸载，但无法检查系统钥匙串中的旧副本：%w", err)
	}
	if strings.Contains(strings.ToUpper(string(out)), strings.ToUpper(m.SHA1())) {
		return privileged("/usr/bin/security delete-certificate -Z " + shellQuote(m.SHA1()) + " /Library/Keychains/System.keychain")
	}
	return nil
}
