package codex

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func fixture(t *testing.T) *Installer {
	t.Helper()
	home := t.TempDir()
	exe := filepath.Join(home, "App with spaces", "mitmAny")
	if err := os.MkdirAll(filepath.Dir(exe), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("test executable"), 0700); err != nil {
		t.Fatal(err)
	}
	return &Installer{Home: home, ConfigHome: filepath.Join(home, "custom-codex"), Executable: exe, DataDir: filepath.Join(home, "Capture Data"), Skill: []byte("---\nname: mitmany-network-analysis\ndescription: test\n---\nTest skill\n")}
}
func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestInstallPreservesConfigAndIsIdempotent(t *testing.T) {
	i := fixture(t)
	skill, config := i.paths()
	original := []byte("# user's comment\nmodel = 'example'\n\n[mcp_servers.other]\ncommand = 'keep me' # comment\n\n[mcp_servers.mitmany]\ncommand = '/old'\nargs = []\n[mcp_servers.mitmany.env]\nOLD = 'old value'\n\n[features]\nfoo = true\n")
	write(t, config, original)
	write(t, skill, []byte("personal changes"))
	s := i.Install()
	if !s.Ready || s.Error != "" {
		t.Fatal(s)
	}
	result, _ := os.ReadFile(config)
	if !bytes.Contains(result, []byte("# user's comment")) || !bytes.Contains(result, []byte("command = 'keep me' # comment")) {
		t.Fatal("unrelated formatting lost")
	}
	parsed, e := parse(result)
	if e != nil {
		t.Fatal(e)
	}
	entry := table(table(parsed, "mcp_servers"), "mitmany")
	if !reflect.DeepEqual(entry, i.entry()) {
		t.Fatal(entry)
	}
	configBackups, _ := filepath.Glob(config + ".mitmany-*.bak")
	skillBackups, _ := filepath.Glob(skill + ".mitmany-*.bak")
	if len(configBackups) != 1 || len(skillBackups) != 1 {
		t.Fatal("missing backups")
	}
	backupData, _ := os.ReadFile(configBackups[0])
	if !bytes.Equal(backupData, original) {
		t.Fatal("wrong backup")
	}
	if !i.Install().Ready {
		t.Fatal("repeat install failed")
	}
	again, _ := os.ReadFile(config)
	if !bytes.Equal(result, again) {
		t.Fatal("non-idempotent config")
	}
	configBackups, _ = filepath.Glob(config + ".mitmany-*.bak")
	if len(configBackups) != 1 {
		t.Fatal("unnecessary backup")
	}
	i.Executable = filepath.Join(i.Home, "Moved App")
	write(t, i.Executable, []byte("new"))
	if i.Status().MCPRegistered {
		t.Fatal("moved app reported ready")
	}
	if !i.Install().Ready {
		t.Fatal("move repair failed")
	}
}
func TestReplaceServerTOMLForms(t *testing.T) {
	i := fixture(t)
	samples := []string{
		"", "model = 'x'\n",
		"[mcp_servers]\nother = {command='keep'}\nmitmany = {command='old'}\n",
		"mcp_servers.mitmany.command = 'old'\nmcp_servers.other.command = 'keep'\n",
		"[mcp_servers.'mitmany']\ncommand = '''line one\n[mcp_servers.fake]\nline three'''\nargs = [\n'old',\n]\n[features]\nx=true\n",
		"[mcp_servers.other]\ncommand='keep'\n",
		"[mcp_servers.mitmany.env]\nX='y'\n[mcp_servers.mitmany]\ncommand='old'\n",
	}
	for _, input := range samples {
		t.Run(input, func(t *testing.T) {
			output, err := replaceServer([]byte(input), i.entry())
			if err != nil {
				t.Fatal(err)
			}
			m, err := parse(output)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(table(table(m, "mcp_servers"), "mitmany"), i.entry()) {
				t.Fatal("incorrect entry")
			}
		})
	}
	windows := map[string]any{"command": `C:\Program Files\mitmAny\mitmAny.exe`, "args": []any{"--mcp", "--data-dir", `C:\Users\测试\AppData\Roaming\mitmAny`}, "enabled": true}
	out, err := replaceServer(nil, windows)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := parse(out)
	if !reflect.DeepEqual(table(table(m, "mcp_servers"), "mitmany"), windows) {
		t.Fatal("windows escaping")
	}
}
func TestFailedInstallDoesNotChangeConfig(t *testing.T) {
	for _, input := range []string{"broken = [", "mcp_servers = {other={command='keep'}}"} {
		i := fixture(t)
		skill, config := i.paths()
		write(t, config, []byte(input))
		s := i.Install()
		if s.Error == "" || s.Ready {
			t.Fatal(s)
		}
		after, _ := os.ReadFile(config)
		if string(after) != input {
			t.Fatal("changed failed config")
		}
		if _, e := os.Stat(skill); !os.IsNotExist(e) {
			t.Fatal("installed skill despite preflight failure")
		}
	}
	i := fixture(t)
	_, config := i.paths()
	write(t, config, []byte("[[skills.config]]\nname='mitmany-network-analysis'\nenabled=false\n"))
	s := i.Install()
	if !s.SkillDisabled || s.Error == "" || s.Ready {
		t.Fatal(s)
	}
}
func TestCustomHomeAndSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require platform permissions")
	}
	i := fixture(t)
	_, config := i.paths()
	real := filepath.Join(i.Home, "dotfiles", "codex.toml")
	write(t, real, []byte("# linked config\n"))
	os.MkdirAll(filepath.Dir(config), 0700)
	if e := os.Symlink(real, config); e != nil {
		t.Fatal(e)
	}
	if s := i.Install(); !s.Ready {
		t.Fatal(s)
	}
	st, _ := os.Lstat(config)
	if st.Mode()&os.ModeSymlink == 0 {
		t.Fatal("replaced user's symlink")
	}
}
func TestCodexCLIRecognizesInstalledEntry(t *testing.T) {
	cli, err := exec.LookPath("codex")
	if err != nil {
		t.Skip("CLI optional")
	}
	i := fixture(t)
	if s := i.Install(); !s.Ready {
		t.Fatal(s)
	}
	cmd := exec.Command(cli, "mcp", "get", "mitmany", "--json")
	env := []string{}
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "CODEX_HOME=") {
			env = append(env, v)
		}
	}
	cmd.Env = append(env, "CODEX_HOME="+i.ConfigHome)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI rejected generated config: %v %s", err, out)
	}
	if !bytes.Contains(out, []byte(`"enabled": true`)) || !bytes.Contains(out, []byte(`--mcp`)) {
		t.Fatal("CLI did not recognize MCP entry")
	}
}
