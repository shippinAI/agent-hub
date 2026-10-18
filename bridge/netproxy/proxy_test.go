package netproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAllowlistConnect(t *testing.T) {
	up := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))
	defer up.Close()
	host := up.Listener.Addr().String()
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	var denied []string
	p := &Proxy{Hosts: []string{"127.0.0.1"}, OnDial: func(h string, ok bool) {
		if !ok {
			denied = append(denied, h)
		}
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go p.Listen(ctx, ln)
	time.Sleep(50 * time.Millisecond)

	connect := func(target string) (int, error) {
		c, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			return 0, err
		}
		defer c.Close()
		c.Write([]byte("CONNECT " + target + " HTTP/1.1\r\nHost: " + target + "\r\n\r\n"))
		resp, err := http.ReadResponse(bufio.NewReader(c), nil)
		if err != nil {
			return 0, err
		}
		if resp.StatusCode == 200 {
			tc := tls.Client(c, &tls.Config{InsecureSkipVerify: true})
			req, _ := http.NewRequest("GET", "https://"+target+"/", nil)
			_ = req.Write(tc)
		}
		return resp.StatusCode, nil
	}
	if code, _ := connect(host); code != 200 {
		t.Fatalf("allowed host got %d", code)
	}
	if code, _ := connect("example.com:443"); code != 403 {
		t.Fatalf("blocked host got %d (want 403)", code)
	}
	if len(denied) != 1 || denied[0] != "example.com:443" {
		t.Fatalf("denied log: %v", denied)
	}
}
