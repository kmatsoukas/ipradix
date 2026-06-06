package ipradix

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"testing"
)

func BenchmarkFindIPv4(b *testing.B) {
	var table Table[struct{}]
	const prefixCount = 100_000
	for i := 0; i < prefixCount; i++ {
		var bytes [4]byte
		binary.BigEndian.PutUint32(bytes[:], uint32(i)<<8)
		prefix := netip.PrefixFrom(netip.AddrFrom4(bytes), 24)
		if err := table.Insert(Prefix[struct{}]{
			Prefix: prefix,
			Routes: []Route[struct{}]{{ID: fmt.Sprintf("%d", i)}},
		}); err != nil {
			b.Fatal(err)
		}
	}

	addresses := make([]netip.Addr, 1_024)
	for i := range addresses {
		var bytes [4]byte
		binary.BigEndian.PutUint32(bytes[:], uint32(i*97)<<8|1)
		addresses[i] = netip.AddrFrom4(bytes)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		table.Find(addresses[i%len(addresses)])
	}
}

func BenchmarkFindIPv6(b *testing.B) {
	var table Table[struct{}]
	const prefixCount = 50_000
	for i := 0; i < prefixCount; i++ {
		var bytes [16]byte
		bytes[0], bytes[1] = 0x20, 0x01
		binary.BigEndian.PutUint32(bytes[4:8], uint32(i))
		prefix := netip.PrefixFrom(netip.AddrFrom16(bytes), 64)
		if err := table.Insert(Prefix[struct{}]{
			Prefix: prefix,
			Routes: []Route[struct{}]{{ID: fmt.Sprintf("%d", i)}},
		}); err != nil {
			b.Fatal(err)
		}
	}

	addresses := make([]netip.Addr, 1_024)
	for i := range addresses {
		var bytes [16]byte
		bytes[0], bytes[1] = 0x20, 0x01
		binary.BigEndian.PutUint32(bytes[4:8], uint32(i*47))
		bytes[15] = 1
		addresses[i] = netip.AddrFrom16(bytes)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		table.Find(addresses[i%len(addresses)])
	}
}
