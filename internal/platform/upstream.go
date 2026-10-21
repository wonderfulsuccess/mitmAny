package platform

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type UpstreamConfig struct {
	HTTP  string
	HTTPS string
}

func (p *ProxyManager) Upstream() (UpstreamConfig, error) {
	backup, err := readProxy()
	if err != nil {
		return UpstreamConfig{}, err
	}
	return upstreamForBackup(backup)
}
func (c *UpstreamConfig) Resolve(req *http.Request) (*url.URL, error) {
	if c == nil {
		return nil, nil
	}
	address := c.HTTP
	if req.Method == http.MethodConnect || req.URL.Scheme == "https" {
		address = c.HTTPS
	}
	if address == "" {
		return nil, nil
	}
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	u, err := url.Parse(address)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("原系统代理地址无效")
	}
	if (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1") && u.Port() == "9080" {
		return nil, fmt.Errorf("原代理指向 mitmAny 端口，拒绝产生代理循环")
	}
	return u, nil
}
func proxyAddress(host string, port int) string { return net.JoinHostPort(host, strconv.Itoa(port)) }
