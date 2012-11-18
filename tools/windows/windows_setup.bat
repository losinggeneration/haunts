@ECHO OFF
REM ..........................................................................
REM One-time install to set cmd.exe to always run a particular 
REM batch file (.cmd) immediately. The .cmd file can be changed as you wish.
REM ..........................................................................
IF EXIST %USERPROFILE%\.cmd (
CHOICE /C YN /M "You already have a .cmd file! Do you want to overwrite it?"
IF errorlevel 2 GOTO skip
IF errorlevel 1 GOTO backup
)
ELSE (
GOTO copy
)
:backup
copy %USERPROFILE%\.cmd %USERPROFILE%\.cmd.old
echo Old .cmd file copied to .cmd.old
:copy
copy .cmd %USERPROFILE%\.cmd
IF NOT EXIST %USERPROFILE%\.cmd (
ECHO Error: Could not create a file in your user directory. 
GOTO:EOF
)
:skip
Echo Adding key to registry
reg add "HKEY_CURRENT_USER\Software\Microsoft\Command Processor" /v "Autorun" /d "%USERPROFILE%\.cmd"
Echo Please edit %USERPROFILE%\.cmd to ensure that the paths are correct (the default assumes C:\haunts, etc.)
Echo Then close this command prompt and open a new one get the features enabled.
