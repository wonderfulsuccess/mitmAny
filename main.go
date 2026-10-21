package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"mitmAny/internal/window"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title: "mitmAny", Width: 1440, Height: 920, MinWidth: 480, MinHeight: 560,
		Frameless:        true,
		BackgroundColour: options.NewRGB(249, 250, 252),
		Mac:              &mac.Options{WebviewIsTransparent: true},
		Windows:          &windows.Options{DisableFramelessWindowDecorations: false},
		OnDomReady:       func(context.Context) { window.Round() },
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup, OnBeforeClose: app.beforeClose, OnShutdown: app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{UniqueId: "mitmAny-9fa32128-4202-4d61-a034-9b34470d4466"},
		Bind:               []interface{}{app},
	})
	if err != nil {
		println(err.Error())
	}
}
