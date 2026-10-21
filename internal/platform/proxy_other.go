//go:build !darwin && !windows

package platform

import "errors"

type proxyBackup struct{}

func readProxy() (proxyBackup, error) {
	return proxyBackup{}, errors.New("system proxy requires macOS or Windows")
}
func applyProxy(string, int) error   { return errors.New("unsupported platform") }
func restoreProxy(proxyBackup) error { return errors.New("unsupported platform") }

func upstreamForBackup(proxyBackup) (UpstreamConfig, error) { return UpstreamConfig{}, nil }
