package service

import "testing"

func TestCanChangeStatus(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{
			name: "created to confirmed",
			from: "created",
			to:   "confirmed",
			want: true,
		},
		{
			name: "created to cancelled",
			from: "created",
			to:   "cancelled",
			want: true,
		},
		{
			name: "confirmed to preparing",
			from: "confirmed",
			to:   "preparing",
			want: true,
		},
		{
			name: "confirmed to cancelled",
			from: "confirmed",
			to:   "cancelled",
			want: true,
		},
		{
			name: "preparing to ready_for_delivery",
			from: "preparing",
			to:   "ready_for_delivery",
			want: true,
		},
		{
			name: "preparing to cancelled",
			from: "preparing",
			to:   "cancelled",
			want: true,
		},
		{
			name: "ready_for_delivery to cancelled",
			from: "ready_for_delivery",
			to:   "cancelled",
			want: false,
		},
		{
			name: "delivering to cancelled",
			from: "delivering",
			to:   "cancelled",
			want: false,
		},
		{
			name: "delivered to cancelled",
			from: "delivered",
			to:   "cancelled",
			want: false,
		},
		{
			name: "cancelled to confirmed",
			from: "cancelled",
			to:   "confirmed",
			want: false,
		},
		{
			name: "created to delivered",
			from: "created",
			to:   "delivered",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanChangeStatus(tt.from, tt.to)

			if got != tt.want {
				t.Fatalf(
					"CanChangeStatus(%q, %q) = %v, want %v",
					tt.from,
					tt.to,
					got,
					tt.want,
				)
			}
		})
	}
}
