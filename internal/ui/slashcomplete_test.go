package ui

import "testing"

func TestFilterSlashCmds_slash(t *testing.T) {
	got := filterSlashCmds("/")
	if len(got) != len(slashCmds) {
		t.Errorf("filterSlashCmds(\"/\") = %d results, want %d", len(got), len(slashCmds))
	}
}

func TestFilterSlashCmds_prefix(t *testing.T) {
	got := filterSlashCmds("/r")
	if len(got) != 2 {
		t.Fatalf("filterSlashCmds(\"/r\") = %d results, want 2", len(got))
	}
	if got[0].Name != "/resume" {
		t.Errorf("got[0].Name = %q, want \"/resume\"", got[0].Name)
	}
	if got[1].Name != "/roles" {
		t.Errorf("got[1].Name = %q, want \"/roles\"", got[1].Name)
	}
}

func TestFilterSlashCmds_exact(t *testing.T) {
	got := filterSlashCmds("/exit")
	if len(got) != 1 || got[0].Name != "/exit" {
		t.Errorf("filterSlashCmds(\"/exit\") = %v, want [{/exit ...}]", got)
	}
}

func TestFilterSlashCmds_noMatch(t *testing.T) {
	got := filterSlashCmds("/zzz")
	if len(got) != 0 {
		t.Errorf("filterSlashCmds(\"/zzz\") = %d results, want 0", len(got))
	}
}
