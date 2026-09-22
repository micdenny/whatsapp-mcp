@echo off
REM Run the WhatsApp bridge in a visible console window.
REM Build it first:  cd whatsapp-bridge && go build -o whatsapp-bridge.exe .
REM On Windows go-sqlite3 needs cgo, so a C compiler (e.g. MinGW-w64) must be on
REM PATH and CGO_ENABLED must be 1.
title WhatsApp Bridge
cd /d "%~dp0whatsapp-bridge"
if not exist "whatsapp-bridge.exe" (
    echo whatsapp-bridge.exe not found. Build it with:
    echo     cd whatsapp-bridge ^&^& go build -o whatsapp-bridge.exe .
    pause
    exit /b 1
)
"%~dp0whatsapp-bridge\whatsapp-bridge.exe"
pause
