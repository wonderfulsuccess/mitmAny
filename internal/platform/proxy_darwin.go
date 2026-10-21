package platform

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type proxySetting struct {
	Enabled       bool
	Server        string
	Port          int
	Authenticated bool
}
type serviceBackup struct {
	Name  string
	HTTP  proxySetting
	HTTPS proxySetting
}
type proxyBackup struct{ Services []serviceBackup }

func network(args ...string) (string, error) {
	out, err := exec.Command("/usr/sbin/networksetup", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("networksetup: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	if strings.Contains(string(out), "Error") {
		return "", fmt.Errorf("networksetup: %s", out)
	}
	return string(out), nil
}
func services() ([]string, error) {
	out, err := network("-listallnetworkservices")
	if err != nil {
		return nil, err
	}
	var list []string
	for i, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if i == 0 || strings.HasPrefix(line, "*") {
			continue
		}
		if line != "" {
			list = append(list, line)
		}
	}
	return list, nil
}
func setting(name, kind string) (proxySetting, error) {
	out, err := network("-get"+kind+"proxy", name)
	var s proxySetting
	if err != nil {
		return s, err
	}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "Enabled":
			s.Enabled = parts[1] == "Yes"
		case "Server":
			s.Server = parts[1]
		case "Port":
			s.Port, _ = strconv.Atoi(parts[1])
		case "Authenticated Proxy Enabled":
			s.Authenticated = parts[1] == "1"
		}
	}
	return s, nil
}
func readProxy() (proxyBackup, error) {
	names, err := services()
	b := proxyBackup{}
	if err != nil {
		return b, err
	}
	for _, name := range names {
		h, err := setting(name, "web")
		if err != nil {
			return b, err
		}
		s, err := setting(name, "secureweb")
		if err != nil {
			return b, err
		}
		if h.Authenticated || s.Authenticated {
			return b, fmt.Errorf("网络服务 %s 使用需要认证的代理，首版不覆盖该配置", name)
		}
		b.Services = append(b.Services, serviceBackup{name, h, s})
	}
	return b, nil
}
func quoteArg(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

// Try normal authorization first; macOS may require its native administrator dialog.
func executeChanges(commands [][]string) error {
	needsAuthorization := false
	for _, args := range commands {
		if _, err := network(args...); err != nil {
			needsAuthorization = true
			break
		}
	}
	if !needsAuthorization {
		return nil
	}
	var lines []string
	for _, args := range commands {
		var quoted []string
		for _, arg := range args {
			quoted = append(quoted, quoteArg(arg))
		}
		lines = append(lines, "/usr/sbin/networksetup "+strings.Join(quoted, " "))
	}
	command := "set -e\n" + strings.Join(lines, "\n")
	script := "do shell script " + strconv.Quote(command) + " with administrator privileges"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("系统代理授权失败: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}
func applyProxy(host string, port int) error {
	names, err := services()
	if err != nil {
		return err
	}
	var commands [][]string
	for _, name := range names {
		for _, kind := range []string{"web", "secureweb"} {
			commands = append(commands, []string{"-set" + kind + "proxy", name, host, strconv.Itoa(port)}, []string{"-set" + kind + "proxystate", name, "on"})
		}
	}
	return executeChanges(commands)
}
func restoreProxy(b proxyBackup) error {
	var commands [][]string
	for _, svc := range b.Services {
		for i, kind := range []string{"web", "secureweb"} {
			s := svc.HTTP
			if i == 1 {
				s = svc.HTTPS
			}
			if s.Server != "" && s.Port > 0 {
				commands = append(commands, []string{"-set" + kind + "proxy", svc.Name, s.Server, strconv.Itoa(s.Port)})
			}
			state := "off"
			if s.Enabled {
				state = "on"
			}
			commands = append(commands, []string{"-set" + kind + "proxystate", svc.Name, state})
		}
	}
	return executeChanges(commands)
}

// Prefer a single consistent configured proxy. Conflicting service proxies need
// explicit user configuration rather than silently changing network routing.
func upstreamForBackup(b proxyBackup) (UpstreamConfig, error) {
	c := UpstreamConfig{}
	for _, service := range b.Services {
		for i, s := range []proxySetting{service.HTTP, service.HTTPS} {
			if !s.Enabled {
				continue
			}
			address := proxyAddress(s.Server, s.Port)
			target := &c.HTTP
			if i == 1 {
				target = &c.HTTPS
			}
			if *target != "" && *target != address {
				return c, fmt.Errorf("不同网络服务使用不同的系统代理，首版无法安全自动串联；请使用手动代理模式")
			}
			*target = address
		}
	}
	return c, nil
}
