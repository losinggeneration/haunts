echo off
for /d %%I IN (*) DO (
cd %%I
git remote rm mrg
git remote add mrg git@github.com:MobRulesGames/%%I.git
cd ..
)

