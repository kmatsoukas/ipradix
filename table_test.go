package ipradix

import (
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"sync"
	"testing"
)

func TestTableFindAndFindAll(t *testing.T) {
	var table Table[string]
	entries := []Prefix[string]{
		testPrefix("0.0.0.0/0", "v4-default"),
		testPrefix("10.0.0.0/8", "v4-private"),
		testPrefix("10.1.42.99/16", "v4-site"),
		testPrefix("::/0", "v6-default"),
		testPrefix("2001:db8::/32", "v6-doc"),
		testPrefix("2001:db8:1::/48", "v6-site"),
	}
	for _, entry := range entries {
		if err := table.Insert(entry); err != nil {
			t.Fatalf("Insert(%s) failed: %v", entry.Prefix, err)
		}
	}

	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "IPv4 exact family", addr: "10.1.2.3", want: "10.1.0.0/16"},
		{name: "IPv4 fallback", addr: "10.2.3.4", want: "10.0.0.0/8"},
		{name: "IPv4 default", addr: "192.0.2.1", want: "0.0.0.0/0"},
		{name: "mapped IPv4", addr: "::ffff:10.1.2.3", want: "10.1.0.0/16"},
		{name: "IPv6 exact family", addr: "2001:db8:1::1", want: "2001:db8:1::/48"},
		{name: "IPv6 fallback", addr: "2001:db8:2::1", want: "2001:db8::/32"},
		{name: "IPv6 default", addr: "2001:4860:4860::8888", want: "::/0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := table.Find(netip.MustParseAddr(tt.addr))
			if !ok {
				t.Fatalf("Find(%s) did not find a match", tt.addr)
			}
			if got.Prefix.String() != tt.want {
				t.Fatalf("Find(%s) = %s; want %s", tt.addr, got.Prefix, tt.want)
			}
		})
	}

	got := table.FindAll(netip.MustParseAddr("10.1.2.3"))
	want := []string{"10.1.0.0/16", "10.0.0.0/8", "0.0.0.0/0"}
	if prefixes := prefixStrings(got); !reflect.DeepEqual(prefixes, want) {
		t.Fatalf("FindAll() = %v; want %v", prefixes, want)
	}

	if got, ok := table.Find(netip.Addr{}); ok || got.Prefix.IsValid() {
		t.Fatalf("Find(invalid) = (%v, %v); want zero value, false", got, ok)
	}
	if got := table.FindAll(netip.Addr{}); got != nil {
		t.Fatalf("FindAll(invalid) = %v; want nil", got)
	}
}

func TestTableWithoutDefaultReturnsNoMatch(t *testing.T) {
	var table Table[struct{}]
	if err := table.Insert(testPrefixOf[struct{}]("10.0.0.0/8", "private")); err != nil {
		t.Fatal(err)
	}

	if got, ok := table.Find(netip.MustParseAddr("192.0.2.1")); ok {
		t.Fatalf("Find() = %v, true; want no match", got)
	}
	if got := table.FindAll(netip.MustParseAddr("192.0.2.1")); len(got) != 0 {
		t.Fatalf("FindAll() = %v; want no matches", got)
	}
	if _, ok := table.Find(netip.MustParseAddr("2001:db8::1")); ok {
		t.Fatal("IPv4 prefix matched an IPv6 address")
	}
}

func TestInsertValidation(t *testing.T) {
	var table Table[struct{}]
	tests := []struct {
		name   string
		prefix Prefix[struct{}]
		err    error
	}{
		{
			name:   "invalid prefix",
			prefix: Prefix[struct{}]{Routes: []Route[struct{}]{{ID: "route"}}},
			err:    ErrInvalidPrefix,
		},
		{
			name:   "empty routes",
			prefix: Prefix[struct{}]{Prefix: netip.MustParsePrefix("10.0.0.0/8")},
			err:    ErrEmptyRoutes,
		},
		{
			name: "empty route ID",
			prefix: Prefix[struct{}]{
				Prefix: netip.MustParsePrefix("10.0.0.0/8"),
				Routes: []Route[struct{}]{{}},
			},
			err: ErrEmptyRouteID,
		},
		{
			name: "duplicate route ID",
			prefix: Prefix[struct{}]{
				Prefix: netip.MustParsePrefix("10.0.0.0/8"),
				Routes: []Route[struct{}]{{ID: "route"}, {ID: "route"}},
			},
			err: ErrDuplicateRouteID,
		},
		{
			name: "unrepresentable mapped prefix",
			prefix: Prefix[struct{}]{
				Prefix: netip.MustParsePrefix("::ffff:192.0.2.1/95"),
				Routes: []Route[struct{}]{{ID: "route"}},
			},
			err: ErrInvalidPrefix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := table.Insert(tt.prefix); !errors.Is(err, tt.err) {
				t.Fatalf("Insert() error = %v; want errors.Is(%v)", err, tt.err)
			}
		})
	}
}

func TestCanonicalization(t *testing.T) {
	var table Table[struct{}]
	mapped := Prefix[struct{}]{
		Prefix: netip.MustParsePrefix("::ffff:192.0.2.129/120"),
		Routes: []Route[struct{}]{{
			ID:       "mapped",
			RouterID: netip.MustParseAddr("::ffff:192.0.2.10"),
			NextHop:  netip.MustParseAddr("fe80::1%eth0"),
		}},
	}
	if err := table.Insert(mapped); err != nil {
		t.Fatal(err)
	}

	got, ok := table.Find(netip.MustParseAddr("192.0.2.200"))
	if !ok {
		t.Fatal("Find() did not match normalized mapped IPv4 prefix")
	}
	if got.Prefix.String() != "192.0.2.0/24" {
		t.Fatalf("stored prefix = %s; want 192.0.2.0/24", got.Prefix)
	}
	if got.Routes[0].RouterID.String() != "192.0.2.10" {
		t.Fatalf("router ID = %s; want 192.0.2.10", got.Routes[0].RouterID)
	}
	if got.Routes[0].NextHop.Zone() != "" {
		t.Fatalf("next-hop zone = %q; want empty", got.Routes[0].NextHop.Zone())
	}

	if !table.Delete(netip.MustParsePrefix("192.0.2.99/24")) {
		t.Fatal("Delete() did not normalize host bits")
	}
}

func TestRouteMutations(t *testing.T) {
	var table Table[string]
	prefix := netip.MustParsePrefix("203.0.113.0/24")
	if err := table.Insert(Prefix[string]{
		Prefix: prefix,
		Routes: []Route[string]{
			{ID: "peer-a", MED: 10, Metadata: "a"},
			{ID: "peer-b", MED: 20, Metadata: "b"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := table.UpsertRoute(prefix, Route[string]{ID: "peer-a", MED: 30, Metadata: "updated"}); err != nil {
		t.Fatal(err)
	}
	if err := table.UpsertRoute(prefix, Route[string]{ID: "peer-c", MED: 40}); err != nil {
		t.Fatal(err)
	}

	got, ok := table.Find(netip.MustParseAddr("203.0.113.1"))
	if !ok {
		t.Fatal("Find() did not find prefix")
	}
	if ids := routeIDs(got.Routes); !reflect.DeepEqual(ids, []string{"peer-a", "peer-b", "peer-c"}) {
		t.Fatalf("route IDs = %v; want replacement in place and append", ids)
	}
	if got.Routes[0].MED != 30 || got.Routes[0].Metadata != "updated" {
		t.Fatalf("replacement route = %+v; want updated values", got.Routes[0])
	}

	if table.DeleteRoute(prefix, "missing") {
		t.Fatal("DeleteRoute() deleted an unknown route")
	}
	if !table.DeleteRoute(prefix, "peer-b") || !table.DeleteRoute(prefix, "peer-a") {
		t.Fatal("DeleteRoute() failed for an existing route")
	}
	if !table.DeleteRoute(prefix, "peer-c") {
		t.Fatal("DeleteRoute() failed for final route")
	}
	if _, ok := table.Find(netip.MustParseAddr("203.0.113.1")); ok {
		t.Fatal("prefix remained after its final route was deleted")
	}

	if err := table.UpsertRoute(prefix, Route[string]{ID: "new"}); err != nil {
		t.Fatalf("UpsertRoute() failed to create prefix: %v", err)
	}
	if _, ok := table.Find(netip.MustParseAddr("203.0.113.1")); !ok {
		t.Fatal("UpsertRoute() did not create a missing prefix")
	}
	if err := table.UpsertRoute(prefix, Route[string]{}); !errors.Is(err, ErrEmptyRouteID) {
		t.Fatalf("UpsertRoute() error = %v; want ErrEmptyRouteID", err)
	}
}

func TestMutationInvalidInputs(t *testing.T) {
	var table Table[struct{}]
	invalid := netip.Prefix{}
	valid := netip.MustParsePrefix("10.0.0.0/8")

	if err := table.UpsertRoute(invalid, Route[struct{}]{ID: "route"}); !errors.Is(err, ErrInvalidPrefix) {
		t.Fatalf("UpsertRoute(invalid) error = %v; want ErrInvalidPrefix", err)
	}
	if table.Delete(invalid) {
		t.Fatal("Delete(invalid) = true; want false")
	}
	if table.DeleteRoute(valid, "") {
		t.Fatal("DeleteRoute(empty ID) = true; want false")
	}
	if table.DeleteRoute(invalid, "route") {
		t.Fatal("DeleteRoute(invalid prefix) = true; want false")
	}
	if table.DeleteRoute(valid, "route") {
		t.Fatal("DeleteRoute(missing prefix) = true; want false")
	}

	if err := table.Insert(testPrefixOf[struct{}]("10.0.0.0/8", "route")); err != nil {
		t.Fatal(err)
	}
	if table.DeleteRoute(netip.MustParsePrefix("10.1.0.0/16"), "route") {
		t.Fatal("DeleteRoute(missing exact prefix) = true; want false")
	}
}

func TestHostPrefixMatches(t *testing.T) {
	var table Table[struct{}]
	entries := []Prefix[struct{}]{
		testPrefixOf[struct{}]("192.0.2.1/32", "v4-host"),
		testPrefixOf[struct{}]("2001:db8::1/128", "v6-host"),
	}
	for _, entry := range entries {
		if err := table.Insert(entry); err != nil {
			t.Fatal(err)
		}
	}

	for _, addr := range []string{"192.0.2.1", "2001:db8::1"} {
		parsed := netip.MustParseAddr(addr)
		match, ok := table.Find(parsed)
		if !ok || match.Prefix.Bits() != parsed.BitLen() {
			t.Fatalf("Find(%s) = (%s, %v); want host prefix", addr, match.Prefix, ok)
		}
		all := table.FindAll(parsed)
		if len(all) != 1 || all[0].Prefix.Bits() != parsed.BitLen() {
			t.Fatalf("FindAll(%s) = %v; want host prefix", addr, prefixStrings(all))
		}
	}
}

func TestInsertReplacesExactPrefix(t *testing.T) {
	var table Table[struct{}]
	if err := table.Insert(testPrefixOf[struct{}]("10.0.0.0/8", "old")); err != nil {
		t.Fatal(err)
	}
	if err := table.Insert(testPrefixOf[struct{}]("10.0.0.0/8", "new")); err != nil {
		t.Fatal(err)
	}

	got, ok := table.Find(netip.MustParseAddr("10.1.2.3"))
	if !ok {
		t.Fatal("Find() did not find replaced prefix")
	}
	if ids := routeIDs(got.Routes); !reflect.DeepEqual(ids, []string{"new"}) {
		t.Fatalf("route IDs = %v; want [new]", ids)
	}
}

func TestDeletePreservesOtherBranches(t *testing.T) {
	var table Table[struct{}]
	for _, prefix := range []string{"10.0.0.0/8", "10.0.0.0/9", "10.128.0.0/9", "10.64.0.0/10"} {
		if err := table.Insert(testPrefixOf[struct{}](prefix, prefix)); err != nil {
			t.Fatal(err)
		}
	}

	if !table.Delete(netip.MustParsePrefix("10.0.0.0/9")) {
		t.Fatal("Delete() failed")
	}
	got, ok := table.Find(netip.MustParseAddr("10.1.1.1"))
	if !ok || got.Prefix.String() != "10.0.0.0/8" {
		t.Fatalf("Find() after delete = (%v, %v); want 10.0.0.0/8", got.Prefix, ok)
	}
	got, ok = table.Find(netip.MustParseAddr("10.70.1.1"))
	if !ok || got.Prefix.String() != "10.64.0.0/10" {
		t.Fatalf("sibling prefix was damaged: (%v, %v)", got.Prefix, ok)
	}
	got, ok = table.Find(netip.MustParseAddr("10.200.1.1"))
	if !ok || got.Prefix.String() != "10.128.0.0/9" {
		t.Fatalf("other branch was damaged: (%v, %v)", got.Prefix, ok)
	}
	if table.Delete(netip.MustParsePrefix("10.0.0.0/9")) {
		t.Fatal("Delete() reported a second deletion")
	}
}

func TestRouteDataIsCopied(t *testing.T) {
	var table Table[string]
	route := Route[string]{
		ID:                  "route",
		ASPath:              []uint32{64512, 64496},
		Communities:         []uint32{100},
		ExtendedCommunities: []uint64{200},
		LargeCommunities:    []LargeCommunity{{GlobalAdministrator: 64512, LocalData1: 1, LocalData2: 2}},
		Metadata:            "immutable",
	}
	entry := Prefix[string]{
		Prefix: netip.MustParsePrefix("198.51.100.0/24"),
		Routes: []Route[string]{route},
	}
	if err := table.Insert(entry); err != nil {
		t.Fatal(err)
	}

	entry.Routes[0].ID = "mutated"
	route.ASPath[0] = 1
	route.Communities[0] = 2
	route.ExtendedCommunities[0] = 3
	route.LargeCommunities[0].LocalData1 = 4

	got, ok := table.Find(netip.MustParseAddr("198.51.100.1"))
	if !ok {
		t.Fatal("Find() did not find prefix")
	}
	if got.Routes[0].ID != "route" ||
		got.Routes[0].ASPath[0] != 64512 ||
		got.Routes[0].Communities[0] != 100 ||
		got.Routes[0].ExtendedCommunities[0] != 200 ||
		got.Routes[0].LargeCommunities[0].LocalData1 != 1 {
		t.Fatalf("stored route was changed through input aliases: %+v", got.Routes[0])
	}

	got.Routes[0].ID = "output-mutated"
	got.Routes[0].ASPath[0] = 5
	again, ok := table.Find(netip.MustParseAddr("198.51.100.1"))
	if !ok || again.Routes[0].ID != "route" || again.Routes[0].ASPath[0] != 64512 {
		t.Fatalf("stored route was changed through output aliases: %+v", again.Routes[0])
	}
}

func TestConcurrentAccess(t *testing.T) {
	var table Table[int]
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	if err := table.UpsertRoute(prefix, Route[int]{ID: "base"}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := fmt.Sprintf("worker-%d", worker)
			for i := 0; i < 500; i++ {
				if worker%2 == 0 {
					if err := table.UpsertRoute(prefix, Route[int]{ID: id, MED: uint32(i), Metadata: i}); err != nil {
						t.Errorf("UpsertRoute() failed: %v", err)
						return
					}
					if i%3 == 0 {
						table.DeleteRoute(prefix, id)
					}
					continue
				}
				if _, ok := table.Find(netip.MustParseAddr("10.1.2.3")); !ok {
					t.Error("Find() missed permanent base route")
					return
				}
				table.FindAll(netip.MustParseAddr("10.1.2.3"))
			}
		}()
	}
	wg.Wait()
}

func testPrefix(prefix, routeID string) Prefix[string] {
	return testPrefixOf[string](prefix, routeID)
}

func testPrefixOf[M any](prefix, routeID string) Prefix[M] {
	return Prefix[M]{
		Prefix: netip.MustParsePrefix(prefix),
		Routes: []Route[M]{{ID: routeID}},
	}
}

func prefixStrings[M any](prefixes []Prefix[M]) []string {
	result := make([]string, len(prefixes))
	for i := range prefixes {
		result[i] = prefixes[i].Prefix.String()
	}
	return result
}

func routeIDs[M any](routes []Route[M]) []string {
	result := make([]string, len(routes))
	for i := range routes {
		result[i] = routes[i].ID
	}
	return result
}
