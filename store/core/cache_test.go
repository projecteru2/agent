package core

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestStatusCache(t *testing.T) {
	cases := []struct {
		name    string
		ttl     time.Duration
		elapsed time.Duration
		remove  bool
		want    string
		wantOK  bool
		wantLen int
	}{
		{name: "unexpired entry hits", ttl: time.Minute, elapsed: 30 * time.Second, want: "running", wantOK: true, wantLen: 1},
		{name: "expired entry misses and is dropped", ttl: time.Minute, elapsed: time.Minute},
		{name: "deleted entry misses", ttl: time.Minute, remove: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := newStatusCache()
				c.Set("id", "running", tt.ttl)
				if tt.remove {
					c.Delete("id")
				}
				time.Sleep(tt.elapsed)

				got, ok := c.Get("id")
				if got != tt.want || ok != tt.wantOK {
					t.Errorf("Get() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
				}
				if len(c.entries) != tt.wantLen {
					t.Errorf("len(entries) = %d, want %d", len(c.entries), tt.wantLen)
				}
			})
		})
	}
}
