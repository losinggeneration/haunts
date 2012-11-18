ECHO off
IF "%1"=="" GOTO HELP
FOR /d %%I IN (*) DO (
echo %%I:
cd %%I
git %*
cd ..
)
GOTO:EOF
:HELP
ECHO USAGE:
ECHO  Replace 'git acommand [params]' with 'forall acommand [params]'
ECHO  This executes that git command inside every current subdirectory.
ECHO Example:
ECHO  forall branch -v -a
ECHO  forall checkout my-bugfix-43


