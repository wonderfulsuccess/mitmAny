// Package codex installs the bundled skill and only the mitmany MCP entry.
// It does not require a CLI, invoke a shell, or download executable code.
package codex

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
)

const SkillName = "mitmany-network-analysis"

var installMu sync.Mutex

type Installer struct {
	Home, ConfigHome, Executable, DataDir string
	Skill                                 []byte
}
type Status struct {
	SkillInstalled bool   `json:"skillInstalled"`
	MCPRegistered  bool   `json:"mcpRegistered"`
	SkillDisabled  bool   `json:"skillDisabled"`
	Ready          bool   `json:"ready"`
	ServiceRunning bool   `json:"serviceRunning"`
	LastActivity   int64  `json:"lastActivity"`
	SkillPath      string `json:"skillPath"`
	ConfigPath     string `json:"configPath"`
	Error          string `json:"error"`
}

func New(skill []byte, dataDir string) (*Installer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	configHome := os.Getenv("CODEX_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".codex")
	}
	configHome, err = filepath.Abs(configHome)
	if err != nil {
		return nil, err
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return nil, err
	}
	return &Installer{Home: home, ConfigHome: configHome, Executable: executable, DataDir: dataDir, Skill: skill}, nil
}
func (i *Installer) paths() (string, string) {
	return filepath.Join(i.Home, ".agents", "skills", SkillName, "SKILL.md"), filepath.Join(i.ConfigHome, "config.toml")
}
func (i *Installer) entry() map[string]any {
	return map[string]any{"command": i.Executable, "args": []any{"--mcp", "--data-dir", i.DataDir}, "enabled": true}
}
func readOptional(path string) ([]byte, error) {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return nil, nil
	}
	return b, e
}
func (i *Installer) Status() Status { installMu.Lock(); defer installMu.Unlock(); return i.status() }
func (i *Installer) status() Status {
	skillPath, configPath := i.paths()
	s := Status{SkillPath: skillPath, ConfigPath: configPath}
	skill, err := readOptional(skillPath)
	if err != nil {
		s.Error = "读取 skill: " + err.Error()
		return s
	}
	s.SkillInstalled = bytes.Equal(skill, i.Skill) && len(skill) > 0
	data, err := readOptional(configPath)
	if err != nil {
		s.Error = "读取 Codex 配置: " + err.Error()
		return s
	}
	config, err := parse(data)
	if err != nil {
		s.Error = "Codex config.toml 格式无效，请先修复配置文件"
		return s
	}
	entry := table(table(config, "mcp_servers"), "mitmany")
	enabled, _ := entry["enabled"].(bool)
	if _, ok := entry["enabled"]; !ok {
		enabled = true
	}
	s.MCPRegistered = entry["command"] == i.Executable && reflect.DeepEqual(entry["args"], i.entry()["args"]) && enabled && entry["url"] == nil
	// A locally disabled skill remains disabled; do not silently bypass that choice.
	if rules, ok := table(config, "skills")["config"].([]any); ok {
		for _, rule := range rules {
			r, _ := rule.(map[string]any)
			path, _ := r["path"].(string)
			if r["name"] == SkillName || (path != "" && filepath.Clean(path) == filepath.Clean(skillPath)) {
				if on, ok := r["enabled"].(bool); ok {
					s.SkillDisabled = !on
				}
			}
		}
	}
	s.Ready = s.SkillInstalled && s.MCPRegistered && !s.SkillDisabled
	if s.SkillDisabled {
		s.Error = "此 skill 在 Codex 配置中被禁用，请先在 Codex 中启用"
	}
	return s
}
func (i *Installer) Install() Status {
	installMu.Lock()
	defer installMu.Unlock()
	fail := func(err error) Status { s := i.status(); s.Error = err.Error(); return s }
	if len(i.Skill) == 0 {
		return fail(errors.New("安装包缺少 skill"))
	}
	if !filepath.IsAbs(i.Executable) || !filepath.IsAbs(i.DataDir) {
		return fail(errors.New("应用或数据目录不是绝对路径"))
	}
	if strings.Contains(i.Executable, "/AppTranslocation/") || strings.HasPrefix(i.Executable, "/Volumes/") {
		return fail(errors.New("请先把 mitmAny 移到应用程序目录并重新打开，再连接 Codex"))
	}
	if info, err := os.Stat(i.Executable); err != nil || info.IsDir() {
		return fail(errors.New("找不到当前应用程序，请重新安装 mitmAny"))
	}
	skillPath, configPath := i.paths()
	// Resolve an existing config symlink instead of replacing the link itself.
	resolved, err := filepath.EvalSymlinks(configPath)
	if err == nil {
		configPath = resolved
	} else if !os.IsNotExist(err) {
		return fail(err)
	}
	original, err := readOptional(configPath)
	if err != nil {
		return fail(err)
	}
	current := i.status()
	if current.Error != "" {
		return current
	}
	if current.Ready {
		return current
	}
	updated := original
	if !current.MCPRegistered {
		updated, err = replaceServer(original, i.entry())
		if err != nil {
			return fail(err)
		}
	}
	if err = os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return fail(err)
	}
	lock, err := os.OpenFile(configPath+".mitmany.lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fail(errors.New("另一个安装操作正在进行，或配置目录不可写；请稍后重试"))
	}
	lock.Close()
	defer os.Remove(configPath + ".mitmany.lock")
	// Stage both files before changing either; keep backups when replacing content.
	if err = os.MkdirAll(filepath.Dir(skillPath), 0700); err != nil {
		return fail(err)
	}
	if st, e := os.Lstat(skillPath); e == nil && st.Mode()&os.ModeSymlink != 0 {
		return fail(errors.New("skill 文件是符号链接，请先在 Codex 中调整此 skill 的安装位置"))
	}
	oldSkill, err := readOptional(skillPath)
	if err != nil {
		return fail(err)
	}
	if !bytes.Equal(oldSkill, i.Skill) {
		if err = backup(skillPath, oldSkill); err != nil {
			return fail(err)
		}
	}
	if !bytes.Equal(original, updated) {
		if err = backup(configPath, original); err != nil {
			return fail(err)
		}
	}
	skillTemp, err := stage(skillPath, i.Skill)
	if err != nil {
		return fail(err)
	}
	defer os.Remove(skillTemp)
	configTemp, err := stage(configPath, updated)
	if err != nil {
		return fail(err)
	}
	defer os.Remove(configTemp)
	latest, err := readOptional(configPath)
	if err != nil || !bytes.Equal(latest, original) {
		return fail(errors.New("Codex 配置在安装期间发生变化，请重新点击连接"))
	}
	if !bytes.Equal(oldSkill, i.Skill) {
		if err = os.Rename(skillTemp, skillPath); err != nil {
			return fail(err)
		}
	}
	if !bytes.Equal(original, updated) {
		// Check again after skill installation; never roll back another writer's config.
		latest, err = readOptional(configPath)
		if err != nil || !bytes.Equal(latest, original) {
			return fail(errors.New("skill 已安装，但 Codex 配置被其他程序修改；请重新连接以完成 MCP 注册"))
		}
		if err = os.Rename(configTemp, configPath); err != nil {
			return fail(fmt.Errorf("skill 已安装，但 MCP 配置写入失败：%w", err))
		}
	}
	return i.status()
}
func backup(path string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".mitmany-"+time.Now().Format("20060102-150405")+"-*.bak")
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func stage(path string, data []byte) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".mitmany-install-*")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}
