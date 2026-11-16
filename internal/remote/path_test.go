package remote

import "testing"

func TestParseSpec(t *testing.T) {
	cases := []struct {
		in      string
		user    string
		host    string
		port    int
		wantErr bool
	}{
		{"ssh username@ip", "username", "ip", 22, false},
		{"user@192.168.1.1", "user", "192.168.1.1", 22, false},
		{"deploy@prod.example.com:2222", "deploy", "prod.example.com", 2222, false},
		{"ssh://alice@box:22/", "alice", "box", 22, false},
		{"", "", "", 0, true},
		{"not-a-spec", "", "", 0, true},
	}
	for _, c := range cases {
		s, err := ParseSpec(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("%q: expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if s.User != c.user || s.Host != c.host || s.Port != c.port {
			t.Errorf("%q: got %+v", c.in, s)
		}
	}
}

func TestParseLocationAndParent(t *testing.T) {
	loc, err := ParseLocation("ssh://bob@server:2222/home/bob/docs")
	if err != nil {
		t.Fatal(err)
	}
	if loc.User != "bob" || loc.Host != "server" || loc.Port != 2222 || loc.RemotePath != "/home/bob/docs" {
		t.Fatalf("got %+v", loc)
	}
	p := ParentRemote(loc)
	if p.RemotePath != "/home/bob" {
		t.Fatalf("parent: %s", p.RemotePath)
	}
	p2 := ParentRemote(ParentRemote(p))
	if p2.RemotePath != "/" {
		t.Fatalf("root parent: %s", p2.RemotePath)
	}
	if !IsRemote("ssh://a@b/") || IsRemote("/tmp") {
		t.Fatal("IsRemote")
	}
}

func TestParseSMBSpecAndLocation(t *testing.T) {
	t.Parallel()
	s, err := ParseSpec("smb://alice@nas:445/")
	if err != nil {
		t.Fatal(err)
	}
	if !s.IsSMB() || s.User != "alice" || s.Host != "nas" || s.Port != 445 {
		t.Fatalf("spec: %+v", s)
	}
	if s.SessionKey() != "smb:alice@nas:445" {
		t.Fatalf("key: %s", s.SessionKey())
	}

	s2, err := ParseSpec("smb guest@box")
	if err != nil {
		t.Fatal(err)
	}
	if s2.User != "guest" || s2.Host != "box" || s2.Port != 445 {
		t.Fatalf("smb user@host: %+v", s2)
	}

	s3, err := ParseSpec(`smb CORP\bob@fileserver`)
	if err != nil {
		t.Fatal(err)
	}
	if s3.User != "bob" || s3.Domain != "CORP" || s3.Host != "fileserver" {
		t.Fatalf("domain user: %+v", s3)
	}

	loc, err := ParseLocation("smb://alice@nas:445/Media/Photos")
	if err != nil {
		t.Fatal(err)
	}
	if loc.ShareName() != "Media" || loc.PathOnShare() != "Photos" {
		t.Fatalf("share/rel: %q %q", loc.ShareName(), loc.PathOnShare())
	}
	p := ParentRemote(loc)
	if p.RemotePath != "/Media" || p.ShareName() != "Media" || p.PathOnShare() != "." {
		t.Fatalf("parent share: %+v rel=%q", p, p.PathOnShare())
	}
	p2 := ParentRemote(p)
	if p2.RemotePath != "/" || p2.ShareName() != "" {
		t.Fatalf("share list parent: %+v", p2)
	}

	guest, err := ParseLocation("smb://nas/Share")
	if err != nil {
		t.Fatal(err)
	}
	if guest.User != "" || guest.Host != "nas" || guest.ShareName() != "Share" {
		t.Fatalf("guest: %+v", guest)
	}
	if !IsSMB("smb://nas/") || IsSSH("smb://nas/") || !IsRemote("smb://nas/") {
		t.Fatal("scheme helpers")
	}

	guestKey, err := ParseSpec("smb:@nas:445")
	if err != nil {
		t.Fatal(err)
	}
	if guestKey.User != "" || guestKey.Host != "nas" || guestKey.Port != 445 {
		t.Fatalf("guest session key: %+v", guestKey)
	}

	name, domain := splitDomainUser(`CORP\alice`)
	if name != "alice" || domain != "CORP" {
		t.Fatalf("DOMAIN\\user: %q %q", name, domain)
	}
	name, domain = splitDomainUser("alice@corp.local")
	if name != "alice" || domain != "corp.local" {
		t.Fatalf("UPN: %q %q", name, domain)
	}
}

func TestRemoteAbsAndChildPath(t *testing.T) {
	t.Parallel()
	if got := remoteAbsPath(""); got != "/" {
		t.Fatalf("empty: %q", got)
	}
	if got := remoteAbsPath("C:/Users"); got != "/C:/Users" {
		t.Fatalf("drive: %q", got)
	}
	if got := remoteAbsPath("/C:/Users"); got != "/C:/Users" {
		t.Fatalf("already abs: %q", got)
	}
	if got := remoteChildPath("/C:/Users", "Desktop"); got != "/C:/Users/Desktop" {
		t.Fatalf("child: %q", got)
	}
	if got := remoteChildPath("C:/Users", "Desktop"); got != "/C:/Users/Desktop" {
		t.Fatalf("child no slash: %q", got)
	}
}

func TestSMBJoinPathIPv6(t *testing.T) {
	t.Parallel()
	s, err := ParseSpec("smb://alice@[::1]:445/")
	if err != nil {
		t.Fatal(err)
	}
	if s.Host != "::1" || s.Port != 445 {
		t.Fatalf("spec: %+v", s)
	}
	root := s.RootPath()
	if root != "smb://alice@[::1]:445/" {
		t.Fatalf("root: %q", root)
	}
	loc, err := ParseLocation(root + "Share/a")
	if err != nil {
		t.Fatal(err)
	}
	if loc.Host != "::1" || loc.ShareName() != "Share" {
		t.Fatalf("roundtrip: %+v", loc)
	}
}

func TestParseFTPSpecAndLocation(t *testing.T) {
	t.Parallel()
	loc, err := ParseLocation("ftp://a@h:21/x")
	if err != nil {
		t.Fatal(err)
	}
	if loc.User != "a" || loc.Host != "h" || loc.Port != 21 || loc.RemotePath != "/x" {
		t.Fatalf("%+v", loc)
	}
	if !IsRemote(loc.JoinPath(loc.RemotePath)) || SchemeOf("ftp://a@h:21/x") != "ftp" || !IsFTP("ftp://a@h:21/x") {
		t.Fatal("scheme helpers")
	}
	if loc.SessionKey() != "ftp:a@h:21" {
		t.Fatalf("key %s", loc.SessionKey())
	}
	anon, err := ParseSpec("ftp anonymous@files.example")
	if err != nil {
		t.Fatal(err)
	}
	if !anon.IsFTP() || anon.User != "anonymous" || anon.Host != "files.example" || anon.Port != 21 {
		t.Fatalf("%+v", anon)
	}
	if _, err := ParseLocation("ftp://host/no-user"); err == nil {
		t.Fatal("expected missing user")
	}
	def, err := ParseLocation("ftp://anonymous@host/pub")
	if err != nil || def.Port != 21 || def.RemotePath != "/pub" {
		t.Fatalf("%+v %v", def, err)
	}
	v6, err := ParseSpec("ftp://alice@[::1]:2121/")
	if err != nil {
		t.Fatal(err)
	}
	if v6.Host != "::1" || v6.Port != 2121 || v6.RootPath() != "ftp://alice@[::1]:2121/" {
		t.Fatalf("%+v root %s", v6, v6.RootPath())
	}
}

func TestParseMEGASpecAndLocation(t *testing.T) {
	t.Parallel()
	s, err := ParseSpec("mega://alice@gmail.com/")
	if err != nil {
		t.Fatal(err)
	}
	if !s.IsMEGA() || s.User != "alice" || s.Host != "gmail.com" || s.Port != 0 {
		t.Fatalf("spec: %+v", s)
	}
	if s.SessionKey() != "mega:alice@gmail.com" {
		t.Fatalf("key: %s", s.SessionKey())
	}
	if s.MEGAEmail() != "alice@gmail.com" {
		t.Fatalf("email: %s", s.MEGAEmail())
	}
	if s.RootPath() != "mega://alice@gmail.com/" {
		t.Fatalf("root: %q", s.RootPath())
	}
	if s.JoinPath("/Photos/x.jpg") != "mega://alice@gmail.com/Photos/x.jpg" {
		t.Fatalf("join: %q", s.JoinPath("/Photos/x.jpg"))
	}

	s2, err := ParseSpec("mega alice@gmail.com")
	if err != nil {
		t.Fatal(err)
	}
	if s2.SessionKey() != "mega:alice@gmail.com" {
		t.Fatalf("mega email: %+v", s2)
	}

	s3, err := ParseSpec("mega:alice@gmail.com")
	if err != nil {
		t.Fatal(err)
	}
	if s3.SessionKey() != "mega:alice@gmail.com" {
		t.Fatalf("session key form: %+v", s3)
	}

	loc, err := ParseLocation("mega://alice@gmail.com/Photos/x.jpg?h=AbCd")
	if err != nil {
		t.Fatal(err)
	}
	if loc.User != "alice" || loc.Host != "gmail.com" || loc.RemotePath != "/Photos/x.jpg" {
		t.Fatalf("loc: %+v", loc)
	}
	if megaHandle("mega://alice@gmail.com/Photos/x.jpg?h=AbCd") != "AbCd" {
		t.Fatalf("handle: %q", megaHandle("mega://alice@gmail.com/Photos/x.jpg?h=AbCd"))
	}
	p := ParentRemote(loc)
	if p.RemotePath != "/Photos" {
		t.Fatalf("parent: %s", p.RemotePath)
	}
	if !IsMEGA("mega://alice@gmail.com/") || IsSSH("mega://alice@gmail.com/") || !IsRemote("mega://alice@gmail.com/") {
		t.Fatal("scheme helpers")
	}
	if SchemeOf("mega://alice@gmail.com/x") != "mega" {
		t.Fatal("SchemeOf")
	}
	if _, err := ParseSpec("mega not-an-email"); err == nil {
		t.Fatal("expected invalid email")
	}
}
