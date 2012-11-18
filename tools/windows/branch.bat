echo off
for /d %%I IN (*) DO (
cd %%I
if NOT "%1"=="" (
git checkout -b %1 
) ELSE (
REM A hack to ECHO something with CRLF
<nul (set/p dummy="%%I ")
git branch %1
)
cd ..
)

