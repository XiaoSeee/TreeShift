@echo off
setlocal EnableExtensions

cd /d "%~dp0"

set "APP_NAME=TreeShift"
set "OUTPUT_PATH=%CD%\build\bin\TreeShift.exe"
set "ROOT_OUTPUT_PATH=%CD%\TreeShift.exe"

echo [INFO] Building %APP_NAME%...

call :require_command go Go
if errorlevel 1 goto :failed

call :require_command npm.cmd npm
if errorlevel 1 goto :failed

call :require_command wails "Wails CLI"
if errorlevel 1 goto :failed

echo [STEP] Running Go tests...
go test ./...
if errorlevel 1 (
  echo [ERROR] Go tests failed.
  goto :failed
)

echo [STEP] Running: wails build
wails build
if not errorlevel 1 goto :check_output

echo [WARN] Standard build failed.
if exist "frontend\dist\index.html" (
  echo [STEP] Running fallback: wails build -s
  wails build -s
  if not errorlevel 1 goto :check_output
  echo [ERROR] Fallback build failed.
) else (
  echo [ERROR] frontend\dist\index.html not found. Fallback build is unavailable.
)

goto :failed

:require_command
where %~1 >nul 2>nul
if errorlevel 1 (
  echo [ERROR] %~2 is not available in PATH.
  exit /b 1
)
exit /b 0

:check_output
if exist "%OUTPUT_PATH%" (
  echo [STEP] Moving output to root directory...
  if exist "%ROOT_OUTPUT_PATH%" del /f /q "%ROOT_OUTPUT_PATH%"
  move /y "%OUTPUT_PATH%" "%ROOT_OUTPUT_PATH%" >nul
  if errorlevel 1 (
    echo [ERROR] Failed to move output to root directory.
    goto :failed
  )
  echo [OK] Build completed.
  echo [OUTPUT] %ROOT_OUTPUT_PATH%
  pause
  exit /b 0
)

echo [ERROR] Build command finished but output was not found.
goto :failed

:failed
echo [FAILED] Build did not complete.
pause
exit /b 1
