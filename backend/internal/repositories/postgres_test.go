package repositories

import "testing"

func TestNew(t *testing.T) {
	repo := New(nil)
	if repo == nil {
		t.Fatal("repo is nil")
	}
}
