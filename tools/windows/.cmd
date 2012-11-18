@echo off

rem
rem HEY!
rem
rem You should absolutely read this script and make changes as appropriate!
rem

rem We always want this. If you installed MinGW elsewhere,
rem be sure to point these to the right directory.
set MINGWDIR=C:\MinGW
set MSYSDIR=%MINGWDIR%\msys\1.0

rem Enable only one of these HOME variables.
rem You disable the command by preceding it with 'rem' (short for remark)

rem If your username contains no spaces, then 
rem you can use your default windows directory.
set HOME=%USERPROFILE%

rem If your username contains spaces, or you want to put you Haunts
rem installation elsewhere, set HOME accordingly. This is one
rem possible location you could use (inside the MinGW directory).
rem set HOME=%MSYSDIR%\home\%USERNAME%

rem This must always be done to put all of the tools into your PATH.
set PATH=%PATH%;%MINGWDIR%\bin
set PATH=%PATH%;%MSYSDIR%\bin
set PATH=%PATH%;C:\Go\bin

rem This is required to tell Go the location of your source files
set GOPATH=C:\haunts

rem This allows us to use some haunts batch files to do Git operations on multiple packages
rem This files can be run from anywhere, but they are meant to be used
rem from the %GOPATH%\src\githhub.com\MobRulesGames directory.
set PATH=%PATH%;%GOPATH%\src\github.com\MobRulesGames\haunts\tools\windows

rem This is to make your favorite editor that much more favorite
set PATH=%PATH%;"C:\Program Files (x86)\Notepad++"
doskey n=notepad++ $*
rem now I can just type "n afile.go" 
