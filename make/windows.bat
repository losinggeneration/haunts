rm GEN_*
cd tools
go run version.go
cd ..

go build .

rm -rf \haunts_app
mkdir \haunts_app\openme
mv haunts.exe \haunts_app\openme
cp -r data\* \haunts_app
copy lib\windows\* \haunts_app\openme
copy lib\windows\* \haunts_app\openme
