package ipradix

import "net/netip"

type node[M any] struct {
	prefix netip.Prefix
	value  *Prefix[M]
	child  [2]*node[M]
}

func insertNode[M any](current *node[M], prefix netip.Prefix, value Prefix[M]) *node[M] {
	if current == nil {
		return &node[M]{prefix: prefix, value: &value}
	}

	common := commonPrefixBits(current.prefix, prefix)
	switch {
	case common == current.prefix.Bits() && common == prefix.Bits():
		current.value = &value
		return current
	case common == current.prefix.Bits():
		direction := bitAt(prefix.Addr(), current.prefix.Bits())
		current.child[direction] = insertNode(current.child[direction], prefix, value)
		return current
	case common == prefix.Bits():
		parent := &node[M]{prefix: prefix, value: &value}
		parent.child[bitAt(current.prefix.Addr(), common)] = current
		return parent
	default:
		branchPrefix := netip.PrefixFrom(prefix.Addr(), common).Masked()
		branch := &node[M]{prefix: branchPrefix}
		branch.child[bitAt(current.prefix.Addr(), common)] = current
		branch.child[bitAt(prefix.Addr(), common)] = &node[M]{
			prefix: prefix,
			value:  &value,
		}
		return branch
	}
}

func findExactNode[M any](current *node[M], prefix netip.Prefix) *node[M] {
	for current != nil {
		if current.prefix == prefix {
			return current
		}
		if current.prefix.Bits() >= prefix.Bits() || !current.prefix.Contains(prefix.Addr()) {
			return nil
		}
		current = current.child[bitAt(prefix.Addr(), current.prefix.Bits())]
	}
	return nil
}

func deleteNode[M any](current *node[M], prefix netip.Prefix) (*node[M], bool) {
	if current == nil {
		return nil, false
	}
	if current.prefix == prefix {
		if current.value == nil {
			return current, false
		}
		current.value = nil
		return compactNode(current), true
	}
	if current.prefix.Bits() >= prefix.Bits() || !current.prefix.Contains(prefix.Addr()) {
		return current, false
	}

	direction := bitAt(prefix.Addr(), current.prefix.Bits())
	var deleted bool
	current.child[direction], deleted = deleteNode(current.child[direction], prefix)
	if !deleted {
		return current, false
	}
	return compactNode(current), true
}

func compactNode[M any](current *node[M]) *node[M] {
	if current == nil || current.value != nil {
		return current
	}
	switch {
	case current.child[0] == nil:
		return current.child[1]
	case current.child[1] == nil:
		return current.child[0]
	default:
		return current
	}
}

func commonPrefixBits(a, b netip.Prefix) int {
	limit := min(a.Bits(), b.Bits())
	aBytes := a.Addr().AsSlice()
	bBytes := b.Addr().AsSlice()
	for bit := 0; bit < limit; bit++ {
		index := bit / 8
		mask := byte(1 << (7 - bit%8))
		if aBytes[index]&mask != bBytes[index]&mask {
			return bit
		}
	}
	return limit
}

func bitAt(addr netip.Addr, bit int) int {
	bytes := addr.AsSlice()
	if bytes[bit/8]&(1<<(7-bit%8)) == 0 {
		return 0
	}
	return 1
}
