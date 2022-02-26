package flood

import (
	"context"
	"testing"
)

func TestGetProxy(t *testing.T) {
	proxySrcs := []string{
		"https://raw.githubusercontent.com/opengs/uashieldtargets/master/proxy.json",
		"https://raw.githubusercontent.com/Arriven/db1000n/main/proxylist.json",
	}

	for _, src := range proxySrcs {
		proxies, err := GetProxy(context.TODO(), src)
		if err != nil {
			t.Fatalf("on getting from %s: %v", src, err)
		} else if len(proxies) == 0 {
			t.Fatalf("on getting empty proxy list from %s", src)
		}
	}
}
