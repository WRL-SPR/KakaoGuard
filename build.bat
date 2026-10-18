@echo off
setlocal
go-winres make --arch amd64,386,arm64 --out cmd/rsrc
if errorlevel 1 exit /b 1

set GOARCH=amd64
go build -trimpath -o KakaoGuard.exe -ldflags "-H windowsgui" ./cmd
if errorlevel 1 exit /b 1

set GOARCH=386
go build -trimpath -o KakaoGuard_i386.exe -ldflags "-H windowsgui" ./cmd
if errorlevel 1 exit /b 1

set GOARCH=arm64
go build -trimpath -o KakaoGuard_arm64.exe -ldflags "-H windowsgui" ./cmd
if errorlevel 1 exit /b 1
