// Package catalog is generic, versioned CRUD for the Devagotchi data tables (capabilities, risks, presets, ...).
// The column lists below describe table shapes only; every value in them is data edited in the UI.
// Providers are not here: they have their own schema and validator (internal/providers, doc 34).
package catalog

type colType int

const (
	tText colType = iota
	tTextNull
	tTextArr
	tJSON
	tBool
	tInt
)

type col struct {
	name string
	typ  colType
}

// Kind describes one editable table.
type Kind struct {
	Table    string
	Action   string // policy action prefix; writes need a human principal (store.W.Human) plus this policy
	Cols     []col
	Tenanted bool
}

var kinds = map[string]Kind{
	"capabilities": {Table: "capabilities", Action: "catalog.capabilities", Tenanted: true, Cols: []col{
		{"label", tText}, {"description", tText}, {"selector_schema", tJSON}, {"sandbox_levels", tTextArr}, {"default_duration", tText}}},
	"risks": {Table: "risks", Action: "catalog.risks", Tenanted: true, Cols: []col{
		{"title", tText}, {"plain_text", tText}, {"severity", tText}, {"mitigation", tText}, {"source_url", tTextNull}}},
	"capability-risks": {Table: "capability_risks", Action: "catalog.risks", Tenanted: true, Cols: []col{
		{"capability_id", tText}, {"risk_id", tText}, {"when_selector", tJSON}}},
	"shell-profiles": {Table: "shell_profiles", Action: "catalog.shell_profiles", Tenanted: true, Cols: []col{
		{"os", tText}, {"label", tText}, {"argv_prefix", tJSON}, {"env_allow", tTextArr}, {"cwd_default", tText}, {"pty", tBool}}},
	"grant-presets": {Table: "grant_presets", Action: "preset", Tenanted: true, Cols: []col{
		{"label", tText}, {"description", tText}, {"items", tJSON}, {"duration", tJSON}, {"full_access", tBool}, {"requires_policy", tTextNull}}},
	"docker-runtime-presets": {Table: "docker_runtime_presets", Action: "catalog.runtimes", Tenanted: true, Cols: []col{
		{"os", tText}, {"label", tText}, {"what", tText}, {"facts", tJSON}, {"operator_commands", tJSON}, {"agent_argv", tJSON},
		{"verify_argv", tJSON}, {"hosts", tTextArr}, {"risk_ids", tTextArr}, {"licence_confirm", tBool}}},
	"router-endpoints": {Table: "router_endpoints", Action: "catalog.router_endpoints", Tenanted: true, Cols: []col{
		{"label", tText}, {"url", tText}, {"kind", tText}}},
	"chat-apps": {Table: "chat_apps", Action: "catalog.chat_apps", Cols: []col{
		{"label", tText}, {"provider_ids", tTextArr}, {"connect_template", tText}}, Tenanted: true},
	"tenants": {Table: "tenants", Action: "tenant", Cols: []col{{"name", tText}, {"edition", tText}, {"plan", tText}}},
}

// Kinds lists the URL names of every catalog table.
func Kinds() []string {
	out := make([]string, 0, len(kinds))
	for k := range kinds {
		out = append(out, k)
	}
	return out
}

// Lookup returns a kind by URL name.
func Lookup(name string) (Kind, bool) { k, ok := kinds[name]; return k, ok }
