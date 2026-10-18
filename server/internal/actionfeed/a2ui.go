package actionfeed

// A2UI v0.9 carriage for the action feed (21-a2ui.md, 39-action-feed.md). The surface is a List whose
// template is our catalog's ActionCard; each event is one item of the /actions array in the data model.

// CatalogID is the Devagotchi A2UI catalog the ActionCard component belongs to.
const CatalogID = "https://devagotchi.com/a2ui/catalog/v1"

// SurfaceID names the feed surface.
const SurfaceID = "action-feed"

type msg = map[string]any

func wrap(key string, body msg) msg { return msg{"version": "v0.9", key: body} }

// A2UI returns the JSONL messages that render events on an A2UI renderer with our catalog.
func A2UI(events []Event) []msg {
	bind := func(p string) msg { return msg{"path": p} }
	card := msg{"id": "action_card", "component": "ActionCard"}
	for _, f := range []string{"id", "at", "kind", "category", "actor", "machine", "op", "tool", "argv", "cwd", "paths", "args",
		"diff", "status", "exit_code", "duration_ms", "stdout", "stderr", "truncated", "error", "policy", "redactions"} {
		card[f] = bind(f) // relative paths resolve against /actions/N
	}
	comps := []msg{
		{"id": "root", "component": "Column", "children": []string{"feed_title", "feed_list"}},
		{"id": "feed_title", "component": "Text", "text": "Agent actions", "variant": "h2"},
		{"id": "feed_list", "component": "List", "children": msg{"path": "/actions", "componentId": "action_card"}},
		card,
	}
	if events == nil {
		events = []Event{}
	}
	return []msg{
		wrap("createSurface", msg{"surfaceId": SurfaceID, "catalogId": CatalogID}),
		wrap("updateComponents", msg{"surfaceId": SurfaceID, "components": comps}),
		wrap("updateDataModel", msg{"surfaceId": SurfaceID, "path": "/actions", "value": events}),
	}
}
