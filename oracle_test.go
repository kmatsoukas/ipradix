package ipradix

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net/netip"
	"testing"
)

func TestRandomizedFindAgainstLinearOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	testRandomFamily(t, rng, true, 2_000, 5_000)
	testRandomFamily(t, rng, false, 1_000, 3_000)
}

func testRandomFamily(t *testing.T, rng *rand.Rand, ipv4 bool, prefixCount, lookupCount int) {
	t.Helper()
	var table Table[int]
	entries := make(map[netip.Prefix]Prefix[int], prefixCount)

	for i := 0; i < prefixCount; i++ {
		addr := randomAddr(rng, ipv4)
		bits := rng.Intn(addr.BitLen() + 1)
		prefix := netip.PrefixFrom(addr, bits).Masked()
		entry := Prefix[int]{
			Prefix: prefix,
			Routes: []Route[int]{{ID: fmt.Sprintf("route-%d", i), Metadata: i}},
		}
		if err := table.Insert(entry); err != nil {
			t.Fatalf("Insert(%s) failed: %v", prefix, err)
		}
		entries[prefix] = entry
	}

	for i := 0; i < lookupCount; i++ {
		addr := randomAddr(rng, ipv4)
		got, gotOK := table.Find(addr)
		want, wantOK := linearFind(entries, addr)
		if gotOK != wantOK {
			t.Fatalf("Find(%s) match = %v; want %v", addr, gotOK, wantOK)
		}
		if gotOK && got.Prefix != want.Prefix {
			t.Fatalf("Find(%s) = %s; want %s", addr, got.Prefix, want.Prefix)
		}

		all := table.FindAll(addr)
		for j := 1; j < len(all); j++ {
			if all[j-1].Prefix.Bits() <= all[j].Prefix.Bits() {
				t.Fatalf("FindAll(%s) is not most-specific first: %v", addr, prefixStrings(all))
			}
		}
		for _, match := range all {
			if !match.Prefix.Contains(addr) {
				t.Fatalf("FindAll(%s) returned non-matching prefix %s", addr, match.Prefix)
			}
		}
	}
}

func linearFind[M any](entries map[netip.Prefix]Prefix[M], addr netip.Addr) (Prefix[M], bool) {
	var best Prefix[M]
	found := false
	for prefix, entry := range entries {
		if prefix.Addr().Is4() != addr.Is4() || !prefix.Contains(addr) {
			continue
		}
		if !found || prefix.Bits() > best.Prefix.Bits() {
			best = entry
			found = true
		}
	}
	return best, found
}

func randomAddr(rng *rand.Rand, ipv4 bool) netip.Addr {
	if ipv4 {
		var bytes [4]byte
		binary.BigEndian.PutUint32(bytes[:], rng.Uint32())
		return netip.AddrFrom4(bytes)
	}
	var bytes [16]byte
	if _, err := rng.Read(bytes[:]); err != nil {
		panic(err)
	}
	return netip.AddrFrom16(bytes)
}

func FuzzFindAgainstLinearOracle(f *testing.F) {
	f.Add([]byte{0, 10, 1, 2, 3, 8, 10, 0, 0, 1, 16, 10, 1, 0, 1})
	f.Add([]byte{1, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 2 {
			return
		}
		ipv4 := data[0]&1 == 0
		width := 4
		if !ipv4 {
			width = 16
		}
		if len(data) < 1+width {
			return
		}

		target := addrFromBytes(data[1:1+width], ipv4)
		data = data[1+width:]
		stride := width + 1
		if len(data)/stride > 256 {
			data = data[:256*stride]
		}

		var table Table[struct{}]
		entries := make(map[netip.Prefix]Prefix[struct{}])
		for i := 0; len(data) >= stride; i++ {
			addr := addrFromBytes(data[:width], ipv4)
			bits := int(data[width]) % (addr.BitLen() + 1)
			prefix := netip.PrefixFrom(addr, bits).Masked()
			entry := Prefix[struct{}]{
				Prefix: prefix,
				Routes: []Route[struct{}]{{ID: fmt.Sprintf("%d", i)}},
			}
			if err := table.Insert(entry); err != nil {
				t.Fatalf("Insert(%s) failed: %v", prefix, err)
			}
			entries[prefix] = entry
			data = data[stride:]
		}

		got, gotOK := table.Find(target)
		want, wantOK := linearFind(entries, target)
		if gotOK != wantOK || gotOK && got.Prefix != want.Prefix {
			t.Fatalf("Find(%s) = (%s, %v); want (%s, %v)", target, got.Prefix, gotOK, want.Prefix, wantOK)
		}
	})
}

func addrFromBytes(data []byte, ipv4 bool) netip.Addr {
	if ipv4 {
		return netip.AddrFrom4([4]byte(data[:4]))
	}
	return netip.AddrFrom16([16]byte(data[:16]))
}
