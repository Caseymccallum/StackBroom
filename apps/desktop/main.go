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
	desktopService, processService, err := NewApplicationServices()
	if err != nil {
		log.Fatalf("cannot initialise process-control security: %v", err)
	}
	app := application.New(application.Options{
		Name:        "StackBroom",
		Description: "PC understanding, maintenance and verified process control",
		Services: []application.Service{
			application.NewService(desktopService),
			application.NewService(processService),
			application.NewService(NewStorageService()),
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
