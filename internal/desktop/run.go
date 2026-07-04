package desktop

import (
	"context"
	"embed"

	csnative "csnative/internal/app"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func Run(appIcon ...[]byte) error {
	app := csnative.New()
	var icon []byte
	if len(appIcon) > 0 {
		icon = appIcon[0]
	}
	return wails.Run(&options.App{
		Title:     "CS Native",
		Width:     960,
		Height:    832,
		MinWidth:  760,
		MinHeight: 620,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(context.Context) {
			setDockIcon(icon)
			go func() { _ = app.RestoreProxy() }()
		},
		Mac: &mac.Options{
			About: &mac.AboutInfo{
				Title:   "CS Native",
				Message: "Provider/runtime control plane by eust-w",
				Icon:    icon,
			},
		},
		Bind: []interface{}{app},
	})
}
