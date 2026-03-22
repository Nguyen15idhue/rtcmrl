@echo off
setlocal

echo ================================================
echo RTCM Gateway - Quick Start
echo ================================================
echo.

set GATEWAY_DIR=%~dp0rtcm-stream-gateway
set GATEWAY_EXE=%GATEWAY_DIR%\gateway.exe
set UI_DIR=%~dp0frontend-standalone

if not exist "%GATEWAY_EXE%" (
    echo ERROR: gateway.exe not found at %GATEWAY_EXE%
    echo Please build the gateway first: cd rtcm-stream-gateway ^&^& go build -o gateway.exe ./cmd/gateway/
    pause
    exit /b 1
)

echo Starting Gateway...
start "RTCM Gateway" cmd /c "cd /d %GATEWAY_DIR% && .\gateway.exe"

echo Waiting for gateway to start...
timeout /t 3 /nobreak >nul

echo Starting UI...
start "" "%UI_DIR%\main.js"

echo.
echo ================================================
echo Gateway: http://localhost:8080
echo ================================================
echo.

endlocal
