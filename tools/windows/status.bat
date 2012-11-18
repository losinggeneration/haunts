echo off
for /d %%I IN (*) DO (
cd %%I
echo %%I %1:
git status %1 
cd ..
)

