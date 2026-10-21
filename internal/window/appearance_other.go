//go:build !darwin || !cgo

package window

import "context"

// Other platforms use their native Wails window presentation.
func Ready(context.Context) {}
