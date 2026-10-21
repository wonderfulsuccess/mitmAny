package capture

import (
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
	"net"
	"strings"
)

// DomainNames uses the public suffix list, including multi-label and private suffixes.
func DomainNames(host string) (domain, root string) {
	host = strings.TrimSpace(host)
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	host = strings.TrimRight(strings.Trim(host, "[]"), ".")
	host = strings.ToLower(host)
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), ip.String()
	}
	if ascii, err := idna.Lookup.ToASCII(host); err == nil {
		host = ascii
	}
	root, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		root = host
	}
	return host, root
}
