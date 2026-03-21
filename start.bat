@echo off
setlocal

set "SCRIPT_DIR=%~dp0"
pushd "%SCRIPT_DIR%" >nul
if errorlevel 1 exit /b 1

if not defined GOPATH set "GOPATH=%CD%\.gopath"
if not defined GOMODCACHE set "GOMODCACHE=%GOPATH%\pkg\mod"
if not defined GOCACHE set "GOCACHE=%CD%\.gocache"

if not exist "%GOMODCACHE%" mkdir "%GOMODCACHE%"
if not exist "%GOCACHE%" mkdir "%GOCACHE%"

if "%~1"=="" (
    go run .\cmd\bubblecopy -config ".\tasks.example.csv" -workers 4
) else (
    go run .\cmd\bubblecopy %*
)

set "EXIT_CODE=%ERRORLEVEL%"
popd >nul
exit /b %EXIT_CODE%
