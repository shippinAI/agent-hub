package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type lease struct {
	Claimed bool  `json:"claimed"`
	Fence   int64 `json:"fence"`
	Task    struct {
		ID      int64  `json:"id"`
		Title   string `json:"title"`
		State   string `json:"state"`
		Version int64  `json:"version"`
	} `json:"task"`
	LeaseExpiresAt string `json:"lease_expires_at"`
	HeartbeatEvery int    `json:"heartbeat_every_seconds"`
	Message        string `json:"message"`
}

func run(c *client, cmd string, pos []string, f map[string]string) error {
	st := loadState()
	var out map[string]any
	switch cmd {
	case "whoami":
		return c.get("/me")
	case "ready", "ls", "search":
		q := url.Values{"project": {f["project"]}, "state": {f["state"]}}
		if cmd == "ready" {
			q.Set("ready", "1")
		}
		if f["mine"] == "true" {
			q.Set("holder", "me")
		}
		if cmd == "search" {
			q.Set("q", strings.Join(pos, " "))
		}
		return c.list("/tasks?" + q.Encode())
	case "show":
		if err := need(pos, 1); err != nil {
			return err
		}
		return c.get("/tasks/" + pos[0])
	case "new":
		if err := need(pos, 1); err != nil {
			return err
		}
		body := map[string]any{"project": or(f["project"], "general"), "title": strings.Join(pos, " "), "description": f["desc"]}
		if f["labels"] != "" {
			body["labels"] = strings.Split(f["labels"], ",")
		}
		if p, err := strconv.Atoi(f["priority"]); err == nil {
			body["priority"] = p
		}
		return c.post("/tasks", body, nil)
	case "claim":
		ttl, _ := strconv.Atoi(f["ttl"])
		var l lease
		path := "/claim-next"
		if len(pos) > 0 {
			path = "/tasks/" + pos[0] + "/claim"
		}
		if err := c.do("POST", path, map[string]any{"ttl_seconds": ttl, "project": f["project"]}, &l); err != nil {
			return err
		}
		if !l.Claimed {
			fmt.Println(or(l.Message, "nothing to claim"))
			return nil
		}
		st.put(l.Task.ID, l.Fence, l.Task.Version)
		fmt.Printf("claimed #%d %q state=%s fence=%d lease_until=%s heartbeat_every=%ds\n",
			l.Task.ID, l.Task.Title, l.Task.State, l.Fence, l.LeaseExpiresAt, l.HeartbeatEvery)
		return nil
	case "beat", "release", "done", "handoff", "note":
		if err := need(pos, 1); err != nil {
			return err
		}
		id, text := pos[0], strings.Join(pos[1:], " ")
		body := map[string]any{"fence": st.fence(id)}
		op := map[string]string{"beat": "heartbeat", "release": "release", "done": "complete", "handoff": "handoff", "note": "notes"}[cmd]
		switch cmd {
		case "release":
			body["reason"] = text
		case "done":
			body["summary"] = text
		case "handoff":
			body["note"], body["to"] = text, f["to"]
		case "note":
			body = map[string]any{"body": text, "kind": or(f["kind"], "note")}
			if fe := st.fence(id); fe > 0 {
				body["fence"] = fe
			}
		}
		if err := c.post("/tasks/"+id+"/"+op, body, &out); err != nil {
			return err
		}
		if cmd == "release" || cmd == "done" || cmd == "handoff" {
			st.drop(id)
		}
		show(out)
		return nil
	case "events":
		return c.get("/events?after_seq=" + or(f["after"], "0"))
	case "verify":
		return c.get("/events/verify")
	case "protocol":
		var p struct {
			AgentsMD string `json:"agents_md"`
		}
		if err := c.do("GET", "/protocol", nil, &p); err != nil {
			return err
		}
		fmt.Println(p.AgentsMD)
		return nil
	}
	return fmt.Errorf("unknown command %q\n%s", cmd, usage)
}
