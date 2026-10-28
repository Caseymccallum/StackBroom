package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The production executable serves these embedded assets through Wails. No
// network listener is opened by the packaged application.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "StackBroom",
		Description: "Read-only PC health, resource and developer dashboard",
		Services: []application.Service{
			application.NewService(NewDesktopService()),
			application.NewService(NewProcessControlService()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	preview := os.Getenv("STACKBROOM_PREVIEW") == "1"
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "StackBroom",
		Width:            1120,
		Height:           720,
		MinWidth:         900,
		MinHeight:        560,
		Frameless:        preview,
		BackgroundColour: application.NewRGB(9, 13, 18),
		URL:              "/",
		DevToolsEnabled:  false,
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
