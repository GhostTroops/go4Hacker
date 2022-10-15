go mod tidy
go mod vendor
rm -rf dist
cd frontend
npm i npm@latest
yarn install
yarn build
mv dist ../
cd ../

# 不要用yarn之外的更新，否则会导致各种版本冲突，从而yarn build失败
# npm i npm-check@latest -g
# npm install -g npm-check-updates
# ncu -u
# npm-check -u
# npm audit fix --force
# npm update --depth 9999 --dev
# npm audit fix --force
# npm cache clear --force

# yarn add cache-loader
# npm run lint
# npx browserslist@latest --update-db
# yarn build 
# yarn upgrade
#yarn build

go build -o go4Hacker main.go
./go4Hacker serve -4 192.168.0.107 -domain 51pwn.com -lang zh-CN
