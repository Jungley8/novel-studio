package desktop

import (
	"net/http"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// DesktopOptions configures the Wails v3 native desktop window.
type DesktopOptions struct {
	Title     string
	Width     int
	Height    int
	MinWidth  int
	MinHeight int
	Handler   http.Handler
	OnExit    func()
}

// RunDesktop starts the native desktop application using Wails v3.
// It blocks until the user closes the window or quits the application.
func RunDesktop(opts DesktopOptions) error {
	if opts.Width <= 0 {
		opts.Width = 1440
	}
	if opts.Height <= 0 {
		opts.Height = 900
	}
	if opts.MinWidth <= 0 {
		opts.MinWidth = 1024
	}
	if opts.MinHeight <= 0 {
		opts.MinHeight = 700
	}
	if opts.Title == "" {
		opts.Title = "NovelStudio (故事工厂)"
	}

	app := application.New(application.Options{
		Name:        "NovelStudio",
		Description: "工业级 AI 网络小说创作桌面工作台",
		Assets: application.AssetOptions{
			Handler: opts.Handler,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     opts.Title,
		Width:     opts.Width,
		Height:    opts.Height,
		MinWidth:  opts.MinWidth,
		MinHeight: opts.MinHeight,
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTranslucent,
			TitleBar: application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})

	err := app.Run()
	if opts.OnExit != nil {
		opts.OnExit()
	}
	return err
}
