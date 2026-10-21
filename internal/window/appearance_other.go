//go:build !darwin || !cgo

package window

// Windows 11 uses Wails' native DWM frameless-window decorations.
func Round() {}
