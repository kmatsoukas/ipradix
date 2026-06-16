package ipradix

import (
	"encoding/json"
	"net/netip"
	"testing"
)

type routeJSONMetadata struct {
	Label string `json:"label"`
}

func TestRouteAndPrefixJSONTags(t *testing.T) {
	entry := Prefix[routeJSONMetadata]{
		Prefix: netip.MustParsePrefix("203.0.113.0/24"),
		Routes: []Route[routeJSONMetadata]{
			{
				ID:                  1,
				RouterID:            netip.MustParseAddr("192.0.2.1"),
				NextHop:             netip.MustParseAddr("192.0.2.254"),
				PeerAS:              64512,
				OriginAS:            64496,
				ASPath:              []uint32{64512, 64496},
				Communities:         []uint32{100},
				ExtendedCommunities: []uint64{200},
				LargeCommunities: []LargeCommunity{
					{
						GlobalAdministrator: 64512,
						LocalData1:          1,
						LocalData2:          2,
					},
				},
				LocalPreference: 100,
				MED:             50,
				Origin:          OriginIGP,
				Metadata:        routeJSONMetadata{Label: "primary"},
			},
		},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}

	var prefix map[string]json.RawMessage
	if err := json.Unmarshal(data, &prefix); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}
	assertJSONKeys(t, prefix, []string{"prefix", "routes"})
	if _, ok := prefix["Prefix"]; ok {
		t.Fatal("Marshal() used exported Go field name Prefix")
	}

	var routes []map[string]json.RawMessage
	if err := json.Unmarshal(prefix["routes"], &routes); err != nil {
		t.Fatalf("Unmarshal(routes) failed: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("route count = %d; want 1", len(routes))
	}
	assertJSONKeys(t, routes[0], []string{
		"id",
		"routerId",
		"nextHop",
		"peerAs",
		"originAs",
		"asPath",
		"communities",
		"extendedCommunities",
		"largeCommunities",
		"localPreference",
		"med",
		"origin",
		"metadata",
	})
	if _, ok := routes[0]["RouterID"]; ok {
		t.Fatal("Marshal() used exported Go field name RouterID")
	}

	var largeCommunities []map[string]json.RawMessage
	if err := json.Unmarshal(routes[0]["largeCommunities"], &largeCommunities); err != nil {
		t.Fatalf("Unmarshal(largeCommunities) failed: %v", err)
	}
	if len(largeCommunities) != 1 {
		t.Fatalf("large community count = %d; want 1", len(largeCommunities))
	}
	assertJSONKeys(t, largeCommunities[0], []string{
		"globalAdministrator",
		"localData1",
		"localData2",
	})
}

func assertJSONKeys(t *testing.T, object map[string]json.RawMessage, keys []string) {
	t.Helper()

	if len(object) != len(keys) {
		t.Fatalf("JSON key count = %d; want %d: %v", len(object), len(keys), object)
	}
	for _, key := range keys {
		if _, ok := object[key]; !ok {
			t.Fatalf("JSON key %q missing from %v", key, object)
		}
	}
}
