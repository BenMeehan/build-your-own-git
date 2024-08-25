@echo off
setlocal enabledelayedexpansion

REM 
cd /d "%~dp0"

REM
go build -o %TEMP%\ben_git.exe ./cmd/mygit

REM
%TEMP%\ben_git.exe %*
