[![Tweet](https://img.shields.io/twitter/url/http/Hktalent3135773.svg?style=social)](https://twitter.com/intent/tweet?original_referer=https%3A%2F%2Fdeveloper.twitter.com%2Fen%2Fdocs%2Ftwitter-for-websites%2Ftweet-button%2Foverview&ref_src=twsrc%5Etfw&text=myhktools%20-%20Automated%20Pentest%20Recon%20Scanner%20%40Hktalent3135773&tw_p=tweetbutton&url=https%3A%2F%2Fgithub.com%2Fhktalent%2Fmyhktools)
[![Follow on Twitter](https://img.shields.io/twitter/follow/Hktalent3135773.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=Hktalent3135773)
# go4Hacker

Automated penetration and auxiliary systems, providing XSS, XXE, DNS log, SSRF, RCE, web netcat and other Servers

English Doc | [中文文档](https://github.com/hktalent/go4Hacker/blob/master/README_CN.md)

## features

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
cd frontend
yarn install
yarn build
```
	
## build backend

requirements: 

`golang >= 1.13.0`

```bash
go build
```

## docker build

```bash
docker build -t "user/godnslog" .
```

For Chinese user:

```bash
docker build -t "user/godnslog" -f DockerfileCN .
```

## RUN

i. Register your domain, eg: `example.com`
Set your DNS Server point to your host, eg: ns.example.com => 100.100.100.100
Some registrar limit set to NS host, your can set two ns host point to only one address.
Some registrar to ns host must be different ip address, you can set one to a fake addresss and then change to the same addresss


ii. self build

```bash
docker run -p80:8080 -p53:53/udp "user/godnslog"  serve -domain yourdomain.com -4 100.100.100.100
```

or use dockerhub

```bash
docker pull "sort/godnslog"
docker run -p80:8080 -p53:53/udp -p80:8080  "sort/godnslog" serve -domain yourdomain.com -4 100.100.100.100
```

iii. access http://100.100.100.100

## Doc

guest/guest123

[introduce](https://www.godnslog.com/document/introduce)
[payload](https://www.godnslog.com/document/payload)
[api](https://www.godnslog.com/document/api)
[rebiding](https://www.godnslog.com/document/rebinding)
[resolve](https://www.godnslog.com/document/resolve)

## TODO && Known Issues

- [ ] admin user can read all recordds
- [ ] allow Anonymous user access document page
- [ ] enable custom rebinding stage two setting
- [ ] fix login logical problem

## build from
https://github.com/chennqqi/godnslog
but the go and node js modules here are updated to the latest