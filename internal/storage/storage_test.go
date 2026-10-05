package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestBookmarks(t *testing.T) {
	db, err := OpenPath(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	bm, err := db.AddBookmark("Tmp", "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if bm.ID == 0 {
		t.Fatal("expected bookmark id")
	}
	list, err := db.ListBookmarks()
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	if err := db.RemoveBookmark(bm.ID); err != nil {
		t.Fatal(err)
	}
	list, err = db.ListBookmarks()
	if err != nil || len(list) != 0 {
		t.Fatalf("expected empty list, got %+v", list)
	}
}

func TestBookmarksReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddBookmark("A", "/a"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddBookmark("B", "/b"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db2, err := OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db2.Close() })
	list, err := db2.ListBookmarks()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Path != "/a" || list[1].Path != "/b" {
		t.Fatalf("got %+v", list)
	}
}

func TestImportBookmarks(t *testing.T) {
	legacyDir := t.TempDir()
	legacy, err := OpenPath(filepath.Join(legacyDir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.AddBookmark("A", "/a"); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.AddBookmark("B", "/b"); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	canon, err := OpenPath(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = canon.Close() })
	legacyPath := filepath.Join(legacyDir, "app.db")
	if err := canon.importBookmarksFrom(legacyPath); err != nil {
		t.Fatal(err)
	}
	if err := canon.importBookmarksFrom(legacyPath); err != nil {
		t.Fatal(err)
	}
	list, err := canon.ListBookmarks()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Path != "/a" || list[1].Path != "/b" {
		t.Fatalf("got %+v", list)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy file: %v", err)
	}
}

func TestImportBookmarksSkipsExistingPath(t *testing.T) {
	legacyDir := t.TempDir()
	legacy, err := OpenPath(filepath.Join(legacyDir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.AddBookmark("A", "/a"); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	canon, err := OpenPath(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = canon.Close() })
	if _, err := canon.AddBookmark("Keep", "/a"); err != nil {
		t.Fatal(err)
	}
	if err := canon.importBookmarksFrom(filepath.Join(legacyDir, "app.db")); err != nil {
		t.Fatal(err)
	}
	list, err := canon.ListBookmarks()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Path != "/a" || list[0].Name != "Keep" {
		t.Fatalf("got %+v", list)
	}
}

func TestCloseNilAndTwice(t *testing.T) {
	var db *DB
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	opened, err := OpenPath(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestEncryptedKV(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenPath(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := os.Stat(filepath.Join(dir, "app.key")); err != nil {
		t.Fatalf("expected app.key: %v", err)
	}

	plain := []byte(`{"theme":"dark","showHidden":true}`)
	if err := db.SetKV("settings", plain); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetKV("settings")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plain) {
		t.Fatalf("got %q want %q", got, plain)
	}

	// Re-open with same key file
	_ = db.Close()
	db2, err := OpenPath(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db2.Close() })
	got2, err := db2.GetKV("settings")
	if err != nil {
		t.Fatal(err)
	}
	if string(got2) != string(plain) {
		t.Fatalf("reopen got %q", got2)
	}
}

func TestRemoteRecent_upsertAndOrder(t *testing.T) {
	db, err := OpenPath(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	const sk = "user@host:22"

	// Insert two paths
	if err := db.AddRemoteRecent(sk, "ssh://user@host:22/home/user", "/home/user"); err != nil {
		t.Fatal(err)
	}
	if err := db.AddRemoteRecent(sk, "ssh://user@host:22/projects", "/projects"); err != nil {
		t.Fatal(err)
	}

	list, err := db.GetRemoteRecent(sk)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
	// Newest first
	if list[0].Label != "/projects" {
		t.Errorf("newest first: want /projects, got %s", list[0].Label)
	}
}

func TestRemoteRecent_upsertUpdatesTimestamp(t *testing.T) {
	db, err := OpenPath(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	const sk = "user@host:22"
	const vpath = "ssh://user@host:22/home/user"

	if err := db.AddRemoteRecent(sk, vpath, "/home/user"); err != nil {
		t.Fatal(err)
	}
	// Insert a different path so it becomes the newest
	if err := db.AddRemoteRecent(sk, "ssh://user@host:22/tmp", "/tmp"); err != nil {
		t.Fatal(err)
	}
	// Re-visit the first path — it should become newest again
	if err := db.AddRemoteRecent(sk, vpath, "/home/user"); err != nil {
		t.Fatal(err)
	}

	list, err := db.GetRemoteRecent(sk)
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Label != "/home/user" {
		t.Errorf("re-visited path should be newest, got %s", list[0].Label)
	}
}

func TestRemoteRecent_capAt10(t *testing.T) {
	db, err := OpenPath(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	const sk = "user@host:22"

	for i := range 15 {
		vpath := fmt.Sprintf("ssh://user@host:22/dir%02d", i)
		label := fmt.Sprintf("/dir%02d", i)
		if err := db.AddRemoteRecent(sk, vpath, label); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	list, err := db.GetRemoteRecent(sk)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 10 {
		t.Errorf("expected cap of 10, got %d", len(list))
	}
	// Most recent should be dir14
	if list[0].Label != "/dir14" {
		t.Errorf("want newest /dir14, got %s", list[0].Label)
	}
}

func TestRemoteRecent_isolatedBySessionKey(t *testing.T) {
	db, err := OpenPath(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.AddRemoteRecent("a@host:22", "ssh://a@host:22/home", "/home"); err != nil {
		t.Fatal(err)
	}
	if err := db.AddRemoteRecent("b@host:22", "ssh://b@host:22/var", "/var"); err != nil {
		t.Fatal(err)
	}

	listA, _ := db.GetRemoteRecent("a@host:22")
	listB, _ := db.GetRemoteRecent("b@host:22")

	if len(listA) != 1 || listA[0].Label != "/home" {
		t.Errorf("session A isolation failed: %+v", listA)
	}
	if len(listB) != 1 || listB[0].Label != "/var" {
		t.Errorf("session B isolation failed: %+v", listB)
	}
}

func TestSearchHistoryCap(t *testing.T) {
	db, err := OpenPath(filepath.Join(t.TempDir(), "hist.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.AddSearchHistory("query", "  "); err != nil {
		t.Fatal(err)
	}
	list, err := db.ListSearchHistory("query", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("blank should be ignored: %#v", list)
	}

	for i := 0; i < 10; i++ {
		if err := db.AddSearchHistory("query", fmt.Sprintf("q-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	// re-use old entry — should move to top
	if err := db.AddSearchHistory("query", "q-0"); err != nil {
		t.Fatal(err)
	}
	list, err = db.ListSearchHistory("query", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 10 || list[0] != "q-0" {
		t.Fatalf("got %#v", list)
	}

	// cap at maxSearchHistoryPerField
	for i := 0; i < maxSearchHistoryPerField+20; i++ {
		if err := db.AddSearchHistory("include", fmt.Sprintf("inc-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	list, err = db.ListSearchHistory("include", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != maxSearchHistoryPerField {
		t.Fatalf("want cap %d got %d", maxSearchHistoryPerField, len(list))
	}
}
