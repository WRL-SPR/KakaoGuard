//go:build windows

package main

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/WRL-SPR/KakaoGuard/internal"
	"github.com/WRL-SPR/KakaoGuard/internal/win"
	"github.com/WRL-SPR/KakaoGuard/winres"
	"github.com/energye/systray"
)

const (
	VERSION        = "1.0.2"
	appDisplayName = "Kakao Guard"
	repositoryURL  = "https://github.com/WRL-SPR/KakaoGuard"
)

func main() {
	startedAtLogin := flag.Bool("startup", false, "started automatically at login")
	flag.Parse()

	releaseInstance, acquired, err := win.AcquireSingleInstance("KakaoGuard")
	if err != nil {
		win.ShowError(appDisplayName, "프로그램을 시작하지 못했습니다.\n\n"+err.Error())
		return
	}
	if !acquired {
		return
	}
	defer releaseInstance()

	if err := win.MigrateLegacyStartupRegistration(); err != nil {
		win.ShowError("자동 시작 설정", "기존 자동 시작 설정을 이전하지 못했습니다.\n\n"+err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	systray.Run(func() {
		systray.SetIcon(winres.IconData)
		systray.SetTooltip(appDisplayName + " " + VERSION + "\n광고 차단 감시 중 · 클릭하여 메뉴 열기")
		header := systray.AddMenuItem("Kakao Guard  ·  v"+VERSION, "프로그램 버전")
		header.Disable()
		status := systray.AddMenuItem("광고 차단 감시 중", "카카오톡 실행 시 자동으로 광고 창을 감시합니다")
		status.Disable()
		systray.AddSeparator()

		systray.AddMenuItem("카카오톡 열기(&K)", "설치된 카카오톡 실행").Click(func() {
			if err := win.LaunchKakaoTalk(); err != nil {
				win.ShowError("카카오톡 열기", "카카오톡을 열지 못했습니다.\n\n"+err.Error())
			}
		})
		systray.AddSeparator()
		startupItem := systray.AddMenuItemCheckbox("로그인 시 자동 실행(&S)", "광고 차단기를 먼저 실행한 다음 카카오톡을 실행합니다", false)
		var startupMu sync.Mutex
		refreshStartup := func() {
			if win.IsStartupEnabled() {
				startupItem.Check()
			} else {
				startupItem.Uncheck()
			}
		}
		refreshStartup()
		startupItem.Click(func() {
			startupMu.Lock()
			defer startupMu.Unlock()
			enabled := !win.IsStartupEnabled()
			if err := win.SetStartupEnabled(enabled); err != nil {
				win.ShowError("자동 시작 설정", "자동 실행 설정을 변경하지 못했습니다.\n\n"+err.Error())
			}
			refreshStartup()
		})
		systray.AddMenuItem("Windows 시작 앱 설정(&W)", "Windows에서 자동 실행 앱 관리").Click(func() {
			openURL("ms-settings:startupapps")
		})
		systray.AddMenuItem("프로그램 폴더 열기(&F)", "현재 실행 파일이 있는 폴더 열기").Click(func() {
			path, err := os.Executable()
			if err == nil {
				err = exec.Command("explorer.exe", filepath.Dir(path)).Start()
			}
			if err != nil {
				win.ShowError(appDisplayName, "폴더를 열지 못했습니다.\n\n"+err.Error())
			}
		})
		systray.AddSeparator()
		checkRelease := systray.AddMenuItem("업데이트 확인(&U)", "GitHub에서 최신 버전 내려받기")
		checkRelease.Click(func() { openURL(repositoryURL + "/releases/latest") })
		go func() {
			if tag, available := internal.CheckLatestVersion(VERSION); available {
				checkRelease.SetTitle("새 버전 다운로드: " + tag + " (&U)")
			}
		}()
		systray.AddMenuItem("프로젝트 페이지(&P)", "소스 코드와 문제 신고").Click(func() { openURL(repositoryURL) })
		systray.AddMenuItem("프로그램 정보(&A)", "버전 및 사용 안내").Click(func() {
			win.ShowInfo(appDisplayName, appDisplayName+" "+VERSION+"\n\n카카오톡 광고 창을 감시하고 숨깁니다.\n\n• 트레이 아이콘을 클릭하면 메뉴가 열립니다.\n• 자동 실행을 켜면 다음 로그인부터 적용됩니다.\n• 종료하면 광고 차단 감시가 중단됩니다.\n  이미 숨긴 광고는 즉시 복원되지 않을 수 있습니다.\n\n비공식 커뮤니티 포크 · WRL-SPR/KakaoGuard")
		})
		systray.AddSeparator()
		systray.AddMenuItem("Kakao Guard 종료(&X)", "광고 차단 감시 종료 · 카카오톡은 계속 실행됩니다").Click(func() { cancel(); systray.Quit() })

		showMenu := func(menu systray.IMenu) {
			startupMu.Lock()
			refreshStartup()
			startupMu.Unlock()
			_ = menu.ShowMenu()
		}
		systray.SetOnClick(showMenu)
		systray.SetOnRClick(showMenu)
		internal.Run(ctx)
		if *startedAtLogin {
			go func() {
				timer := time.NewTimer(200 * time.Millisecond)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				}
				if err := win.LaunchManagedKakaoTalk(); err != nil {
					win.ShowError("카카오톡 자동 실행", "광고 차단기 실행 후 카카오톡을 시작하지 못했습니다.\n\n"+err.Error())
				}
			}()
		}
	}, cancel)
}

func openURL(url string) {
	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start(); err != nil {
		win.ShowError(appDisplayName, "페이지를 열지 못했습니다.\n\n"+err.Error())
	}
}

