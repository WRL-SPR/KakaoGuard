package win

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	startupKey         = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
	startupApprovedKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
	appSettingsKey     = `SOFTWARE\KakaoGuard`
	legacySettingsKey  = `SOFTWARE\KakaoTalkAdBlock`
	appName            = "KakaoGuard"
	legacyAppName      = "KakaoTalkAdBlock"
	kakaoStartupName   = "KakaoTalk"
	kakaoCommandValue  = "ManagedKakaoTalkStartup"
)

var executablePath = os.Executable

// IsStartupEnabled reports whether the current executable is registered to run
// at login. A stale registration for a moved executable is treated as disabled.
func IsStartupEnabled() bool {
	expected, err := startupCommand()
	if err != nil {
		return false
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, startupKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	value, _, err := k.GetStringValue(appName)
	return err == nil &&
		strings.EqualFold(value, expected) &&
		startupApprovalAllows(appName)
}

// SetStartupEnabled enables or disables the application running at login.
func SetStartupEnabled(enable bool) error {
	k, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		startupKey,
		registry.QUERY_VALUE|registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("open startup registry key: %w", err)
	}
	defer k.Close()

	if enable {
		command, err := startupCommand()
		if err != nil {
			return err
		}

		managedKakao, err := takeOverKakaoStartup(k)
		if err != nil {
			return err
		}
		if err := k.SetStringValue(appName, command); err != nil {
			if managedKakao {
				_ = restoreKakaoStartup(k)
			}
			return fmt.Errorf("write startup registration: %w", err)
		}
		_ = k.DeleteValue(legacyAppName)

		// Task Manager stores a separate disabled marker. Remove a stale marker
		// when the user explicitly enables startup from this application.
		if approved, err := registry.OpenKey(
			registry.CURRENT_USER,
			startupApprovedKey,
			registry.SET_VALUE,
		); err == nil {
			defer approved.Close()
			if err := approved.DeleteValue(appName); err != nil &&
				!errors.Is(err, registry.ErrNotExist) {
				_ = k.DeleteValue(appName)
				if managedKakao {
					_ = restoreKakaoStartup(k)
				}
				return fmt.Errorf("clear disabled startup state: %w", err)
			}
			_ = approved.DeleteValue(legacyAppName)
		}

		return nil
	}

	if err := restoreKakaoStartup(k); err != nil {
		return err
	}
	if err := k.DeleteValue(appName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("remove startup registration: %w", err)
	}
	_ = k.DeleteValue(legacyAppName)
	return nil
}

// MigrateLegacyStartupRegistration renames the visible startup entry while
// preserving the current executable path and managed KakaoTalk command.
func MigrateLegacyStartupRegistration() error {
	if err := migrateLegacySettings(); err != nil {
		return err
	}

	k, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		startupKey,
		registry.QUERY_VALUE|registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("open startup registry key: %w", err)
	}
	defer k.Close()

	if _, _, err := k.GetStringValue(appName); err == nil {
		_ = k.DeleteValue(legacyAppName)
		return nil
	} else if !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("inspect startup registration: %w", err)
	}

	if _, _, err := k.GetStringValue(legacyAppName); errors.Is(err, registry.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("read legacy startup registration: %w", err)
	}

	command, err := startupCommand()
	if err != nil {
		return err
	}
	if err := k.SetStringValue(appName, command); err != nil {
		return fmt.Errorf("write branded startup registration: %w", err)
	}
	if err := k.DeleteValue(legacyAppName); err != nil {
		_ = k.DeleteValue(appName)
		return fmt.Errorf("remove legacy startup registration: %w", err)
	}

	if approved, err := registry.OpenKey(
		registry.CURRENT_USER,
		startupApprovedKey,
		registry.SET_VALUE,
	); err == nil {
		defer approved.Close()
		_ = approved.DeleteValue(appName)
		_ = approved.DeleteValue(legacyAppName)
	}
	return nil
}

func migrateLegacySettings() error {
	legacy, err := registry.OpenKey(
		registry.CURRENT_USER,
		legacySettingsKey,
		registry.QUERY_VALUE,
	)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open legacy application settings: %w", err)
	}
	defer legacy.Close()

	command, _, err := legacy.GetStringValue(kakaoCommandValue)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read legacy KakaoTalk startup backup: %w", err)
	}

	settings, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		appSettingsKey,
		registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("open application settings: %w", err)
	}
	defer settings.Close()

	if err := settings.SetStringValue(kakaoCommandValue, command); err != nil {
		return fmt.Errorf("migrate KakaoTalk startup backup: %w", err)
	}
	return nil
}

func startupCommand() (string, error) {
	exe, err := executablePath()
	if err != nil {
		return "", fmt.Errorf("find executable path: %w", err)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	return `"` + strings.ReplaceAll(exe, `"`, `\"`) + `" --startup`, nil
}

func startupApprovalAllows(valueName string) bool {
	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		startupApprovedKey,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return true
	}
	defer k.Close()

	state, _, err := k.GetBinaryValue(valueName)
	if errors.Is(err, registry.ErrNotExist) {
		return true
	}
	if err != nil {
		return false
	}
	return isStartupApprovedState(state)
}

func isStartupApprovedState(state []byte) bool {
	return len(state) > 0 && state[0] == 0x02
}

// LaunchManagedKakaoTalk starts the KakaoTalk command that was previously
// registered to run independently at login.
func LaunchManagedKakaoTalk() error {
	command, err := managedKakaoCommand()
	if err != nil {
		return err
	}
	if command == "" {
		return nil
	}

	args, err := windows.DecomposeCommandLine(command)
	if err != nil {
		return fmt.Errorf("parse KakaoTalk startup command: %w", err)
	}
	if len(args) == 0 {
		return nil
	}

	if err := exec.Command(args[0], args[1:]...).Start(); err != nil {
		return fmt.Errorf("start KakaoTalk: %w", err)
	}
	return nil
}

func takeOverKakaoStartup(runKey registry.Key) (bool, error) {
	if !startupApprovalAllows(kakaoStartupName) {
		return false, nil
	}

	command, _, err := runKey.GetStringValue(kakaoStartupName)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read KakaoTalk startup registration: %w", err)
	}

	settings, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		appSettingsKey,
		registry.SET_VALUE,
	)
	if err != nil {
		return false, fmt.Errorf("open application settings: %w", err)
	}
	defer settings.Close()

	if err := settings.SetStringValue(kakaoCommandValue, command); err != nil {
		return false, fmt.Errorf("back up KakaoTalk startup registration: %w", err)
	}
	if err := runKey.DeleteValue(kakaoStartupName); err != nil {
		_ = settings.DeleteValue(kakaoCommandValue)
		return false, fmt.Errorf("delegate KakaoTalk startup registration: %w", err)
	}

	return true, nil
}

func restoreKakaoStartup(runKey registry.Key) error {
	command, err := managedKakaoCommand()
	if err != nil {
		return err
	}
	if command == "" {
		return nil
	}

	if _, _, err := runKey.GetStringValue(kakaoStartupName); errors.Is(err, registry.ErrNotExist) {
		if err := runKey.SetStringValue(kakaoStartupName, command); err != nil {
			return fmt.Errorf("restore KakaoTalk startup registration: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect KakaoTalk startup registration: %w", err)
	}

	settings, err := registry.OpenKey(
		registry.CURRENT_USER,
		appSettingsKey,
		registry.SET_VALUE,
	)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open application settings: %w", err)
	}
	defer settings.Close()

	if err := settings.DeleteValue(kakaoCommandValue); err != nil &&
		!errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("remove KakaoTalk startup backup: %w", err)
	}
	return nil
}

func managedKakaoCommand() (string, error) {
	settings, err := registry.OpenKey(
		registry.CURRENT_USER,
		appSettingsKey,
		registry.QUERY_VALUE,
	)
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("open application settings: %w", err)
	}
	defer settings.Close()

	command, _, err := settings.GetStringValue(kakaoCommandValue)
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read KakaoTalk startup backup: %w", err)
	}
	return command, nil
}

