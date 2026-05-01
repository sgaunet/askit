package client_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sgaunet/askit/internal/client"
)

func TestListModels_Happy(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprint(w, `{"object":"list","data":[{"id":"m1","object":"model"},{"id":"m2","object":"model","owned_by":"me"}]}`)
	}))
	defer srv.Close()
	c := client.New(srv.URL+"/v1", "k", time.Second, time.Second)
	res, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(res.Data) != 2 {
		t.Errorf("data len = %d; want 2", len(res.Data))
	}
	if res.Data[0].ID != "m1" || res.Data[1].ID != "m2" {
		t.Errorf("ids = %+v", res.Data)
	}
	if len(res.Raw) == 0 {
		t.Error("Raw should be populated for --json passthrough")
	}
}

func TestListModels_Empty(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"object":"list","data":[]}`)
	}))
	defer srv.Close()
	c := client.New(srv.URL+"/v1", "", time.Second, time.Second)
	res, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(res.Data) != 0 {
		t.Errorf("want empty, got %v", res.Data)
	}
}

func TestListModels_401(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":{"message":"bad key"}}`)
	}))
	defer srv.Close()
	c := client.New(srv.URL+"/v1", "bad", time.Second, time.Second)
	_, err := c.ListModels(context.Background())
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want APIError, got %T %v", err, err)
	}
	if apiErr.Status != 401 {
		t.Errorf("status = %d", apiErr.Status)
	}
}

func TestListModels_Unreachable(t *testing.T) {
	t.Parallel()
	c := client.New("http://127.0.0.1:1/v1", "", time.Second, time.Second)
	_, err := c.ListModels(context.Background())
	var netErr *client.NetworkError
	if !errors.As(err, &netErr) {
		t.Errorf("want NetworkError, got %T %v", err, err)
	}
}

func TestListModels_MalformedJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"data":`)
	}))
	defer srv.Close()
	c := client.New(srv.URL+"/v1", "", time.Second, time.Second)
	_, err := c.ListModels(context.Background())
	if err == nil {
		t.Fatal("want decode error")
	}
}
