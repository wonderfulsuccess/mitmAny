package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ProxyManager struct{ path string }

func NewProxyManager(dir string) *ProxyManager {
	return &ProxyManager{path: filepath.Join(dir, "proxy-backup.json")}
}
func (p *ProxyManager) Pending() bool { _, err := os.Stat(p.path); return err == nil }
func (p *ProxyManager) Enable(host string, port int) error {
	if p.Pending() {
		return fmt.Errorf("发现未恢复的系统代理，请先恢复网络配置")
	}
	backup, err := readProxy()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(backup)
	if err != nil {
		return err
	}
	if err = os.WriteFile(p.path, encoded, 0600); err != nil {
		return err
	}
	if err = applyProxy(host, port); err != nil {
		restoreErr := p.Restore()
		if restoreErr != nil {
			return fmt.Errorf("启用失败: %v；恢复失败: %w", err, restoreErr)
		}
		return err
	}
	return nil
}
func (p *ProxyManager) Restore() error {
	data, err := os.ReadFile(p.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var backup proxyBackup
	if err = json.Unmarshal(data, &backup); err != nil {
		return err
	}
	if err = restoreProxy(backup); err != nil {
		return err
	}
	return os.Remove(p.path)
}
