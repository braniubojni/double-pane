package imghash

import "testing"

func TestClusterSimilarPairAndUnrelated(t *testing.T) {
	src := flagImage(128, 128, 0)
	same := DHash(scale(src, 64, 64))
	orig := DHash(src)
	other := DHash(flagImage(128, 128, 1))
	got := Cluster([]Item{
		{Path: "a.jpg", Hash: orig},
		{Path: "b.jpg", Hash: same},
		{Path: "c.jpg", Hash: other},
	}, 90)
	if len(got) != 1 {
		t.Fatalf("want 1 group, got %+v", got)
	}
	if len(got[0].Items) != 2 {
		t.Fatalf("want resized pair only, got %+v", got[0].Items)
	}
	if got[0].Similarity < 90 {
		t.Fatalf("similarity %d", got[0].Similarity)
	}
}

func TestClusterTransitive(t *testing.T) {
	// A~~B and B~~C at 90% must become one group even if A~~C is weaker.
	h := uint64(0)
	got := Cluster([]Item{
		{Path: "a", Hash: h},
		{Path: "b", Hash: h ^ 1},       // hamming 1 → 98%
		{Path: "c", Hash: h ^ (1 | 2)}, // vs b hamming 1; vs a hamming 2 → 96%
	}, 90)
	if len(got) != 1 || len(got[0].Items) != 3 {
		t.Fatalf("want one group of 3, got %+v", got)
	}
}

func TestClusterBelowThresholdEmpty(t *testing.T) {
	got := Cluster([]Item{
		{Path: "a", Hash: 0},
		{Path: "b", Hash: ^uint64(0)},
	}, 90)
	if len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
