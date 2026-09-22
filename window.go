package main

import (
	"sync"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/service"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const windowSizeSaveDelay = 400 * time.Millisecond

func loadWindowSize(settingsSvc *service.SettingsService) (width, height int) {
	st, err := settingsSvc.GetWindowState()
	if err != nil {
		return domain.DefaultWindowWidth, domain.DefaultWindowHeight
	}
	return st.Width, st.Height
}

func persistWindowSize(win *application.WebviewWindow, settingsSvc *service.SettingsService) {
	w, h := win.Size()
	if w < domain.MinWindowWidth || h < domain.MinWindowHeight {
		return
	}
	_ = settingsSvc.SaveWindowState(domain.WindowState{Width: w, Height: h})
}

func attachWindowSizePersistence(win *application.WebviewWindow, settingsSvc *service.SettingsService) {
	var mu sync.Mutex
	var timer *time.Timer
	save := func() { persistWindowSize(win, settingsSvc) }

	win.OnWindowEvent(events.Common.WindowDidResize, func(_ *application.WindowEvent) {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(windowSizeSaveDelay, save)
	})
	win.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		mu.Lock()
		if timer != nil {
			timer.Stop()
			timer = nil
		}
		mu.Unlock()
		save()
	})
}
