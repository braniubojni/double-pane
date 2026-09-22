package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/erikharutyunyan/double-pane/internal/config"
	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
	"github.com/erikharutyunyan/double-pane/internal/remote"
	"github.com/erikharutyunyan/double-pane/internal/service"
	"github.com/erikharutyunyan/double-pane/internal/storage"
	"github.com/erikharutyunyan/double-pane/internal/version"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cfgStore, err := config.Open(domain.AppName)
	if err != nil {
		log.Fatal(err)
	}

	db, err := storage.Open(domain.AppName)
	if err != nil {
		log.Fatal(err)
	}

	remoteMgr := service.NewRemoteManager(db)
	smbMgr := remote.NewSMBManager()
	megaMgr := remote.NewMEGAManager()
	fileSvc := service.NewFileService(remoteMgr, smbMgr, megaMgr, filepath.Join(cfgStore.Dir(), "trash"))
	if os.Getenv(config.EnvConfigDir) == "" {
		fileSvc.SetTrashBackend(filesystem.NewPlatformTrash())
	}
	if err := fileSvc.PurgeDupCache(); err != nil {
		log.Printf("purge dup-cache: %v", err)
	}
	settingsSvc := service.NewSettingsService(db, cfgStore)
	bookmarkSvc := service.NewBookmarkService(db)
	termSvc := service.NewTerminalService(remoteMgr)
	connSvc := service.NewConnectionService(db, remoteMgr, smbMgr, megaMgr)
	updateSvc := service.NewUpdateService()
	gitSvc := service.NewGitService()
	portSvc := service.NewPortService()
	aiUsageSvc := service.NewAIUsageService()

	app := application.New(application.Options{
		Name:        "Double Pane",
		Description: "Dual-pane file manager (Double Commander style)",
		Services: []application.Service{
			application.NewService(fileSvc),
			application.NewService(settingsSvc),
			application.NewService(bookmarkSvc),
			application.NewService(termSvc),
			application.NewService(connSvc),
			application.NewService(updateSvc),
			application.NewService(gitSvc),
			application.NewService(portSvc),
			application.NewService(aiUsageSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		// Used when built with -tags server (e2e / headless).
		Server: application.ServerOptions{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	gh, err := github.New(github.Config{
		Repository:    "braniubojni/double-pane",
		ChecksumAsset: "SHA256SUMS",
	})
	if err != nil {
		log.Fatalf("github updater provider: %v", err)
	}
	if err := app.Updater.Init(updater.Config{
		CurrentVersion: service.CurrentVersionForUpdater(version.Version),
		Providers:      []updater.Provider{gh},
	}); err != nil {
		log.Fatalf("Updater.Init: %v", err)
	}

	termSvc.SetApp(app)
	fileSvc.SetApp(app)
	service.AttachUpdateApp(updateSvc, app)

	app.OnShutdown(func() {
		_ = termSvc.Stop("left")
		_ = termSvc.Stop("right")
		remoteMgr.CloseAll()
		smbMgr.CloseAll()
		megaMgr.CloseAll()
		_ = db.Close()
	})

	winW, winH := loadWindowSize(settingsSvc)
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "Double Pane",
		Width:          winW,
		Height:         winH,
		MinWidth:       domain.MinWindowWidth,
		MinHeight:      domain.MinWindowHeight,
		EnableFileDrop: true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 0,
			Backdrop:                application.MacBackdropNormal,
			TitleBar:                application.MacTitleBarDefault,
		},
		BackgroundColour: application.NewRGB(18, 18, 18),
		URL:              "/",
	})

	// OS file manager → app (Finder / Explorer / Linux FMs). Frontend marks
	// panes and folder rows with data-file-drop-target; drop is forwarded as
	// a custom event so the UI can copy into the resolved destination.
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		files := event.Context().DroppedFiles()
		if len(files) == 0 {
			return
		}
		payload := map[string]any{"files": files}
		if details := event.Context().DropTargetDetails(); details != nil {
			payload["target"] = map[string]any{
				"id":         details.ElementID,
				"classList":  details.ClassList,
				"attributes": details.Attributes,
				"x":          details.X,
				"y":          details.Y,
			}
		}
		app.Event.Emit("files-dropped", payload)
	})
	attachWindowSizePersistence(win, settingsSvc)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
