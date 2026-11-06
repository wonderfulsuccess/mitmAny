package main

import (
	"embed"
	"fmt"
	"mitmAny/internal/agent"
	"os"
	"runtime"

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
	if len(os.Args) > 1 && os.Args[1] == "--mcp" {
		if err := agent.RunStdio(os.Args[2:], os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	app := NewApp()
	err := wails.Run(&options.App{
		Title: "mitmAny", Width: 1440, Height: 920, MinWidth: 480, MinHeight: 560,
		Frameless:        runtime.GOOS != "darwin",
		StartHidden:      runtime.GOOS == "darwin",
		BackgroundColour: options.NewRGB(249, 250, 252),
		Mac:              &mac.Options{TitleBar: mac.TitleBarHidden(), WebviewIsTransparent: true},
		Windows:          &windows.Options{DisableFramelessWindowDecorations: false},
		OnDomReady:       window.Ready,
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup, OnBeforeClose: app.beforeClose, OnShutdown: app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{UniqueId: "mitmAny-9fa32128-4202-4d61-a034-9b34470d4466"},
		Bind:               []interface{}{app},
	})
	if err != nil {
		println(err.Error())
	}
}
