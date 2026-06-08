package ipradix

import (
	"net/netip"
	"testing"
)

func TestFindExactNodeBranches(t *testing.T) {
	childPrefix := netip.MustParsePrefix("10.1.0.0/16")
	root := &node[struct{}]{
		prefix: netip.MustParsePrefix("10.0.0.0/8"),
		child: [2]*node[struct{}]{
			{prefix: childPrefix},
			nil,
		},
	}

	if got := findExactNode(root, childPrefix); got != root.child[0] {
		t.Fatalf("findExactNode() = %p; want child %p", got, root.child[0])
	}
	if got := findExactNode(root, netip.MustParsePrefix("10.1.1.0/24")); got != nil {
		t.Fatalf("findExactNode() = %v; want nil after missing child", got)
	}
	if got := findExactNode(root, netip.MustParsePrefix("10.0.0.0/7")); got != nil {
		t.Fatalf("findExactNode() = %v; want nil for shorter prefix", got)
	}
	if got := findExactNode(root, netip.MustParsePrefix("192.0.2.0/24")); got != nil {
		t.Fatalf("findExactNode() = %v; want nil for a different branch", got)
	}
}

func TestDeleteNodeBranches(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")

	if got, deleted := deleteNode[struct{}](nil, prefix); got != nil || deleted {
		t.Fatalf("deleteNode(nil) = (%v, %v); want (nil, false)", got, deleted)
	}

	branch := &node[struct{}]{prefix: prefix}
	if got, deleted := deleteNode(branch, prefix); got != branch || deleted {
		t.Fatalf("deleteNode(branch) = (%v, %v); want unchanged branch, false", got, deleted)
	}

	value := Prefix[struct{}]{Prefix: prefix, Routes: []Route[struct{}]{{ID: 1}}}
	leaf := &node[struct{}]{prefix: prefix, value: &value}
	if got, deleted := deleteNode(leaf, netip.MustParsePrefix("10.0.0.0/7")); got != leaf || deleted {
		t.Fatalf("deleteNode(shorter) = (%v, %v); want unchanged leaf, false", got, deleted)
	}
	if got, deleted := deleteNode(leaf, netip.MustParsePrefix("192.0.2.0/24")); got != leaf || deleted {
		t.Fatalf("deleteNode(other branch) = (%v, %v); want unchanged leaf, false", got, deleted)
	}
}

func TestCompactNodeBranches(t *testing.T) {
	if got := compactNode[struct{}](nil); got != nil {
		t.Fatalf("compactNode(nil) = %v; want nil", got)
	}

	value := Prefix[struct{}]{}
	valued := &node[struct{}]{value: &value}
	if got := compactNode(valued); got != valued {
		t.Fatal("compactNode() changed a valued node")
	}

	left := &node[struct{}]{prefix: netip.MustParsePrefix("0.0.0.0/1")}
	right := &node[struct{}]{prefix: netip.MustParsePrefix("128.0.0.0/1")}

	onlyLeft := &node[struct{}]{child: [2]*node[struct{}]{left, nil}}
	if got := compactNode(onlyLeft); got != left {
		t.Fatalf("compactNode(left) = %p; want %p", got, left)
	}

	onlyRight := &node[struct{}]{child: [2]*node[struct{}]{nil, right}}
	if got := compactNode(onlyRight); got != right {
		t.Fatalf("compactNode(right) = %p; want %p", got, right)
	}

	both := &node[struct{}]{child: [2]*node[struct{}]{left, right}}
	if got := compactNode(both); got != both {
		t.Fatal("compactNode() changed a two-child branch")
	}
}
