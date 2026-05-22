package outbox

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	worker := New(nil, time.Second)
	if worker == nil {
		t.Fatal("worker is nil")
	}
	if worker.interval != time.Second {
		t.Fatalf("interval = %s, want %s", worker.interval, time.Second)
	}
}
