# ipradix

[![CI](https://github.com/kmatsoukas/ipradix/actions/workflows/ci.yml/badge.svg)](https://github.com/kmatsoukas/ipradix/actions/workflows/ci.yml)

`ipradix` is a generic, concurrency-safe IPv4 and IPv6 routing table for Go.
It uses separate compressed Patricia radix trees for efficient longest-prefix
lookups.

## Features

- IPv4 and IPv6 support
- Longest-prefix matching with `Find`
- All matching prefixes with `FindAll`
- Multiple BGP routes per prefix
- Atomic prefix replacement and route-level updates
- Generic route metadata
- Safe concurrent reads and writes
- Automatic prefix and IPv4-mapped IPv6 normalization

## Installation

```sh
go get github.com/kmatsoukas/ipradix
```

## Longest-Prefix Match

The zero value of `Table` is ready to use:

```go
package main

import (
	"fmt"
	"net/netip"

	"github.com/kmatsoukas/ipradix"
)

func main() {
	type WhoisInfo struct {
		NetName      string
		Organization string
		Registry     string
	}

	type Metadata struct {
		Country string
		Whois   WhoisInfo
	}

	var table ipradix.Table[Metadata]

	_ = table.Insert(ipradix.Prefix[Metadata]{
		Prefix: netip.MustParsePrefix("10.0.0.0/8"),
		Routes: []ipradix.Route[Metadata]{
			{
				ID:      "peer-a/path-0",
				NextHop: netip.MustParseAddr("192.0.2.1"),
				Metadata: Metadata{
					Country: "US",
					Whois: WhoisInfo{
						NetName:      "PRIVATE-ADDRESS-ABLK-RFC1918-IANA-RESERVED",
						Organization: "Internet Assigned Numbers Authority",
						Registry:     "ARIN",
					},
				},
			},
		},
	})

	_ = table.Insert(ipradix.Prefix[Metadata]{
		Prefix: netip.MustParsePrefix("10.20.0.0/16"),
		Routes: []ipradix.Route[Metadata]{
			{
				ID:      "peer-b/path-0",
				NextHop: netip.MustParseAddr("192.0.2.2"),
				Metadata: Metadata{
					Country: "US",
					Whois: WhoisInfo{
						NetName:      "EXAMPLE-CUSTOMER-NET",
						Organization: "Example Customer, Inc.",
						Registry:     "ARIN",
					},
				},
			},
		},
	})

	match, ok := table.Find(netip.MustParseAddr("10.20.30.40"))
	if ok {
		fmt.Println(match.Prefix)             // 10.20.0.0/16
		fmt.Println(match.Routes[0].NextHop) // 192.0.2.2
		fmt.Println(match.Routes[0].Metadata.Country) // US
	}
}
```

## Find All Matches

`FindAll` returns matches from most specific to least specific:

```go
matches := table.FindAll(netip.MustParseAddr("10.20.30.40"))
for _, match := range matches {
	fmt.Println(match.Prefix)
}

// 10.20.0.0/16
// 10.0.0.0/8
```

IPv4 and IPv6 entries can coexist in the same table:

```go
var table ipradix.Table[Metadata]

_ = table.UpsertRoute(
	netip.MustParsePrefix("2001:db8::/32"),
	ipradix.Route[Metadata]{
		ID:      "ipv6-peer/path-0",
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Metadata: Metadata{
			Country: "US",
			Whois: WhoisInfo{
				NetName:      "DOCUMENTATION",
				Organization: "Internet Assigned Numbers Authority",
				Registry:     "ARIN",
			},
		},
	},
)

match, ok := table.Find(netip.MustParseAddr("2001:db8:42::10"))
```

## BGP Route Attributes

Routes include common BGP attributes and caller-defined metadata:

```go
type WhoisInfo struct {
	NetName      string
	Organization string
	Registry     string
}

type Metadata struct {
	Country string
	Whois   WhoisInfo
}

var table ipradix.Table[Metadata]

err := table.UpsertRoute(
	netip.MustParsePrefix("203.0.113.0/24"),
	ipradix.Route[Metadata]{
		ID:              "peer-192.0.2.1/path-0",
		RouterID:        netip.MustParseAddr("192.0.2.1"),
		NextHop:         netip.MustParseAddr("192.0.2.254"),
		PeerAS:          64512,
		OriginAS:        64496,
		ASPath:          []uint32{64512, 64496},
		Communities:     []uint32{100},
		LocalPreference: 100,
		MED:             50,
		Origin:          ipradix.OriginIGP,
		Metadata: Metadata{
			Country: "US",
			Whois: WhoisInfo{
				NetName:      "TEST-NET-3",
				Organization: "Internet Assigned Numbers Authority",
				Registry:     "ARIN",
			},
		},
	},
)
if err != nil {
	panic(err)
}
```

`Route.ID` is supplied by the caller and identifies a route within a prefix.
A peer or session identity combined with the BGP ADD-PATH identifier is a
typical choice:

```text
peer-192.0.2.1/path-0
```

The ID must remain stable across updates. Do not derive it from mutable
attributes such as the next hop, AS path, or communities.

## Updating and Deleting Routes

`UpsertRoute` creates the prefix when necessary. For an existing prefix, it
replaces a route with the same ID or appends a route with a new ID:

```go
prefix := netip.MustParsePrefix("203.0.113.0/24")

_ = table.UpsertRoute(prefix, ipradix.Route[Metadata]{
	ID:      "peer-a/path-0",
	NextHop: netip.MustParseAddr("192.0.2.10"),
	Metadata: Metadata{
		Country: "US",
		Whois: WhoisInfo{
			NetName:      "TEST-NET-3",
			Organization: "Internet Assigned Numbers Authority",
			Registry:     "ARIN",
		},
	},
})

table.DeleteRoute(prefix, "peer-a/path-0") // Removes one route.
table.Delete(prefix)                       // Removes the entire prefix.
```

Deleting the final route also removes its prefix.

## Data Ownership

The table copies route and BGP attribute slices during insertion and lookup.
Generic metadata is copied by assignment. Metadata containing maps, slices,
pointers, or other reference types must therefore be treated as immutable
while stored in the table and after lookup.

## Testing

[Task](https://taskfile.dev/) can run the complete test suite verbosely without
using cached results:

```sh
task test
```

Run the test suite with race detection and print a per-function coverage
report:

```sh
task coverage
```

Run the IPv4 and IPv6 lookup benchmarks with allocation statistics:

```sh
task bench
```

Alternatively:

```sh
go test -race -covermode=atomic ./...
```

## License

This project is available under the [MIT License](LICENSE). It permits private,
commercial, and open-source use, modification, and redistribution while
retaining the copyright and license notice.
