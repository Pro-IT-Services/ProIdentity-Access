package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "ProIdentity Access",
		Width:            960,
		Height:           660,
		MinWidth:         800,
		MinHeight:        580,
		DisableResize:    false,
		Fullscreen:       false,
		WindowStartState: options.Normal,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:  &options.RGBA{R: 10, G: 12, B: 18, A: 255},
		OnStartup:         app.startup,
		OnDomReady:        app.domReady,
		OnShutdown:        app.shutdown,
		HideWindowOnClose: true,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.proidentity.access.client",
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				// Re-launch attempt — bring the existing window forward.
				if app.ctx == nil {
					return
				}
				runtime.WindowShow(app.ctx)
				runtime.WindowUnminimise(app.ctx)
				runtime.Show(app.ctx)
			},
		},
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			IsZoomControlEnabled: false,
			EnableSwipeGestures:  false,
			WebviewUserDataPath:  "",
			Theme:                windows.Dark,
			CustomTheme: &windows.ThemeSettings{
				DarkModeTitleBar:   windows.RGB(10, 12, 18),
				DarkModeTitleText:  windows.RGB(201, 209, 217),
				DarkModeBorder:     windows.RGB(30, 36, 43),
				LightModeTitleBar:  windows.RGB(248, 249, 250),
				LightModeTitleText: windows.RGB(31, 35, 40),
				LightModeBorder:    windows.RGB(208, 215, 222),
			},
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "ProIdentity Access",
				Message: "VPN Client",
			},
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
