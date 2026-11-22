package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"wowcensuscompanion/companion"
)

type trayStrings struct {
	open   string
	update string
	quit   string
}

var trayTranslations = map[string]trayStrings{
	"en": {
		open:   "Open WoWCensus Companion",
		update: "Update available",
		quit:   "Quit",
	},
	"ko": {
		open:   "WoWCensus Companion 열기",
		update: "업데이트 가능",
		quit:   "종료",
	},
	"es": {
		open:   "Abrir WoWCensus Companion",
		update: "Actualización disponible",
		quit:   "Salir",
	},
	"de": {
		open:   "WoWCensus Companion öffnen",
		update: "Aktualisierung verfügbar",
		quit:   "Beenden",
	},
	"fr": {
		open:   "Ouvrir WoWCensus Companion",
		update: "Mise à jour disponible",
		quit:   "Quitter",
	},
	"it": {
		open:   "Apri WoWCensus Companion",
		update: "Aggiornamento disponibile",
		quit:   "Esci",
	},
}

func detectSystemLang() string {
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if val := strings.ToLower(os.Getenv(env)); val != "" {
			for _, code := range []string{"ko", "es", "de", "fr", "it"} {
				if strings.HasPrefix(val, code) {
					return code
				}
			}
		}
	}
	return "en"
}

func getTrayStrings(lang string) trayStrings {
	if lang == "" || lang == "auto" {
		lang = detectSystemLang()
	}
	if ts, ok := trayTranslations[lang]; ok {
		return ts
	}
	return trayTranslations["en"]
}

//go:embed all:frontend/dist
var assets embed.FS

//go:embed trayicon.png
var trayIcon []byte

//go:embed build/appicon.png
var appIcon []byte

func init() {

	application.RegisterEvent[companion.Status]("status")
	application.RegisterEvent[string]("confirm-leave")
}

func main() {
	relaunch, installed, err := companion.InstallToUserHome(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	if installed {
		if err := exec.Command(relaunch.Path, relaunch.Args...).Start(); err != nil {
			log.Fatal(err)
		}
		return
	}
	startHidden := false
	for _, a := range os.Args[1:] {
		if a == "--minimized" {
			startHidden = true
		}
	}

	engine, err := companion.NewEngine("")
	if err != nil {
		log.Fatal(err)
	}
	companion.CleanupOldVersion()
	if engine.Config().StartAtLogin {
		if err := companion.SetStartAtLogin(true); err != nil {
			log.Printf("could not update start-at-login entry: %v", err)
		}
	}
	svc := &CompanionService{engine: engine}

	app := application.New(application.Options{
		Name:        "WoWCensus Companion",
		Description: "Watches WoWCensus SavedVariables and uploads them with your consent",
		Icon:        appIcon,
		Services:    []application.Service{application.NewService(svc)},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac: application.MacOptions{

			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "WoWCensus Companion",
		Width:            980,
		Height:           720,
		MinWidth:         720,
		MinHeight:        520,
		URL:              "/",
		Hidden:           startHidden,
		BackgroundColour: application.NewRGB(17, 19, 24),
	})

	var quitting atomic.Bool
	var allowMinimise atomic.Bool
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if quitting.Load() {
			return
		}
		e.Cancel()
		if svc.hasUnsavedChanges() {
			svc.askToLeave("close")
			return
		}
		win.Hide()
	})
	win.OnWindowEvent(events.Common.WindowMinimise, func(*application.WindowEvent) {
		if allowMinimise.Swap(false) || !svc.hasUnsavedChanges() {
			return
		}
		win.Restore()
		win.Focus()
		svc.askToLeave("minimise")
	})

	show := func() {
		win.Show()
		win.Restore()
		win.Focus()
		go engine.CheckForUpdateIfOlder(context.Background(), time.Hour)
	}
	quit := func() {
		quitting.Store(true)
		app.Quit()
	}
	svc.quit = quit
	svc.leave = func(action string) {
		switch action {
		case "close":
			win.Hide()
		case "minimise":
			allowMinimise.Store(true)
			win.Minimise()
		case "quit":
			quit()
		}
	}
	quitFromTray := func() {
		if svc.hasUnsavedChanges() {
			show()
			svc.askToLeave("quit")
			return
		}
		quit()
	}

	menu := application.NewMenu()
	curLang := engine.Config().Language
	tStr := getTrayStrings(curLang)
	var latestVersion string

	openItem := menu.Add(tStr.open).OnClick(func(*application.Context) { show() })
	updateItem := menu.Add(tStr.update).OnClick(func(*application.Context) { show() })
	updateItem.SetHidden(true)
	menu.AddSeparator()
	quitItem := menu.Add(tStr.quit).OnClick(func(*application.Context) { quitFromTray() })

	updateLabel := func(ts trayStrings) string {
		if latestVersion == "" {
			return ts.update
		}
		return fmt.Sprintf("%s (%s)", ts.update, latestVersion)
	}
	updateTrayLabels := func(lang string) {
		curLang = lang
		ts := getTrayStrings(curLang)
		openItem.SetLabel(ts.open)
		updateItem.SetLabel(updateLabel(ts))
		quitItem.SetLabel(ts.quit)
		menu.Update()
	}
	setTrayUpdate := func(version string) {
		if version == latestVersion {
			return
		}
		latestVersion = version
		updateItem.SetLabel(updateLabel(getTrayStrings(curLang)))
		updateItem.SetHidden(version == "")
		menu.Update()
	}

	svc.onLanguageChange = func(lang string) {
		application.InvokeSync(func() {
			updateTrayLabels(lang)
		})
	}

	svc.onStatus = func(st companion.Status) {
		version := ""
		if st.Update.Available {
			version = st.Update.Version
		}
		application.InvokeAsync(func() { setTrayUpdate(version) })
	}

	engine.OnUpdateFound(func(version string) {
		application.InvokeSync(func() {
			setTrayUpdate(version)
			show()
		})
	})

	tray := app.SystemTray.New()
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(trayIcon)
	} else {
		tray.SetIcon(trayIcon)
	}
	tray.SetTooltip("WoWCensus Companion")
	tray.SetMenu(menu)
	if runtime.GOOS != "darwin" {
		tray.OnClick(show)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
