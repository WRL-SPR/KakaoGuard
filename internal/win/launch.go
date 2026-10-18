//go:build windows

package win

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// LaunchKakaoTalk uses the saved startup command or a registered installation.
func LaunchKakaoTalk() error {
	command, err := managedKakaoCommand()
	if err != nil {
		return err
	}
	if command != "" {
		return LaunchManagedKakaoTalk()
	}
	var candidates []string
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		for _, view := range []uint32{registry.WOW64_64KEY, registry.WOW64_32KEY} {
			key, err := registry.OpenKey(root, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\KakaoTalk.exe`, registry.QUERY_VALUE|view)
			if err != nil {
				continue
			}
			path, _, err := key.GetStringValue("")
			key.Close()
			if err == nil {
				candidates = append(candidates, path)
			}
		}
	}
	for _, variable := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"} {
		if base := os.Getenv(variable); base != "" {
			candidates = append(candidates, filepath.Join(base, "Kakao", "KakaoTalk", "KakaoTalk.exe"))
		}
	}
	path, err := findKakaoExecutable(candidates)
	if err != nil {
		return err
	}
	return exec.Command(path).Start()
}

func findKakaoExecutable(candidates []string) (string, error) {
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("카카오톡 설치 경로를 찾지 못했습니다. 카카오톡을 직접 실행해 주세요")
}

