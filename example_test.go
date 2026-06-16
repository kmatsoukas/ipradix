package ipradix_test

import (
	"fmt"
	"hash/fnv"
	"net/netip"

	"github.com/kmatsoukas/ipradix"
)

type exampleWhoisInfo struct {
	NetName      string
	Organization string
	Registry     string
}

type exampleMetadata struct {
	Country string
	Whois   exampleWhoisInfo
}

func ExampleTable_Find() {
	var table ipradix.Table[exampleMetadata]
	prefix := netip.MustParsePrefix("203.0.113.0/24")
	nextHop := netip.MustParseAddr("192.0.2.254")
	routerID := netip.MustParseAddr("192.0.2.1")

	err := table.Insert(ipradix.Prefix[exampleMetadata]{
		Prefix: prefix,
		Routes: []ipradix.Route[exampleMetadata]{
			{
				ID:       routeID(prefix, nextHop),
				NextHop:  nextHop,
				PeerAS:   64512,
				OriginAS: 64496,
				ASPath:   []uint32{64512, 64496},
				Metadata: exampleMetadata{
					Country: "US",
					Whois: exampleWhoisInfo{
						NetName:      "TEST-NET-3",
						Organization: "Internet Assigned Numbers Authority",
						Registry:     "ARIN",
					},
				},
			},
			{
				ID:       routeIDWithRouter(routerID, prefix, nextHop),
				RouterID: routerID,
				NextHop:  nextHop,
				PeerAS:   64513,
				OriginAS: 64496,
				ASPath:   []uint32{64513, 64496},
				Metadata: exampleMetadata{
					Country: "US",
					Whois: exampleWhoisInfo{
						NetName:      "TEST-NET-3",
						Organization: "Internet Assigned Numbers Authority",
						Registry:     "ARIN",
					},
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	match, ok := table.Find(netip.MustParseAddr("203.0.113.42"))
	fmt.Println(match.Prefix, len(match.Routes), match.Routes[0].Metadata.Country, match.Routes[0].Metadata.Whois.NetName, ok)

	// Output:
	// 203.0.113.0/24 2 US TEST-NET-3 true
}

func routeID(prefix netip.Prefix, nextHop netip.Addr) uint64 {
	return hashRouteID(prefix.Masked().String(), nextHop.String())
}

func routeIDWithRouter(routerID netip.Addr, prefix netip.Prefix, nextHop netip.Addr) uint64 {
	return hashRouteID(routerID.String(), prefix.Masked().String(), nextHop.String())
}

func hashRouteID(parts ...string) uint64 {
	h := fnv.New64a()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}

	id := h.Sum64()
	if id == 0 {
		return 1
	}
	return id
}
