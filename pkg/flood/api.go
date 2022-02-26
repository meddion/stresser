package flood

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	DIAL_TIMEOUT    = time.Second * 5
	TEST_PROXY_SITE = "https://google.com" // Used to check if a proxy server is responding
	CHECKSUM_SIZE   = 256
)
const (
	APIv1 = iota + 1
	APIv2
)

type (
	Target struct {
		ID           int    `json:"-"`
		URL          string `json:"url"`
		Page         string `json:"page"`
		NeedParseURL int    `json:"need_parse_url"`
		Attack       int    `json:"atack"`
	}

	Proxy struct {
		ID   int    `json:"id"`
		IP   string `json:"ip"`
		Auth string `json:"auth"`
	}

	srcDataAPIv1 struct {
		Target  Target  `json:"site"`
		Proxies []Proxy `json:"proxy"`
	}

	srcDataAPIv2 struct {
		Targets []Target `json:"site"`
		Proxies []Proxy  `json:"proxy"`
	}
)

var DefClient *http.Client

func init() {
	// Setup
	rand.Seed(time.Now().UnixNano())
	// Default http client
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	// tr.IdleConnTimeout = time.Second * 5
	tr.ResponseHeaderTimeout = time.Second * 5
	tr.MaxConnsPerHost = 0 // no limit
	tr.DisableKeepAlives = true
	tr.Dial = (&net.Dialer{
		Timeout: DIAL_TIMEOUT,
	}).Dial
	DefClient = &http.Client{Transport: tr, Timeout: DIAL_TIMEOUT}
	// Logger
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	})
}

func newReq(ctx context.Context, method string, target string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return nil, err
	}
	// TODO: add some useful headers
	req.Header.Add("User-Agent", GetUserAgent())
	req.Header.Add("Cache-Control", "no-store, max-age=0")
	req.Header.Add("Accept", "application/json, text/plain, */*")
	req.Header.Add("Accept-Language", "ru")
	req.Header.Add("x-forward-proto", "https")

	// dumpedBody, _ := httputil.DumpRequest(req, true)

	return req, nil
}

func toDevNull(readCloser io.ReadCloser) error {
	defer readCloser.Close()
	_, err := ioutil.ReadAll(readCloser)
	if err != nil {
		return err
	}
	return nil
}

func GetSrcFromGateway(rootCtx context.Context, gateway string) (string, error) {
	gCtx, cancel := context.WithTimeout(rootCtx, DIAL_TIMEOUT)
	defer cancel()

	req, err := newReq(gCtx, "GET", gateway)
	if err != nil {
		return "", err
	}

	resp, err := DefClient.Do(req)
	if err != nil {
		return "", err
	}
	defer toDevNull(resp.Body)

	dec := json.NewDecoder(resp.Body)
	var sources []string
	if err := dec.Decode(&sources); err != nil {
		return "", err
	}

	if len(sources) > 0 {
		return sources[rand.Intn(len(sources))], nil
	}

	return "", nil
}

func GetProxy(rootCtx context.Context, proxySrcAddr string) ([]Proxy, error) {
	srcCtx, cancel := context.WithTimeout(rootCtx, DIAL_TIMEOUT)
	defer cancel()

	req, err := newReq(srcCtx, "GET", proxySrcAddr)
	if err != nil {
		return nil, err
	}

	resp, err := DefClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer toDevNull(resp.Body)

	b := new(bytes.Buffer)
	if _, err := b.ReadFrom(resp.Body); err != nil {
		return nil, err
	}

	var proxies []Proxy
	if err := json.Unmarshal(b.Bytes(), &proxies); err != nil {
		var stringProxies []string
		if err := json.Unmarshal(b.Bytes(), &stringProxies); err != nil {
			return nil, err
		}
		proxies = make([]Proxy, len(stringProxies))
		for i := range stringProxies {
			proxies[i] = Proxy{IP: stringProxies[i]}
		}
	}

	return proxies, nil
}

func GetTargets(rootCtx context.Context, targetSrcAddr string) ([]Target, error) {
	srcCtx, cancel := context.WithTimeout(rootCtx, DIAL_TIMEOUT)
	defer cancel()

	req, err := newReq(srcCtx, "GET", targetSrcAddr)
	if err != nil {
		return nil, err
	}

	resp, err := DefClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer toDevNull(resp.Body)

	b := new(bytes.Buffer)
	if _, err := b.ReadFrom(resp.Body); err != nil {
		return nil, err
	}

	var targets []Target
	if err := json.Unmarshal(b.Bytes(), &targets); err != nil {
		var stringTargets []string
		if err := json.Unmarshal(b.Bytes(), &stringTargets); err != nil {
			return nil, err
		}
		targets = make([]Target, len(stringTargets))
		for i := range stringTargets {
			targets[i] = Target{URL: stringTargets[i]}
		}
	}

	return targets, nil
}

func GetTargetData(rootCtx context.Context, src string, apiVer int) ([]Target, []Proxy, error) {
	srcCtx, cancel := context.WithTimeout(rootCtx, DIAL_TIMEOUT)
	defer cancel()

	req, err := newReq(srcCtx, "GET", src)
	if err != nil {
		return nil, nil, err
	}

	resp, err := DefClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer toDevNull(resp.Body)

	return decodeJSON(resp.Body, apiVer)
}

func GetTargetDataFromFile(rootCtx context.Context, path string, apiVer int) ([]Target, []Proxy, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	return decodeJSON(bufio.NewReader(file), apiVer)
}

func decodeJSON(content io.Reader, apiVer int) ([]Target, []Proxy, error) {
	dec := json.NewDecoder(content)
	switch apiVer {
	case APIv1:
		srcResp := new(srcDataAPIv1)
		if err := dec.Decode(srcResp); err != nil {
			return nil, nil, err
		}
		return []Target{srcResp.Target}, srcResp.Proxies, nil
	case APIv2:
		srcResp := new(srcDataAPIv2)
		if err := dec.Decode(srcResp); err != nil {
			return nil, nil, err
		}
		return srcResp.Targets, srcResp.Proxies, nil
	}

	log.Fatalf("Версія APIv%d не підтримується\n", apiVer)
	return nil, nil, nil
}
