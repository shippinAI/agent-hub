package hubtest

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSSEStreamsEventsAndResumes(t *testing.T) {
	e := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", e.url+"/api/v1/stream", nil)
	req.Header.Set("Authorization", "Bearer "+e.tokens["jesse"])
	req.Header.Set("Last-Event-ID", "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("stream: %v %v", err, resp)
	}
	defer resp.Body.Close()
	go func() {
		time.Sleep(300 * time.Millisecond)
		e.newTask("codex", "live event", nil)
	}()
	sc := bufio.NewScanner(resp.Body)
	ids := []string{}
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "id: ") {
			ids = append(ids, line[4:])
		}
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"task.created"`) {
			break
		}
	}
	if len(ids) == 0 || ids[0] != "2" {
		t.Fatalf("resume after Last-Event-ID 1 should start at 2: %v", ids)
	}
}
