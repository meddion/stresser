package flood

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// Taken from: https://github.com/Arriven/db1000n
const (
	IP_REGEX  = "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"
	DNS_REGEX = "^(([a-zA-Z]|[a-zA-Z][a-zA-Z0-9\\-]*[a-zA-Z0-9])\\.)*([A-Za-z]|[A-Za-z][A-Za-z0-9\\-]*[A-Za-z0-9])$"
)

func ValidateTargets(ctxRoot context.Context, targets []Target, dnsResolv bool) []Target {
	var wg sync.WaitGroup
	wg.Add(len(targets))
	res := make(chan Target, len(targets))
	validaTargets := make([]Target, 0, len(targets))

	for _, target := range targets {
		go func(target Target) {
			defer wg.Done()

			if target.URL == "" {
				if target.Page != "" {
					target.URL = target.Page
				} else {
					log.Errorln("empty address; no target")
					return
				}
			}

			if url, err := ValidateAddress(ctxRoot, target.URL, dnsResolv); err != nil {
				log.Errorf("on parsing target url %s: %v", url, err)
				return
			} else {
				target.URL = url
				res <- target
			}
		}(target)
	}
	go func() {
		wg.Wait()
		close(res)
	}()

	for t := range res {
		validaTargets = append(validaTargets, t)
	}

	return validaTargets
}

func ValidateAddress(ctxRoot context.Context, addr string, dnsResolve bool) (string, error) {
	CleanupURL := func(targetURL string) string {
		targetURL = strings.Trim(targetURL, "\r")
		targetURL = strings.Trim(targetURL, "\n")
		return strings.TrimFunc(targetURL, func(r rune) bool {
			return r == ' ' || r == '\n' || r == '\r'
		})
	}

	addr = CleanupURL(addr)
	if !strings.Contains(addr, "http") {
		addr = "http://" + addr
	}

	url, err := url.Parse(addr)
	if err != nil {
		return "", err
	}

	var port string
	pair := strings.Split(url.Host, ":")
	if len(pair) > 1 {
		port = pair[1]
	}
	host := pair[0]

	// if dnsResolve && net.ParseIP(host) == nil {}
	if dnsResolve && !isIP(host) && isDNS(host) {
		if host, err = resolveHost(ctxRoot, host); err != nil {
			return "", err
		}
	}

	if port != "" {
		url.Host = host + ":" + port
	} else {
		url.Host = host
	}

	return url.String(), nil
}

func ValidateProxy(rootCtx context.Context, proxies []Proxy) []Proxy {
	checkProxy := func(rootCtx context.Context, proxy Proxy) error {
		bot, err := NewBot(0, func() *Proxy { return &proxy })
		if err != nil {
			return err
		}

		proxyCtx, cancel := context.WithTimeout(rootCtx, time.Second*2)
		defer cancel()

		msgs := make(chan BotResp)
		go bot.Start(proxyCtx, TEST_PROXY_SITE, msgs)

		select {
		case <-proxyCtx.Done():
			return fmt.Errorf("ніякої відповіді від проксі: %s", proxy.IP)
		case msg := <-msgs:
			return msg.Err
		}
	}

	if len(proxies) == 0 {
		return proxies
	}

	type result struct {
		p Proxy
		e error
	}
	resChan := make(chan result)
	validProxies := make([]Proxy, 0, len(proxies))
	go func() {
		for {
			select {
			case <-rootCtx.Done():
				return
			case res, ok := <-resChan:
				if !ok {
					return
				}
				if res.e != nil {
					log.Errorln(res.e)
				} else {
					log.Infof("Валідовано %s", res.p.IP)
					validProxies = append(validProxies, res.p)
				}
			}
		}
	}()

	var wg sync.WaitGroup
	wg.Add(len(proxies))

	for i, proxy := range proxies {
		go func(proxy Proxy) {
			defer wg.Done()
			select {
			case <-rootCtx.Done():
				return
			default:
				select {
				case <-rootCtx.Done():
					return
				case resChan <- result{proxy, checkProxy(rootCtx, proxy)}:
				}
			}
		}(proxy)

		if t := time.NewTimer(time.Second); i%100 == 0 {
			select {
			case <-t.C:
			// Continue since all threads in wg must stop
			case <-rootCtx.Done():
			}
		}

	}
	wg.Wait()
	close(resChan)

	return validProxies
}

func SourceContentTracker(rootCtx context.Context, sourceAddrs []string, changeTracker chan<- string) {
	hasher := sha256.New()
	getChecksum := func(rootCtx context.Context, src string) (b [CHECKSUM_SIZE]byte, err error) {
		srcCtx, cancel := context.WithTimeout(rootCtx, DIAL_TIMEOUT)
		defer cancel()

		req, err := newReq(srcCtx, "GET", src)
		if err != nil {
			return b, err
		}

		resp, err := DefClient.Do(req)
		if err != nil {
			return b, err
		}
		defer toDevNull(resp.Body)

		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return b, err
		}
		hasher.Reset()
		if _, err := hasher.Write(content); err != nil {
			return b, err
		}

		copy(b[:], hasher.Sum(nil))

		return b, nil
	}

	checksums := make(map[string][CHECKSUM_SIZE]byte, len(sourceAddrs))
	// Check if targets have changed each minute
	t := time.NewTicker(time.Minute)
	for {
		for _, src := range sourceAddrs {
			select {
			case <-rootCtx.Done():
				return
			default:
			}

			sum, err := getChecksum(rootCtx, src)
			if err != nil {
				// log.Errorf("Під час отримання checksum від %s: %v", src, err)
				continue
			}

			if s, ok := checksums[src]; ok && sum != s {
				select {
				case <-rootCtx.Done():
				case changeTracker <- src:
				}
				return
			}

			checksums[src] = sum
		}

		select {
		case <-rootCtx.Done():
			return
		case <-t.C:
		}
	}
}

func resolveHost(ctxRoot context.Context, host string) (string, error) {
	ipRecords, err := net.DefaultResolver.LookupIP(ctxRoot, "ip4", host)
	if err != nil {
		return "", err
	}
	ip := ipRecords[0].String()
	if ip == "127.0.0.1" || ip == "0.0.0.0" {
		return "", errors.New("couldn't resolve")
	}

	return ip, nil
}

func isIP(host string) bool {
	res, err := regexp.MatchString(IP_REGEX, host)
	if err != nil {
		// log.Errorf("Під час зіставлення IP regex із %s: %v", host, err)
		return false
	}

	return res
}

func isDNS(host string) bool {
	res, err := regexp.MatchString(DNS_REGEX, host)
	if err != nil {
		// log.Fatalf("ід час зіставлення DNS regex із %s: %v", host, err)
		return false
	}

	return res
}
