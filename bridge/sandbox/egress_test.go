//go:build linux

package sandbox_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"

	"devagotchi/bridge/netproxy"
	"devagotchi/bridge/sandbox"
	"devagotchi/bridge/sandbox/sandboxtest"
)

// The test binary doubles as the in-sandbox init (sandbox-init) and as the probe it runs.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "sandbox-init":
			fmt.Fprintln(os.Stderr, sandbox.Init(os.Args[2:]))
			os.Exit(127)
		case "egress-probe":
			os.Exit(probe(os.Args[2]))
		}
	}
	os.Exit(m.Run())
}

// probe CONNECTs to target through the in-sandbox proxy port and prints the HTTP body it gets.
func probe(target string) int {
	c, err := net.Dial("tcp", sandbox.ProxyAddr)
	if err != nil {
		fmt.Println("dial proxy:", err)
		return 3
	}
	defer c.Close()
	fmt.Fprintf(c, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target)
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		fmt.Println("connect:", err)
		return 4
	}
	if resp.StatusCode != 200 {
		fmt.Println("connect status", resp.StatusCode)
		return 5
	}
	fmt.Fprintf(c, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", target)
	r, err := http.ReadResponse(br, nil)
	if err != nil {
		fmt.Println("get:", err)
		return 6
	}
	b, _ := io.ReadAll(r.Body)
	fmt.Print(string(b))
	return 0
}

// TestEgressThroughAllowlistProxy: a confined command reaches an allowed host only through the egress proxy,
// for the whole life of the command, and a host outside the allowlist is refused.
func TestEgressThroughAllowlistProxy(t *testing.T) {
	sandboxtest.Require(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "reached") }))
	self, _ := os.Executable()
	run := func(hosts []string) (string, error) {
		sock, _, stop := netproxy.Start(context.Background(), hosts, nil)
		defer stop()
		c, err := sandbox.Command(sandbox.Spec{Level: "workspace-write", Paths: []string{t.TempDir()}, ProxySock: sock, Self: self,
			Argv: []string{"/run/devagotchi-init", "egress-probe", ln.Addr().String()}})
		if err != nil {
			t.Fatal(err)
		}
		out, err := c.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	if out, err := run([]string{"127.0.0.1"}); err != nil || out != "reached" {
		t.Fatalf("allowed host not reached through the proxy: %q %v", out, err)
	}
	if out, err := run([]string{"example.invalid"}); err == nil || !strings.Contains(out, "403") {
		t.Fatalf("host outside the allowlist was reachable: %q %v", out, err)
	}
}
