//go:build !darwin && !windows

package certificates

import "errors"

func checkTrust(*Manager) error { return errors.New("unsupported platform") }
func install(*Manager) error    { return errors.New("unsupported platform") }
func uninstall(*Manager) error  { return errors.New("unsupported platform") }
