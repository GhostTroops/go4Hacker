go get -u
go mod tidy
cd frontend
npm i npm@latest
npm i npm-check@latest -g
npm install -g npm-check-updates
ncu -u
npm audit fix --force
npm-check -u
npm update --depth 9999 --dev
npm audit fix --force
npm cache clear --force

yarn install
yarn upgrade
yarn build

cd ..
go build

