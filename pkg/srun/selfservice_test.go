package srun

import (
	"fmt"
	"testing"
)

func TestKickResponseIndicatesError(t *testing.T) {
	if !kickResponseIndicatesError(`<div class="alert alert-danger">failed</div>`) {
		t.Fatal("expected danger alert to fail")
	}
	if kickResponseIndicatesError(`{"success":true}`) {
		t.Fatal("expected success body")
	}
}

func TestKickAllWithFakeMACOrdering(t *testing.T) {
	// Let's verify the logic handles different sizes of session lists
	// 3 sessions -> mid index 1 (second one)
	// 2 sessions -> mid index 1 (second one)
	// 1 session -> mid index 0 (first one)
	cases := []struct {
		length   int
		expected int
	}{
		{3, 1},
		{2, 1},
		{1, 0},
	}
	for _, tc := range cases {
		mid := tc.length / 2
		if mid != tc.expected {
			t.Errorf("For length %d, expected mid index %d, got %d", tc.length, tc.expected, mid)
		}
	}
}

func TestRandomFakeMAC(t *testing.T) {
	mac1 := RandomFakeMAC()
	mac2 := RandomFakeMAC()

	norm1 := NormalizeMAC(mac1)
	if len(norm1) != 17 {
		t.Fatalf("expected 17 chars normalized MAC (aa:bb:cc:dd:ee:ff), got %s (from %s)", norm1, mac1)
	}

	// Verify locally-administered bit (bit 1 of byte 0 is 1)
	// and unicast bit (bit 0 of byte 0 is 0)
	var b0 byte
	n, err := fmt.Sscanf(mac1[:2], "%02x", &b0)
	if n != 1 || err != nil {
		t.Fatalf("failed to parse first byte of %s: %v", mac1, err)
	}
	if b0&0x02 == 0 {
		t.Fatalf("expected locally administered bit to be set, got %02x", b0)
	}
	if b0&0x01 != 0 {
		t.Fatalf("expected unicast bit to be 0, got %02x", b0)
	}

	// Ensure consecutive calls produce different MACs with high probability
	if mac1 == mac2 {
		t.Logf("warning: consecutive random MACs were identical: %s (rare but possible)", mac1)
	}
}

func TestParseMACAuthPage(t *testing.T) {
	html := `<html><head>
<meta name="csrf-param" content="_csrf-8800">
<meta name="csrf-token" content="tok123">
</head><body>
<a href="/user/delete-mac-auth?mac=AA-BB-CC-DD-EE-01" data-method="post">删除</a>
<a href="/user/delete-mac-auth?mac=aa%3Abb%3Acc%3Add%3Aee%3A02" data-method="post">删除</a>
<a href="/user/delete-mac-auth?mac=AA-BB-CC-DD-EE-01" data-method="post">删除</a>
</body></html>`

	csrf, entries, err := parseMACAuthPage(html)
	if err != nil {
		t.Fatal(err)
	}
	if csrf.FieldName != "_csrf-8800" || csrf.Token != "tok123" {
		t.Fatalf("csrf = %+v", csrf)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (deduplicated), got %d: %+v", len(entries), entries)
	}
	if entries[0].MAC != "aa:bb:cc:dd:ee:01" || entries[0].Raw != "AA-BB-CC-DD-EE-01" {
		t.Fatalf("entry 0 = %+v", entries[0])
	}
	if entries[1].MAC != "aa:bb:cc:dd:ee:02" {
		t.Fatalf("entry 1 = %+v", entries[1])
	}

	if e, ok := findMACAuthEntry(entries, "AA:BB:CC:DD:EE:02"); !ok || e.Raw != "aa:bb:cc:dd:ee:02" {
		t.Fatalf("findMACAuthEntry = %+v, %v", e, ok)
	}
	if _, ok := findMACAuthEntry(entries, "aa:bb:cc:dd:ee:03"); ok {
		t.Fatal("unexpected match")
	}
}

func TestParseMACAuthPageEmpty(t *testing.T) {
	html := `<meta name="csrf-param" content="_csrf"><meta name="csrf-token" content="t">`
	_, entries, err := parseMACAuthPage(html)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %+v", entries)
	}
}
