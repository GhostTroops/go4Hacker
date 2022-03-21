[![Tweet](https://img.shields.io/twitter/url/http/Hktalent3135773.svg?style=social)](https://twitter.com/intent/tweet?original_referer=https%3A%2F%2Fdeveloper.twitter.com%2Fen%2Fdocs%2Ftwitter-for-websites%2Ftweet-button%2Foverview&ref_src=twsrc%5Etfw&text=myhktools%20-%20Automated%20Pentest%20Recon%20Scanner%20%40Hktalent3135773&tw_p=tweetbutton&url=https%3A%2F%2Fgithub.com%2Fhktalent%2Fmyhktools)
[![Follow on Twitter](https://img.shields.io/twitter/follow/Hktalent3135773.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=Hktalent3135773)
# go4Hacker

Automated penetration and auxiliary systems, providing XSS, XXE, DNS log, SSRF, RCE, web netcat and other Servers

English Doc | [中文文档](https://github.com/hktalent/go4Hacker/blob/master/README_CN.md)

## features
- gin
- vue
- suport http2, -ServerPem -ServerKey
- Standard Domain Resolve Service
- DNSLOG
- HTTPLOG
- Rebinding/CustomRebinding
- Push (callback)
- Multi-user
- dockerlized
- python/golang client sdk
- as a standard name resolve service with support `A,CNAME,TXT,MX`
- xip


### DNSLOG

super admin user: `admin`
password will be showed in console logs when first run.
you can change it by subcommand `resetpw`

![](https://s1.ax1x.com/2020/08/31/dXPba4.png)


### HTTPLOG
![](https://s1.ax1x.com/2020/08/31/dXiiIH.png)


## build frontend

requirements: 

`yarn`

```
npm i npm@latest
npm install --global yarn
cd frontend
yarn install
yarn build
```
	
## build backend

requirements: 

`golang >= 1.17`
`node >= 14.17.6`
`npm >= 8.5.5`
`yarn >= 1.22.17`

```bash
cd frontend
npm i npm@latest
yarn install
yarn add cache-loader
yarn build
cd ..
go build

# run help
./go4Hacker -h
./go4Hacker serve -h
```

## docker build

```bash
docker build -t "user/go4Hacker" .
```

For Chinese user:

```bash
docker build -t "user/go4Hacker" -f DockerfileCN .
```

## RUN

i. Register your domain, eg: `example.com`
Set your DNS Server point to your host, eg: ns.example.com => 100.100.100.100
Some registrar limit set to NS host, your can set two ns host point to only one address.
Some registrar to ns host must be different ip address, you can set one to a fake addresss and then change to the same addresss


ii. self build

```bash
docker run -p80:8080 -p53:53/udp "user/go4Hacker"  serve -domain yourdomain.com -4 100.100.100.100
```

or use dockerhub

```bash
docker pull "sort/go4Hacker"
docker run -p80:8080 -p53:53/udp -p80:8080  "sort/go4Hacker" serve -domain yourdomain.com -4 100.100.100.100
```

iii. access http://100.100.100.100

## Doc

guest/guest123


## TODO && Known Issues

- [ ] admin user can read all recordds
- [ ] allow Anonymous user access document page
- [ ] enable custom rebinding stage two setting
- [ ] fix login logical problem

## build from
https://github.com/chennqqi/go4Hacker
but the go and node js modules here are updated to the latest