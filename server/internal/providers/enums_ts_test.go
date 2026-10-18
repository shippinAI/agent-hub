package providers

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// The web UI's enum lists (web/src/lib/providerTypes.ts) must match the validator's.
func TestEnumsMatchWebTypes(t *testing.T) {
	b, err := os.ReadFile("../../../web/src/lib/providerTypes.ts")
	if err != nil {
		t.Skip("web sources not present")
	}
	re := regexp.MustCompile(`(?m)^export const ([A-Z_]+) = \[(.*)\] as const;$`)
	ts := map[string][]string{}
	for _, m := range re.FindAllStringSubmatch(string(b), -1) {
		var vals []string
		for _, v := range strings.Split(m[2], ",") {
			vals = append(vals, strings.Trim(strings.TrimSpace(v), "'"))
		}
		ts[m[1]] = vals
	}
	var caps []string
	ct := reflect.TypeOf(Capabilities{})
	for i := 0; i < ct.NumField(); i++ {
		caps = append(caps, strings.Split(ct.Field(i).Tag.Get("json"), ",")[0])
	}
	want := map[string][]string{"ADAPTER_KINDS": AdapterKinds, "API_ADAPTERS": APIAdapters, "ROUTER_MODES": RouterModes,
		"AUTH_KINDS": AuthKinds, "RUNNERS": Runners, "EDITIONS": Editions, "OSES": OSes, "MODEL_SOURCES": ModelSources,
		"TOKEN_SOURCES": TokenSources, "QUOTA_SOURCES": QuotaSources, "COST_BASES": CostBases, "STATE_READS": StateReads,
		"CAPABILITIES": caps}
	for name, w := range want {
		if !reflect.DeepEqual(ts[name], w) {
			t.Errorf("%s: web %v, Go %v", name, ts[name], w)
		}
	}
}
