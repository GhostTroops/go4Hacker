package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/google/subcommands"
	"github.com/hktalent/go4Hacker/cache"
	"github.com/hktalent/go4Hacker/server"
	"github.com/sirupsen/logrus"
)

type servePwCmd struct {
	swagger   bool
	withGuest bool
	ServerPem string
	ServerKey string
	domain,
	driver, dsn,
	ipv4, ipv6,
	defaultLanguage string
	httpListen string
	upstream   string
}

func (*servePwCmd) Name() string     { return "serve" }
func (*servePwCmd) Synopsis() string { return "Serve dnslog." }
func (*servePwCmd) Usage() string {
	return `serve [-options] <some text>:
  Print args to stdout.
`
}

func (p *servePwCmd) SetFlags(f *flag.FlagSet) {
	sKp := `
Generate private key (.key)

# Key considerations for algorithm "RSA" ≥ 2048-bit
openssl genrsa -out server.key 2048
	
# Key considerations for algorithm "ECDSA" ≥ secp384r1
# List ECDSA the supported curves (openssl ecparam -list_curves)
openssl ecparam -genkey -name secp384r1 -out server.key
Generation of self-signed(x509) public key (PEM-encodings .pem|.crt) based on the private (.key)

openssl req -new -x509 -sha256 -key server.key -out server.pem -days 3650`
	f.StringVar(&p.domain, "domain", "example.com", "set domain, required")
	f.StringVar(&p.ServerPem, "ServerPem", "", "set Server Pem: Server.pem "+sKp)
	f.StringVar(&p.ServerKey, "ServerKey", "", "set Server key: Server.key")
	f.StringVar(&p.ipv4, "4", "", "set public IPv4, required")
	//flag.StringVar(&ipv6, "6", "", "set ipv6 publicIP, option")	// not support IPv6 now

	//https://github.com/mattn/go-sqlite3/issues/39
	f.StringVar(&p.dsn, "dsn", "file:godnslog.db?cache=shared&mode=rwc", "set database source name, option")
	f.StringVar(&p.driver, "driver", "sqlite3", "set database driver, [sqlite3/mysql], option")

	f.StringVar(&p.upstream, "upstreamp", "8.8.8.8:53", "set upstream dns")
	f.BoolVar(&p.swagger, "swagger", false, "with swagger, option")
	f.BoolVar(&p.withGuest, "guest", false, "init with guest user")

	f.StringVar(&p.defaultLanguage, "lang", DefaultLanguage, "set default language, [en-US/zh-CN], option")
	f.StringVar(&p.httpListen, "http", ":8080", "set http listen, option")
}

func (p *servePwCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	// verify input
	{
		if p.ipv4 == "" || p.domain == "" {
			logrus.Fatal("[main.go::main] You should set ipv4 and domain at least.")
			return subcommands.ExitUsageError
		}
		if p.swagger {
			logrus.Warnf("[main.go::main] We only suggest set this option in debug enviroment.")
			return subcommands.ExitUsageError
		}
	}

	var wg sync.WaitGroup

	//	cache store
	store := cache.NewCache(24*3600*time.Second, 10*time.Minute)

	web, err := server.NewWebServer(&server.WebServerConfig{
		Driver:                       p.driver,
		Dsn:                          p.dsn,
		Domain:                       p.domain,
		IP:                           p.ipv4,
		ServerPem:                    p.ServerPem,
		ServerKey:                    p.ServerKey,
		Listen:                       p.httpListen,
		Swagger:                      p.swagger,
		WithGuest:                    p.withGuest,
		AuthExpire:                   AuthExpire,
		DefaultCleanInterval:         DefaultCleanInterval,
		DefaultQueryApiMaxItem:       DefaultQueryApiMaxItem,
		DefaultMaxCallbackErrorCount: DefaultMaxCallbackErrorCount,
		DefaultLanguage:              DefaultLanguage,
	}, store)
	if err != nil {
		logrus.Fatalf("[main.go::main] NewWebServer: %v", err)
	}

	//run async store routine
	{
		wg.Add(1)
		go func() {
			defer wg.Done()
			web.RunStoreRoutine()
		}()
	}

	//run web server routine
	{
		wg.Add(1)
		go func() {
			defer wg.Done()
			web.Run()
		}()
	}

	dns, err := server.NewDnsServer(&server.DnsServerConfig{
		Domain:   p.domain,
		RTimeout: 3 * time.Second,
		WTimeout: 3 * time.Second,
		V4:       net.ParseIP(p.ipv4),
		V6:       net.ParseIP(p.ipv6),
		Upstream: p.upstream,
	}, store)
	if err != nil {
		logrus.Fatalf("[main.go::main] NewWebServer: %v", err)
	}

	//run dns server
	{
		wg.Add(1)
		go func() {
			defer wg.Done()
			dns.Run()
		}()
	}

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Kill, os.Interrupt)
	<-sigCh

	dns.Shutdown()
	store.Close()
	web.Shutdown(context.Background())

	wg.Wait()

	fmt.Println()
	return subcommands.ExitSuccess
}
