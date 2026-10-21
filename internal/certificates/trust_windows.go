package certificates

import (
	"fmt"
	"os/exec"
	"strings"
)

func checkTrust(m *Manager) error {
	script := "if (!(Test-Path 'Cert:\\CurrentUser\\Root\\" + strings.ToUpper(m.SHA1()) + "')) { exit 1 }"
	if err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Run(); err != nil {
		return fmt.Errorf("证书尚未安装到当前用户受信任根证书存储")
	}
	return nil
}
func install(m *Manager) error {
	out, err := exec.Command("certutil.exe", "-user", "-addstore", "Root", m.Path()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	return nil
}
func uninstall(m *Manager) error {
	out, err := exec.Command("certutil.exe", "-user", "-delstore", "Root", m.SHA1()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	return nil
}
