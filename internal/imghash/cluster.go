package imghash

// Item is one hashed image path.
type Item struct {
	Path string
	Hash uint64
}

// Group is a union-find component of similar images (len >= 2).
type Group struct {
	Items      []Item
	Similarity int // min pairwise similarity among connecting edges
}

// Cluster unions items whose dHash similarity is >= minPct.
// ponytail: O(n²) pairs; BK-tree if visual pass is slow on huge folders.
func Cluster(items []Item, minPct int) []Group {
	n := len(items)
	if n < 2 {
		return nil
	}
	minPct = ClampPct(minPct)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	union := func(a, b int) {
		pa, pb := find(a), find(b)
		if pa != pb {
			parent[pa] = pb
		}
	}
	edgeMin := map[[2]int]int{}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			sim := Similarity(items[i].Hash, items[j].Hash)
			if sim < minPct {
				continue
			}
			union(i, j)
			a, b := i, j
			if a > b {
				a, b = b, a
			}
			edgeMin[[2]int{a, b}] = sim
		}
	}
	members := map[int][]int{}
	for i := 0; i < n; i++ {
		members[find(i)] = append(members[find(i)], i)
	}
	var out []Group
	for _, idxs := range members {
		if len(idxs) < 2 {
			continue
		}
		g := Group{Similarity: 100}
		for _, i := range idxs {
			g.Items = append(g.Items, items[i])
		}
		for a := 0; a < len(idxs); a++ {
			for b := a + 1; b < len(idxs); b++ {
				i, j := idxs[a], idxs[b]
				if i > j {
					i, j = j, i
				}
				if sim, ok := edgeMin[[2]int{i, j}]; ok && sim < g.Similarity {
					g.Similarity = sim
				}
			}
		}
		out = append(out, g)
	}
	return out
}
