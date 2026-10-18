# Kakao Guard

KakaoTalk Windows client advertisement window guard.

> Unofficial personal fork based on
> [blurfx/KakaoTalkAdBlock](https://github.com/blurfx/KakaoTalkAdBlock).

## Features

- Runs quietly in the Windows notification area
- Starts before KakaoTalk at sign-in
- Repairs stale or disabled startup registration
- Prevents duplicate instances
- Restores KakaoTalk's original startup entry when disabled

## Startup

Use **Run on startup** from the tray menu. Kakao Guard stores KakaoTalk's
existing startup command and launches KakaoTalk after the guard is active.

## Build

Run `build.bat` with Go and `go-winres` installed. Release builds keep Go
symbols and debugging metadata to make inspection easier and reduce unsigned
GUI binary false positives.

## Version

Kakao Guard `1.0.2`


