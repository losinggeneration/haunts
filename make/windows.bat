rm GEN_*
cd tools
go run version.go
cd ..

REM go build .

@REM rm -rf \haunts_app
@REM mkdir \haunts_app\openme
copy haunts.exe \haunts_app\openme\haunts.exe
copy lib\windows\* \haunts_app\openme
copy ..\fmod\lib\windows\* \haunts_app\openme

@REM the \data dir gets flattened in the installation dir
@ECHO OFF
ECHO Copying data directories. 
ECHO You can now cancel (CTRL-C) if nothing has changed.
cd data
cp *.json \haunts_app
for /d %%I IN (*) DO (
echo %%I
cp -r %%I \haunts_app
)

cd ..