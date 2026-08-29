package gatewayclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecuteLocalGET(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hello" {
			t.Errorf("path=%s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("local response"))
	}))
	defer server.Close()
	response, err := ExecuteLocal(context.Background(), server.URL, Request{Method: "GET", Path: "/hello"})
	if err != nil || response.StatusCode != http.StatusCreated || string(response.Body) != "local response" {
		t.Fatalf("response=%#v err=%v", response, err)
	}
}

func TestNormalizeLocalAddressAcceptsPortOnly(t *testing.T) {
	address, err := normalizeLocalAddress("3000")
	if err != nil || address != "http://127.0.0.1:3000" {
		t.Fatalf("address=%q err=%v", address, err)
	}
	address, err = normalizeLocalAddress("localhost:3000/")
	if err != nil || address != "http://localhost:3000" {
		t.Fatalf("address=%q err=%v", address, err)
	}
}

func TestExecuteLocalForwardsQueryString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("page"); got != "2" {
			t.Errorf("page=%q", got)
		}
	}))
	defer server.Close()
	if _, err := ExecuteLocal(context.Background(), server.URL, Request{Method: http.MethodGet, Path: "/items?page=2"}); err != nil {
		t.Fatalf("ExecuteLocal: %v", err)
	}
}

func TestExecuteLocalForwardsRequestBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != "request payload" {
			t.Errorf("body=%q err=%v", body, err)
		}
	}))
	defer server.Close()
	if _, err := ExecuteLocal(context.Background(), server.URL, Request{Method: http.MethodGet, Path: "/", Body: []byte("request payload")}); err != nil {
		t.Fatalf("ExecuteLocal: %v", err)
	}
}
func TestExecuteLocalForwardsCommonHTTPMethods(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Method", r.Method)
	}))
	defer server.Close()
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			response, err := ExecuteLocal(context.Background(), server.URL, Request{Method: method, Path: "/"})
			if err != nil || response.Headers.Get("X-Method") != method {
				t.Fatalf("response=%#v err=%v", response, err)
			}
		})
	}
}

func TestExecuteLocalForwardsRequestAndResponseHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Values("X-Trace"); len(got) != 2 || got[0] != "first" || got[1] != "second" {
			t.Errorf("request headers = %#v", r.Header)
		}
		w.Header().Add("X-Local", "one")
		w.Header().Add("X-Local", "two")
	}))
	defer server.Close()
	response, err := ExecuteLocal(context.Background(), server.URL, Request{Method: http.MethodGet, Path: "/", Headers: http.Header{"X-Trace": {"first", "second"}}})
	if err != nil || response.Headers.Values("X-Local")[0] != "one" || response.Headers.Values("X-Local")[1] != "two" {
		t.Fatalf("response=%#v err=%v", response, err)
	}
}

func TestWithoutHopByHopHeadersFiltersConnectionNamedHeader(t *testing.T) {
	headers := withoutHopByHopHeaders(http.Header{"Connection": {"X-Remove"}, "X-Remove": {"secret"}, "X-Keep": {"ok"}})
	if headers.Get("Connection") != "" || headers.Get("X-Remove") != "" || headers.Get("X-Keep") != "ok" {
		t.Fatalf("headers=%#v", headers)
	}
}

func TestLocalForwardersRejectEmptyDestination(t *testing.T) {
	if _, err := ExecuteLocal(context.Background(), "", Request{Method: http.MethodGet, Path: "/"}); err == nil {
		t.Fatal("empty local destination was accepted")
	}
	if err := StreamLocal(context.Background(), "", Request{Method: http.MethodGet, Path: "/"}, func(LocalResponse) error { return nil }, func([]byte) error { return nil }); err == nil {
		t.Fatal("empty local destination was accepted")
	}
}
