//go:build windows

package main

import (
	"context"
	"flag"
	"os/exec"
	"time"

	"github.com/WRL-SPR/KakaoGuard/internal"
	"github.com/WRL-SPR/KakaoGuard/internal/win"
	"github.com/WRL-SPR/KakaoGuard/winres"
	"github.com/energye/systray"

	_ "github.com/WRL-SPR/KakaoGuard/winres"
)

const (
	VERSION        = "1.0.1"
	appDisplayName = "Kakao Guard"
	repositoryURL  = "https://github.com/WRL-SPR/KakaoGuard"
)

func main() {
	startedAtLogin := flag.Bool("startup", false, "started automatically at login")
	flag.Parse()

	if err := win.MigrateLegacyStartupRegistration(); err != nil {
		win.ShowError(appDisplayName, "Could not migrate startup settings:\n"+err.Error())
	}

	releaseInstance, acquired, err := win.AcquireSingleInstance("KakaoGuard")
	if err != nil {
		win.ShowError(appDisplayName, "Could not initialize the application:\n"+err.Error())
		return
	}
	if !acquired {
		return
	}
	defer releaseInstance()

	ctx, cancel := context.WithCancel(context.Background())
	destroy := func() {
		cancel()
		systray.Quit()
	}

	systray.SetOnRClick(func(menu systray.IMenu) {
		menu.ShowMenu()
	})
	systray.Run(func() {
		systray.SetIcon(winres.IconData)
		systray.SetTooltip(appDisplayName + " - Active")

		statusItem := systray.AddMenuItem("Ad blocking is active", "Current status")
		statusItem.Disable()
		versionItem := systray.AddMenuItem("Version "+VERSION, VERSION)
		versionItem.Disable()
		systray.AddSeparator()

		checkRelease := systray.AddMenuItem("Check for updates", "Open the releases page")
		checkRelease.Click(func() {
			openURL(repositoryURL + "/releases")
		})
		go func() {
			tagName, hasNewRelease := internal.CheckLatestVersion(VERSION)
			if hasNewRelease {
				checkRelease.SetTitle("New version available: " + tagName)
			}
		}()

		startupItem := systray.AddMenuItem(
			"Run on startup",
			"Run automatically after signing in",
		)
		if win.IsStartupEnabled() {
			startupItem.Check()
		}
		startupItem.Click(func() {
			if startupItem.Checked() {
				if err := win.SetStartupEnabled(false); err != nil {
					win.ShowError(
						"Startup settings",
						"Could not disable startup:\n"+err.Error(),
					)
					return
				}
				startupItem.Uncheck()
				return
			}

			if err := win.SetStartupEnabled(true); err != nil {
				win.ShowError(
					"Startup settings",
					"Could not enable startup:\n"+err.Error(),
				)
				return
			}
			startupItem.Check()
			win.ShowInfo(
				"Startup settings",
				appDisplayName+" will start automatically the next time you sign in.",
			)
		})

		projectItem := systray.AddMenuItem("Open project page", "Open GitHub")
		projectItem.Click(func() {
			openURL(repositoryURL)
		})
		systray.AddSeparator()
		systray.AddMenuItem("E&xit", "Exit").Click(destroy)

		internal.Run(ctx)
		if *startedAtLogin {
			go func() {
				// Give the watcher one polling cycle before KakaoTalk starts.
				time.Sleep(200 * time.Millisecond)
				if err := win.LaunchManagedKakaoTalk(); err != nil {
					win.ShowError(
						"KakaoTalk startup",
						"Could not start KakaoTalk after the blocker:\n"+err.Error(),
					)
				}
			}()
		}
	}, func() {
		cancel()
	})
}

func openURL(url string) {
	if err := exec.Command(
		"rundll32",
		"url.dll,FileProtocolHandler",
		url,
	).Start(); err != nil {
		win.ShowError(appDisplayName, "Could not open the web browser:\n"+err.Error())
	}
}

