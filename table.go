// Package ipradix provides a concurrent IPv4 and IPv6 routing table backed by
// compressed Patricia radix trees.
package ipradix

import (
	"errors"
	"fmt"
	"net/netip"
	"sync"
)

var (
	ErrInvalidPrefix    = errors.New("invalid prefix")
	ErrEmptyRoutes      = errors.New("prefix has no routes")
	ErrEmptyRouteID     = errors.New("route ID is empty")
	ErrDuplicateRouteID = errors.New("duplicate route ID")
)

// Table is a concurrent IPv4 and IPv6 routing table. Its zero value is ready
// for use. A Table must not be copied after first use.
type Table[M any] struct {
	mu sync.RWMutex
	v4 *node[M]
	v6 *node[M]
}

// Insert atomically inserts or replaces a prefix and all of its routes.
func (t *Table[M]) Insert(prefix Prefix[M]) error {
	normalized, err := normalizePrefix(prefix.Prefix)
	if err != nil {
		return err
	}
	if err := validateRoutes(prefix.Routes); err != nil {
		return err
	}

	prefix.Prefix = normalized
	prefix = clonePrefix(prefix)

	t.mu.Lock()
	defer t.mu.Unlock()

	root := t.root(normalized.Addr().Is4())
	*root = insertNode(*root, normalized, prefix)
	return nil
}

// UpsertRoute inserts a route or replaces the route with the same ID at an
// exact prefix. New routes are appended, while replacements retain their
// existing position.
func (t *Table[M]) UpsertRoute(prefix netip.Prefix, route Route[M]) error {
	normalized, err := normalizePrefix(prefix)
	if err != nil {
		return err
	}
	if route.ID == "" {
		return ErrEmptyRouteID
	}
	route = cloneRoute(route)

	t.mu.Lock()
	defer t.mu.Unlock()

	root := t.root(normalized.Addr().Is4())
	current := findExactNode(*root, normalized)
	if current == nil || current.value == nil {
		value := Prefix[M]{Prefix: normalized, Routes: []Route[M]{route}}
		*root = insertNode(*root, normalized, value)
		return nil
	}

	for i := range current.value.Routes {
		if current.value.Routes[i].ID == route.ID {
			current.value.Routes[i] = route
			return nil
		}
	}
	current.value.Routes = append(current.value.Routes, route)
	return nil
}

// Delete removes an exact prefix and all routes associated with it.
func (t *Table[M]) Delete(prefix netip.Prefix) bool {
	normalized, err := normalizePrefix(prefix)
	if err != nil {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	root := t.root(normalized.Addr().Is4())
	var deleted bool
	*root, deleted = deleteNode(*root, normalized)
	return deleted
}

// DeleteRoute removes a route by ID from an exact prefix. Removing the final
// route also removes the prefix.
func (t *Table[M]) DeleteRoute(prefix netip.Prefix, routeID string) bool {
	if routeID == "" {
		return false
	}
	normalized, err := normalizePrefix(prefix)
	if err != nil {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	root := t.root(normalized.Addr().Is4())
	current := findExactNode(*root, normalized)
	if current == nil || current.value == nil {
		return false
	}

	for i := range current.value.Routes {
		if current.value.Routes[i].ID != routeID {
			continue
		}
		if len(current.value.Routes) == 1 {
			*root, _ = deleteNode(*root, normalized)
			return true
		}
		copy(current.value.Routes[i:], current.value.Routes[i+1:])
		var zero Route[M]
		current.value.Routes[len(current.value.Routes)-1] = zero
		current.value.Routes = current.value.Routes[:len(current.value.Routes)-1]
		return true
	}
	return false
}

// Find returns the longest-prefix match for addr.
func (t *Table[M]) Find(addr netip.Addr) (Prefix[M], bool) {
	addr, ok := normalizeAddr(addr)
	if !ok {
		return Prefix[M]{}, false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	current := *t.root(addr.Is4())
	var match *Prefix[M]
	for current != nil && current.prefix.Contains(addr) {
		if current.value != nil {
			match = current.value
		}
		if current.prefix.Bits() == addr.BitLen() {
			break
		}
		current = current.child[bitAt(addr, current.prefix.Bits())]
	}
	if match == nil {
		return Prefix[M]{}, false
	}
	return clonePrefix(*match), true
}

// FindAll returns every prefix containing addr, ordered from most specific to
// least specific.
func (t *Table[M]) FindAll(addr netip.Addr) []Prefix[M] {
	addr, ok := normalizeAddr(addr)
	if !ok {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	current := *t.root(addr.Is4())
	var matches []Prefix[M]
	for current != nil && current.prefix.Contains(addr) {
		if current.value != nil {
			matches = append(matches, clonePrefix(*current.value))
		}
		if current.prefix.Bits() == addr.BitLen() {
			break
		}
		current = current.child[bitAt(addr, current.prefix.Bits())]
	}
	for left, right := 0, len(matches)-1; left < right; left, right = left+1, right-1 {
		matches[left], matches[right] = matches[right], matches[left]
	}
	return matches
}

func (t *Table[M]) root(ipv4 bool) **node[M] {
	if ipv4 {
		return &t.v4
	}
	return &t.v6
}

func validateRoutes[M any](routes []Route[M]) error {
	if len(routes) == 0 {
		return ErrEmptyRoutes
	}
	ids := make(map[string]struct{}, len(routes))
	for i := range routes {
		if routes[i].ID == "" {
			return fmt.Errorf("%w at index %d", ErrEmptyRouteID, i)
		}
		if _, exists := ids[routes[i].ID]; exists {
			return fmt.Errorf("%w %q", ErrDuplicateRouteID, routes[i].ID)
		}
		ids[routes[i].ID] = struct{}{}
	}
	return nil
}

func normalizePrefix(prefix netip.Prefix) (netip.Prefix, error) {
	if !prefix.IsValid() {
		return netip.Prefix{}, ErrInvalidPrefix
	}
	addr := prefix.Addr()
	bits := prefix.Bits()
	if addr.Is4In6() {
		if bits < 96 {
			return netip.Prefix{}, fmt.Errorf("%w: mapped IPv4 prefix length %d", ErrInvalidPrefix, bits)
		}
		return netip.PrefixFrom(addr.Unmap(), bits-96).Masked(), nil
	}
	return netip.PrefixFrom(addr, bits).Masked(), nil
}

func normalizeAddr(addr netip.Addr) (netip.Addr, bool) {
	if !addr.IsValid() {
		return netip.Addr{}, false
	}
	if addr.Zone() != "" {
		addr = addr.WithZone("")
	}
	return addr.Unmap(), true
}

func normalizeOptionalAddr(addr netip.Addr) netip.Addr {
	normalized, ok := normalizeAddr(addr)
	if !ok {
		return netip.Addr{}
	}
	return normalized
}
