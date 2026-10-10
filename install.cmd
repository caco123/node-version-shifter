@echo off
rem ==============================================================================
rem nvsh (node-version-shifter) installer launcher for Windows CMD/PowerShell
rem Bypasses PowerShell ExecutionPolicy restrictions without administrator rights
rem ==============================================================================

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0install.ps1" %*
