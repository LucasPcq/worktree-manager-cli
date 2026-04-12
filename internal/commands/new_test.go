package commands

import "testing"

func TestBranchInList_Found(t *testing.T) {
	branches := []string{"main", "feature/auth", "develop"}
	if !branchInList(branches, "feature/auth") {
		t.Error("expected true for existing branch")
	}
}

func TestBranchInList_NotFound(t *testing.T) {
	branches := []string{"main", "feature/auth", "develop"}
	if branchInList(branches, "feature/missing") {
		t.Error("expected false for missing branch")
	}
}

func TestBranchInList_EmptyList(t *testing.T) {
	if branchInList(nil, "main") {
		t.Error("expected false for empty list")
	}
}
