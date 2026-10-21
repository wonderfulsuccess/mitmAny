package platform

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
	"strings"
	"syscall"
	"unsafe"
)

const internetSettings = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

type registryValue struct {
	Exists bool
	Type   uint32
	Data   []byte
}
type proxyBackup struct{ Values map[string]registryValue }

func readProxy() (proxyBackup, error) {
	b := proxyBackup{Values: map[string]registryValue{}}
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettings, registry.QUERY_VALUE)
	if err != nil {
		return b, err
	}
	defer k.Close()
	for _, name := range []string{"ProxyEnable", "ProxyServer", "ProxyOverride"} {
		n, typ, err := k.GetValue(name, nil)
		if err == registry.ErrNotExist {
			b.Values[name] = registryValue{}
			continue
		}
		if err != nil {
			return b, err
		}
		data := make([]byte, n)
		_, _, err = k.GetValue(name, data)
		if err != nil {
			return b, err
		}
		b.Values[name] = registryValue{true, typ, data}
	}
	return b, nil
}
func notify() error {
	dll := syscall.NewLazyDLL("wininet.dll")
	f := dll.NewProc("InternetSetOptionW")
	for _, option := range []uintptr{39, 37} {
		ok, _, err := f.Call(0, option, 0, 0)
		if ok == 0 {
			return err
		}
	}
	return nil
}
func applyProxy(host string, port int) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettings, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err = k.SetStringValue("ProxyServer", fmt.Sprintf("http=%s:%d;https=%s:%d", host, port, host, port)); err != nil {
		return err
	}
	if err = k.SetDWordValue("ProxyEnable", 1); err != nil {
		return err
	}
	return notify()
}
func restoreProxy(b proxyBackup) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettings, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	for name, v := range b.Values {
		if !v.Exists {
			err = k.DeleteValue(name)
			if err == registry.ErrNotExist {
				err = nil
			}
		} else {
			var ptr *byte
			if len(v.Data) > 0 {
				ptr = &v.Data[0]
			}
			p, _ := syscall.UTF16PtrFromString(name)
			proc := syscall.NewLazyDLL("advapi32.dll").NewProc("RegSetValueExW")
			result, _, _ := proc.Call(uintptr(k), uintptr(unsafe.Pointer(p)), 0, uintptr(v.Type), uintptr(unsafe.Pointer(ptr)), uintptr(len(v.Data)))
			if result != 0 {
				err = syscall.Errno(result)
			}
		}
		if err != nil {
			return err
		}
	}
	return notify()
}

func upstreamForBackup(b proxyBackup) (UpstreamConfig, error) {
	enabled := b.Values["ProxyEnable"]
	if len(enabled.Data) < 4 || enabled.Data[0] == 0 {
		return UpstreamConfig{}, nil
	}
	value := b.Values["ProxyServer"]
	raw := make([]uint16, 0, len(value.Data)/2)
	for i := 0; i+1 < len(value.Data); i += 2 {
		raw = append(raw, uint16(value.Data[i])|uint16(value.Data[i+1])<<8)
	}
	server := syscall.UTF16ToString(raw)
	c := UpstreamConfig{}
	if !strings.Contains(server, "=") {
		c.HTTP = server
		c.HTTPS = server
		return c, nil
	}
	for _, part := range strings.Split(server, ";") {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) != 2 {
			continue
		}
		switch strings.ToLower(pair[0]) {
		case "http":
			c.HTTP = pair[1]
		case "https":
			c.HTTPS = pair[1]
		}
	}
	return c, nil
}
