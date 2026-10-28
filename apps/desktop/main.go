package main

import (
	"embed"
	"log"
	"os"

	"github.com/Caseymccallum/StackBroom/internal/processcontrol"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// The production executable serves these embedded assets through Wails. No
// network listener is opened by the packaged application.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	tokens, _ := processcontrol.NewTokenIssuer()
	app := application.New(application.Options{
		Name:        "StackBroom",
		Description: "PC understanding, maintenance and verified process control",
		Services: []application.Service{
			application.NewService(NewDesktopServiceWithTokens(tokens)),
			application.NewService(NewProcessControlServiceWithTokens(tokens)),
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
