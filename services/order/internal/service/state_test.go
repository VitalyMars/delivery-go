package service

import "testing"

func TestCanChangeStatus(t *testing.T) {
	for _, tt := range []struct {
		from, to string
		want     bool
	}{
		{"created", "ready_for_delivery", true}, {"created", "cancelled", true},
		{"created", "courier_assigned", false}, {"ready_for_delivery", "courier_assigned", true},
		{"courier_assigned", "delivered", true}, {"delivering", "delivered", true},
		{"delivered", "courier_assigned", false}, {"cancelled", "ready_for_delivery", false},
		{"unknown", "delivered", false}, {"ready_for_delivery", "cancelled", false},
	} {
		if got := CanChangeStatus(tt.from, tt.to); got != tt.want {
			t.Errorf("%s -> %s: got %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}
