package hacker

import (
	"net"
)

// domain
// opType 0 all type，1 ipv4，2 ipv6
func GetDomian2Ips(domain string, opType int) []string {
	ips, _ := net.LookupIP(domain)
	aIps := []string{}
	for _, ip := range ips {
		if 0 == opType || 1 == opType {
			if ipv4 := ip.To4(); ipv4 != nil {
				aIps = append(aIps, ipv4.String())
			}
		}
		if 0 == opType || 2 == opType {
			if ipv6 := ip.To16(); ipv6 != nil {
				aIps = append(aIps, ipv6.String())
			}
		}
	}
	return aIps
}

func GetDomian2IpsAll(domain string) []string {
	return GetDomian2Ips(domain, 0)
}

// get free port
func GetFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}

//
//type DnsJson struct {
//	Status   int  `json:"Status"`
//	TC       bool `json:"TC"`
//	RD       bool `json:"RD"`
//	RA       bool `json:"RA"`
//	AD       bool `json:"AD"`
//	CD       bool `json:"CD"`
//	Question []struct {
//		Name string `json:"name"`
//		Type int    `json:"type"`
//	} `json:"Question"`
//	Answer []struct {
//		Name string `json:"name"`
//		Type int    `json:"type"`
//		TTL  int    `json:"TTL"`
//		Data string `json:"data"`
//	} `json:"Answer"`
//}
//
//func GetDomian2Ips4Cloudflare(domain string) []string {
//	aIps := []string{}
//	req, err := http.NewRequest("GET", "https://cloudflare-dns.com/dns-query", nil)
//	if err != nil {
//		log.Println(err)
//		return aIps
//	}
//
//	q := req.URL.Query()
//	q.Add("ct", "application/dns-json")
//	q.Add("name", domain)
//	req.URL.RawQuery = q.Encode()
//	client := &http.Client{}
//	if err != nil {
//		log.Println(err)
//		return aIps
//	}
//	res, err := client.Do(req)
//	defer res.Body.Close()
//	body, err := ioutil.ReadAll(res.Body)
//	if nil == err {
//		var dnsResult DnsJson
//		if err := json.Unmarshal(body, &dnsResult); err != nil {
//			log.Println(err)
//			return aIps
//		}
//
//		for _, dnsAnswer := range dnsResult.Answer {
//			aIps = append(aIps, dnsAnswer.Data)
//		}
//	}
//	return aIps
//}

//func main() {
//	log.Println(GetDomian2IpsAll("www.sina.com.cn"))
//}
