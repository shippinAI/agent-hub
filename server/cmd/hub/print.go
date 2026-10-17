package main

import "fmt"

func (c *client) get(path string) error {
	var out any
	if err := c.do("GET", path, nil, &out); err != nil {
		return err
	}
	show(out)
	return nil
}

func (c *client) post(path string, body any, out *map[string]any) error {
	var v map[string]any
	if err := c.do("POST", path, body, &v); err != nil {
		return err
	}
	if out != nil {
		*out = v
		return nil
	}
	show(v)
	return nil
}

func (c *client) list(path string) error {
	var r struct {
		Tasks []struct {
			ID       int64    `json:"id"`
			Title    string   `json:"title"`
			State    string   `json:"state"`
			Priority int      `json:"priority"`
			Holder   *string  `json:"holder"`
			Labels   []string `json:"labels"`
		} `json:"tasks"`
	}
	if err := c.do("GET", path, nil, &r); err != nil {
		return err
	}
	for _, t := range r.Tasks {
		h := "-"
		if t.Holder != nil {
			h = *t.Holder
		}
		fmt.Printf("#%-5d P%d %-12s %-12s %s %v\n", t.ID, t.Priority, t.State, h, t.Title, t.Labels)
	}
	if len(r.Tasks) == 0 {
		fmt.Println("(no tasks)")
	}
	return nil
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
