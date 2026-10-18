package hubtest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestRealBridgeGUI drives the real X11 GUI helper on an Xvfb display: capture a screenshot (image bytes +
// dimensions) and send a real click, both under a hub grant.
func TestRealBridgeGUI(t *testing.T) {
	for _, b := range []string{"bwrap", "Xvfb", "xmessage"} {
		if _, err := exec.LookPath(b); err != nil {
			t.Skipf("%s not installed", b)
		}
	}
	disp := ":97"
	xvfb := exec.Command("Xvfb", disp, "-screen", "0", "1280x800x24", "-nolisten", "tcp")
	if err := xvfb.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = xvfb.Process.Kill(); _, _ = xvfb.Process.Wait() })
	time.Sleep(700 * time.Millisecond)
	_ = exec.Command("xsetroot", "-display", disp, "-solid", "#1e3a8a").Run()

	bin := buildBridge(t)
	e := setup(t)
	dir := e.startRealBridge(t, bin, "guipc")

	// Start the desktop helper on the virtual display, same config dir as the service.
	helper := exec.Command(bin, "gui-agent")
	helper.Env = append(os.Environ(), "DEVAGOTCHI_CONFIG_DIR="+dir, "DISPLAY="+disp)
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = helper.Process.Kill(); _, _ = helper.Process.Wait() })

	cs := e.mcpSession("claude-code")
	// Observe grant (capture) and an input grant naming xmessage.
	obs, _ := callTool(t, cs, "request_access", map[string]any{"machine": "guipc", "capability": "gui.observe",
		"selector":      map[string]any{"monitors": []string{"primary"}, "windows": "any"},
		"sandbox_level": "desktop", "duration": "1h", "reason": "See the virtual screen for the GUI e2e run."})
	approve(t, e, reqID(t, obs))
	inp, _ := callTool(t, cs, "request_access", map[string]any{"machine": "guipc", "capability": "gui.input",
		"selector":      map[string]any{"apps": []string{"xmessage"}},
		"sandbox_level": "desktop", "duration": "1h", "reason": "Click the xmessage button for the GUI e2e run."})
	approve(t, e, reqID(t, inp))

	ss, isErr := callTool(t, cs, "gui_session_start", map[string]any{"machine": "guipc"})
	if isErr {
		t.Fatalf("gui_session_start: %v", ss)
	}
	sess, _ := ss["session"].(map[string]any)
	sid, _ := sess["gui_session_id"].(string)
	fence := numOf(sess["fence"])
	if sid == "" {
		t.Fatalf("no session id: %v", ss)
	}

	evdir := "../../test-evidence/screenshots"
	_ = os.MkdirAll(evdir, 0o755)

	// Launch a clickable xmessage, capture the screen while it is visible, then click its OK button.
	xm := exec.Command("xmessage", "-geometry", "300x120+200+200", "-buttons", "ok:0", "-default", "ok", "hello devagotchi")
	xm.Env = append(os.Environ(), "DISPLAY="+disp)
	if err := xm.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(900 * time.Millisecond)
	exited := make(chan struct{})
	go func() { _, _ = xm.Process.Wait(); close(exited) }()

	shot := e.guiScreenshot(t, cs, sid)
	if err := os.WriteFile(filepath.Join(evdir, "devagotchi-e2e-gui-capture.png"), shot, 0o644); err != nil {
		t.Fatal(err)
	}
	if len(shot) < 1000 || string(shot[1:4]) != "PNG" {
		t.Fatalf("screenshot not a PNG (%d bytes)", len(shot))
	}

	// Click near the centre where the OK button sits.
	_, isErr = callTool(t, cs, "gui_input", map[string]any{"gui_session_id": sid, "fence": fence, "app": "xmessage",
		"actions": []map[string]any{{"type": "click", "x": 230, "y": 300, "button": 1}}})
	if isErr {
		// still capture evidence before failing
		s2 := e.guiScreenshot(t, cs, sid)
		_ = os.WriteFile(filepath.Join(evdir, "devagotchi-e2e-gui-after-click.png"), s2, 0o644)
		t.Logf("gui_input returned error payload; click coords may miss the button")
	}
	s2 := e.guiScreenshot(t, cs, sid)
	_ = os.WriteFile(filepath.Join(evdir, "devagotchi-e2e-gui-after-click.png"), s2, 0o644)

	select {
	case <-exited:
		// clicked OK -> process exited: real synthetic input reached the app.
	case <-time.After(3 * time.Second):
		_ = xm.Process.Kill()
		t.Fatal("xmessage did not exit after the click (synthetic input did not land on the button)")
	}
}

func approve(t *testing.T, e *env, id string) {
	t.Helper()
	r := e.call("jesse", "POST", "/access-requests/"+id+"/approve", nil)
	if r.Status != 200 {
		t.Fatalf("approve %s: %d %v", id, r.Status, r.Body)
	}
}

func (e *env) guiScreenshot(t *testing.T, cs *mcp.ClientSession, sid string) []byte {
	t.Helper()
	r, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "gui_screenshot",
		Arguments: map[string]any{"gui_session_id": sid, "max_edge": 1280}})
	if err != nil || r.IsError {
		msg := ""
		for _, c := range r.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				msg += tc.Text
			}
		}
		t.Fatalf("gui_screenshot err=%v msg=%s", err, msg)
	}
	for _, c := range r.Content {
		if ic, ok := c.(*mcp.ImageContent); ok {
			return ic.Data
		}
	}
	t.Fatalf("gui_screenshot returned no image: %v", r)
	return nil
}

func numOf(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	}
	return 0
}
