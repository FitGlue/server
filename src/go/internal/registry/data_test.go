// nolint:proto-json
package registry

import (
	"encoding/json"
	"os"
	"testing"

	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestParseRegistryJSON(t *testing.T) {
	data, err := os.ReadFile("data/registry.json")
	if err != nil {
		t.Fatalf("failed to read registry.json: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	// Because protojson expects strict keys, let's just unmarshal directly with
	// discard unknown in case there are TS-only fields (like functions, though stringify dropped them).

	unmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}

	var resp pbplugin.PluginRegistryResponse
	if err := unmarshaler.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal into proto: %v", err)
	}

	if len(resp.Sources) == 0 {
		t.Fatal("expected \u003e0 sources")
	}
	if len(resp.Enrichers) == 0 {
		t.Fatal("expected \u003e0 enrichers")
	}

	t.Logf("Successfully unmarshaled %d sources, %d enrichers, %d destinations, %d integrations",
		len(resp.Sources), len(resp.Enrichers), len(resp.Destinations), len(resp.Integrations))
}
