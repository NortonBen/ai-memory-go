package sessionid

import "testing"

func TestForDataPointAdd(t *testing.T) {
	sid, g := ForDataPointAdd("")
	if g || sid != DefaultName {
		t.Fatalf("empty: got sid=%q global=%v", sid, g)
	}
	sid, g = ForDataPointAdd("  global  ")
	if !g || sid != "" {
		t.Fatalf("global: got sid=%q global=%v", sid, g)
	}
	sid, g = ForDataPointAdd("proj-a")
	if g || sid != "proj-a" {
		t.Fatalf("named: got sid=%q global=%v", sid, g)
	}
}

func TestForEngineContext(t *testing.T) {
	if ForEngineContext("") != DefaultName || ForEngineContext("global") != DefaultName {
		t.Fatal("expected default context")
	}
	if ForEngineContext("x") != "x" {
		t.Fatal("named preserved")
	}
}

func TestForBulkDeleteAll(t *testing.T) {
	_, _, err := ForBulkDeleteAll("")
	if err == nil {
		t.Fatal("want error")
	}
	u, n, err := ForBulkDeleteAll("global")
	if err != nil || !u || n != "" {
		t.Fatalf("global: u=%v n=%q err=%v", u, n, err)
	}
	u, n, err = ForBulkDeleteAll("proj")
	if err != nil || u || n != "proj" {
		t.Fatalf("named: u=%v n=%q err=%v", u, n, err)
	}
}
