@REM This batch file will build and install Haunts in \haunts_app,
@REM i.e. at the root of the current drive (presumably C:, but D:\ or 
@REM anything else works too.)
@REM It should be invoked from the MobRulesGames\haunts directory as:
@REM make\windows 
@REM make\windows clean (to force a rebuild of everything)

@REM First, we need to create a GEN_version.go file, which
@REM embeds the current git branch name into the executable.
@ECHO OFF
ECHO Making version file
rm GEN_*
cd tools
go run version.go
cd ..

@REM This is the primary build command

IF /I "%1"=="clean" ( 
IF EXIST \haunts_app (
ECHO Deleting \haunts_app directory
rm -r \haunts_app
)
ECHO Building haunts -a
go build -a . 
) ELSE (
ECHO Building haunts
go build .
)

IF %ERRORLEVEL% GTR 0 (
ECHO Go build failed, stopping. Errorlevel = %ERRORLEVEL%
GOTO:EOF
)

ECHO Creating \haunts_app directories
IF NOT EXIST \haunts_app mkdir \haunts_app
IF NOT EXIST \haunts_app\openme mkdir \haunts_app\openme

ECHO Copying .exe and .dlls
copy haunts.exe \haunts_app\openme\haunts.exe
copy lib\windows\* \haunts_app\openme
copy ..\fmod\lib\windows\* \haunts_app\openme

@REM the \data dir gets flattened in the installation dir
ECHO Copying data directories. 
IF /I NOT "%1"=="clean" ECHO You can now cancel (CTRL-C) if no data has changed
cd data
cp *.json \haunts_app
for /d %%I IN (*) DO (
echo %%I
cp -r %%I \haunts_app
)
cd ..
