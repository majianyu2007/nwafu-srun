package srun

import "testing"

const sampleHomeHTML = `
<html>
<head>
<meta name="csrf-token" content="TOKEN123">
</head>
<body>
<form>
<input type="hidden" name="_csrf-8800" value="TOKEN123">
<a href="/home/delete?id=42&amp;user_mac=aa%3Abb%3Acc%3Add%3Aee%3Aff">kick</a>
<a href="/home/delete?id=99&user_mac=11:22:33:44:55:66">kick2</a>
</form>
</body>
</html>
`

func TestParseHomePage(t *testing.T) {
	csrf, sessions, err := ParseHomePage(sampleHomeHTML)
	if err != nil {
		t.Fatal(err)
	}
	if csrf.FieldName != "_csrf-8800" {
		t.Fatalf("field name = %q", csrf.FieldName)
	}
	if csrf.Token != "TOKEN123" {
		t.Fatalf("token = %q", csrf.Token)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions = %d", len(sessions))
	}
	if sessions[0].ID != "42" || sessions[0].MAC != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("session0 = %+v", sessions[0])
	}
	if sessions[1].ID != "99" || sessions[1].MAC != "11:22:33:44:55:66" {
		t.Fatalf("session1 = %+v", sessions[1])
	}
}

func TestParseHomePage_missingCSRF(t *testing.T) {
	_, _, err := ParseHomePage("<html></html>")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRandomMAC(t *testing.T) {
	m1, err := randomMAC()
	if err != nil {
		t.Fatal(err)
	}
	m2, err := randomMAC()
	if err != nil {
		t.Fatal(err)
	}
	if len(m1) != 17 || len(m2) != 17 {
		t.Fatalf("bad mac format: %q %q", m1, m2)
	}
}
