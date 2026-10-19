package cloudflare

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDNSExistsAndCreate(t *testing.T) {
	var created bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("auth %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"success":true,"result":[]}`))
		case http.MethodPost:
			created = true
			_, _ = w.Write([]byte(`{"success":true,"result":{"id":"rec1"}}`))
		default:
			t.Errorf("method %s", r.Method)
		}
	}))
	defer srv.Close()

	client := New("token", "zone", "CNAME", "target.example", true).WithBase(srv.URL)
	taken, err := client.Exists("budi.example.com")
	if err != nil || taken {
		t.Fatalf("exists %v %v", taken, err)
	}
	id, err := client.Create("budi.example.com")
	if err != nil || id != "rec1" || !created {
		t.Fatalf("create %s %v", id, err)
	}
}
