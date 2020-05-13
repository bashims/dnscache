package dnscache

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestResolver_LookupHost(t *testing.T) {
	r := &Resolver{}
	var cacheMiss bool
	r.OnCacheMiss = func() {
		cacheMiss = true
	}
	hosts := []string{"google.com", "google.com.", "netflix.com"}
	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			for _, wantMiss := range []bool{true, false, false} {
				cacheMiss = false
				addrs, err := r.LookupHost(context.Background(), host)
				if err != nil {
					t.Fatal(err)
				}
				if len(addrs) == 0 {
					t.Error("got no record")
				}
				for _, addr := range addrs {
					if net.ParseIP(addr) == nil {
						t.Errorf("got %q; want a literal IP address", addr)
					}
				}
				if wantMiss != cacheMiss {
					t.Errorf("got cache miss=%v, want %v", cacheMiss, wantMiss)
				}
			}
		})
	}
}

func TestClearCache(t *testing.T) {
	r := &Resolver{}
	_, _ = r.LookupHost(context.Background(), "google.com")
	if e := r.cache["hgoogle.com"]; e != nil && !e.used {
		t.Error("cache entry used flag is false, want true")
	}
	r.Refresh(true)
	if e := r.cache["hgoogle.com"]; e != nil && e.used {
		t.Error("cache entry used flag is true, want false")
	}
	r.Refresh(true)
	if e := r.cache["hgoogle.com"]; e != nil {
		t.Error("cache entry is not cleared")
	}
}

// TODO clean up this test.
func TestRefreshWithCallback(t *testing.T) {
	callCount := 0
	host := "google.com"

	expectedResult := &RefreshResult{
		Changed: false,
	}
	r := &Resolver{
		RefreshCallBack: func(actual *RefreshResult) {
			callCount++
			if diff := cmp.Diff(expectedResult, actual); diff != "" {
				t.Errorf("RefreshResult mismatch (-want +got):\n%s", diff)
			}
		},
	}

	_, err := r.LookupHost(context.Background(), host)
	if err != nil {
		t.Errorf("LookupHost() unexpected error = %q", err)
		return
	}

	r.RefreshWithCallback(true)
	if callCount != 1 {
		t.Errorf("RefreshCallBack() expectedResult %d calls, got %d", 1, callCount)
	}

	cur := r.cache["h"+host]
	if cur == nil {
		t.Errorf("Unexpected error, cache missing expectedResult entry %s", host)
		return
	}
	cur.rrs = []string{}
	cur.used = true

	expectedResult = &RefreshResult{
		Changed: true,
	}
	r.RefreshWithCallback(false)
	if callCount != 2 {
		t.Errorf("RefreshCallBack() expectedResult %d calls, got %d", 1, callCount)
	}
}

func TestRaceOnDelete(t *testing.T) {
	r := &Resolver{}
	ls := make(chan bool)
	rs := make(chan bool)

	go func() {
		for {
			select {
			case <-ls:
				return
			default:
				r.LookupHost(context.Background(), "google.com")
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-rs:
				return
			default:
				r.Refresh(true)
				time.Sleep(time.Millisecond)
			}
		}
	}()

	time.Sleep(1 * time.Second)

	ls <- true
	rs <- true

}
