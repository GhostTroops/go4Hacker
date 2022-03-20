go get -u
go mod tidy
cd frontend
npm i npm@latest
# 不要用yarn之外的更新，否则会导致各种版本冲突，从而yarn build失败
# npm i npm-check@latest -g
# npm install -g npm-check-updates
# ncu -u
# npm-check -u
# npm audit fix --force
# npm update --depth 9999 --dev
# npm audit fix --force
# npm cache clear --force

yarn install
yarn add cache-loader
# npx browserslist@latest --update-db
yarn build 
# yarn upgrade
yarn build

cd ..
go build
