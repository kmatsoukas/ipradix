package ipradix

import "net/netip"

// Origin is a BGP ORIGIN path attribute.
type Origin uint8

const (
	OriginIGP Origin = iota
	OriginEGP
	OriginIncomplete
)

// LargeCommunity is a BGP large community.
type LargeCommunity struct {
	GlobalAdministrator uint32 `json:"globalAdministrator"`
	LocalData1          uint32 `json:"localData1"`
	LocalData2          uint32 `json:"localData2"`
}

// Route describes a path to a prefix. ID is a nonzero value supplied by the
// caller and must remain stable for the lifetime of the path.
//
// Metadata is copied by assignment. Callers must treat metadata containing
// pointers, maps, slices, or other reference types as immutable while stored
// in a Table and after it is returned by a lookup.
type Route[M any] struct {
	ID                  uint64           `json:"id"`
	RouterID            netip.Addr       `json:"routerId"`
	NextHop             netip.Addr       `json:"nextHop"`
	PeerAS              uint32           `json:"peerAs"`
	OriginAS            uint32           `json:"originAs"`
	ASPath              []uint32         `json:"asPath"`
	Communities         []uint32         `json:"communities"`
	ExtendedCommunities []uint64         `json:"extendedCommunities"`
	LargeCommunities    []LargeCommunity `json:"largeCommunities"`
	LocalPreference     uint32           `json:"localPreference"`
	MED                 uint32           `json:"med"`
	Origin              Origin           `json:"origin"`
	Metadata            M                `json:"metadata"`
}

// Prefix associates a network prefix with all known routes to it.
type Prefix[M any] struct {
	Prefix netip.Prefix `json:"prefix"`
	Routes []Route[M]   `json:"routes"`
}

func clonePrefix[M any](prefix Prefix[M]) Prefix[M] {
	cloned := Prefix[M]{
		Prefix: prefix.Prefix,
		Routes: make([]Route[M], len(prefix.Routes)),
	}
	for i := range prefix.Routes {
		cloned.Routes[i] = cloneRoute(prefix.Routes[i])
	}
	return cloned
}

func cloneRoute[M any](route Route[M]) Route[M] {
	route.RouterID = normalizeOptionalAddr(route.RouterID)
	route.NextHop = normalizeOptionalAddr(route.NextHop)
	route.ASPath = append([]uint32(nil), route.ASPath...)
	route.Communities = append([]uint32(nil), route.Communities...)
	route.ExtendedCommunities = append([]uint64(nil), route.ExtendedCommunities...)
	route.LargeCommunities = append([]LargeCommunity(nil), route.LargeCommunities...)
	return route
}
