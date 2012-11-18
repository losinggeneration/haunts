echo off
for /d %%I IN (*) DO (
cd %%I
echo %%I :
git config push.default current
git push origin
cd ..
)

