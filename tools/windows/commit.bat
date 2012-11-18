echo off
IF NOT "%1"=="" (
SET MSG=%*
) ELSE (
SET /P MSG=Please enter a commit message:
)
for /d %%I IN (*) DO (
cd %%I
git commit -a -m "%MSG%"
cd ..
)

