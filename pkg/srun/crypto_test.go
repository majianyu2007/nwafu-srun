package srun

import "testing"

func TestHMACMD5Hex_deterministic(t *testing.T) {
	got := HMACMD5Hex("password", "token")
	want := HMACMD5Hex("password", "token")
	if got != want || len(got) != 32 {
		t.Fatalf("unexpected HMACMD5Hex: %q", got)
	}
}

func TestSHA1Hex_deterministic(t *testing.T) {
	got := SHA1Hex("test")
	want := "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"
	if got != want {
		t.Fatalf("SHA1Hex = %q, want %q", got, want)
	}
}

func TestGetMD5_alias(t *testing.T) {
	if GetMD5("p", "t") != HMACMD5Hex("p", "t") {
		t.Fatal("GetMD5 alias mismatch")
	}
}

func TestGetSha1_alias(t *testing.T) {
	if GetSha1("test") != SHA1Hex("test") {
		t.Fatal("GetSha1 alias mismatch")
	}
}

func TestXencodeAndJsBase64(t *testing.T) {
	challenge := "0123456789abcdef"
	msg := `{"username":"u","password":"p","ip":"1.2.3.4","acid":"1","enc_ver":"srun_bx1"}`
	enc := xencode(msg, challenge)
	if enc == "" {
		t.Fatal("xencode returned empty")
	}
	b64 := jsBase64(enc)
	if b64 == "" {
		t.Fatal("jsBase64 returned empty")
	}
	// Same input must produce same output
	if xencode(msg, challenge) != enc {
		t.Fatal("xencode not deterministic")
	}
	if jsBase64(enc) != b64 {
		t.Fatal("jsBase64 not deterministic")
	}
}

func TestGetInfoStringJSON(t *testing.T) {
	c := &Client{
		Username: "user\"test",
		Password: `pass\word`,
		IP:       "10.0.0.1",
		AcID:     "1",
	}
	s, err := c.getInfoString("challenge")
	if err != nil {
		t.Fatal(err)
	}
	if s == "" || s[:7] != "{SRBX1}" {
		t.Fatalf("unexpected info string prefix: %q", s)
	}
}
