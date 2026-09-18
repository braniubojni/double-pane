package ocr

// Item is one OCR'd image path with its token set and raw text for snippet.
type Item struct {
	Path   string
	Tokens []string
	Text   string
}

// Group is a union-find component of similar OCR texts (len >= 2).
type Group struct {
	Items      []Item
	Similarity int    // min pairwise Jaccard % among connecting edges
	Snippet    string // first 80 chars of normalized text
}

// Cluster unions items whose text similarity meets minPct (Jaccard * 100).
// ponytail: O(n²) pairs; fine for screenshot folders.
func Cluster(items []Item, minPct int) []Group {
	n := len(items)
	if n < 2 {
		return nil
	}
	if minPct < 70 {
		minPct = 70
	}
	if minPct > 100 {
		minPct = 100
	}
	thr := float64(minPct) / 100
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
			if !Similar(items[i].Tokens, items[j].Tokens, thr) {
				continue
			}
			sim := int(jaccard(setOf(items[i].Tokens), setOf(items[j].Tokens)) * 100)
			if sim < minPct {
				// containment match without high Jaccard — still record as minPct
				sim = minPct
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
		g.Snippet = Snippet(g.Items[0].Text)
		out = append(out, g)
	}
	return out
}
