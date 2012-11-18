echo off
for /d %%I IN (*) DO (
cd %%I
REM delete local branch
git branch -d %1
REM delete local tracking branch
git branch -d -r origin/%1
REM delete remote branch
git push origin :%1
cd ..
)

