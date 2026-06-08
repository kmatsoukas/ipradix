package ipradix_test

import (
	"fmt"
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
	err := table.Insert(ipradix.Prefix[exampleMetadata]{
		Prefix: netip.MustParsePrefix("203.0.113.0/24"),
		Routes: []ipradix.Route[exampleMetadata]{
			{
				ID:       1,
				NextHop:  netip.MustParseAddr("192.0.2.254"),
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
		},
	})
	if err != nil {
		panic(err)
	}

	match, ok := table.Find(netip.MustParseAddr("203.0.113.42"))
	fmt.Println(match.Prefix, match.Routes[0].Metadata.Country, match.Routes[0].Metadata.Whois.NetName, ok)

	// Output:
	// 203.0.113.0/24 US TEST-NET-3 true
}
