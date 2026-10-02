package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"proj/model"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchUsersFrom_Success(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("seed") != "abc" {
			t.Errorf("seed = %q, ожидалось abc", q.Get("seed"))
		}
		if q.Get("page") != "2" {
			t.Errorf("page = %q, ожидалось 2", q.Get("page"))
		}
		if q.Get("results") != "3" {
			t.Errorf("results = %q, ожидалось 3", q.Get("results"))
		}
		if q.Get("nat") != "gb" {
			t.Errorf("nat = %q, ожидалось gb", q.Get("nat"))
		}
		writeUsers(t, w, []model.User{{Email: "a@example.com"}, {Email: "b@example.com"}})
	})

	got, err := FetchUsersFrom(context.Background(), http.DefaultClient, srv.URL+"/", "abc", 2, 3)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(got) != 2 || got[0].Email != "a@example.com" {
		t.Fatalf("неожиданный результат: %+v", got)
	}
}

func TestFetchUsersFrom_EmptySeed(t *testing.T) {
	_, err := FetchUsersFrom(context.Background(), http.DefaultClient, DefaultBaseURL, "", 1, 1)
	if err == nil {
		t.Fatal("ожидалась ошибка для пустого seed")
	}
}

func TestFetchUsersFrom_ClampsResults(t *testing.T) {
	var gotResults string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotResults = r.URL.Query().Get("results")
		writeUsers(t, w, nil)
	})

	if _, err := FetchUsersFrom(context.Background(), http.DefaultClient, srv.URL+"/", "s", 1, 999999); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if gotResults != "5000" {
		t.Fatalf("results не ограничен: %q", gotResults)
	}
}

func TestFetchUsersFrom_RetriesOn500(t *testing.T) {
	var calls int32
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeUsers(t, w, []model.User{{Email: "ok@example.com"}})
	})

	got, err := FetchUsersFrom(context.Background(), http.DefaultClient, srv.URL+"/", "s", 1, 1)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(got) != 1 || got[0].Email != "ok@example.com" {
		t.Fatalf("неожиданный результат: %+v", got)
	}
	if n := atomic.LoadInt32(&calls); n != 3 {
		t.Fatalf("calls = %d, ожидалось 3", n)
	}
}

func TestFetchUsersFrom_NoRetryOn400(t *testing.T) {
	var calls int32
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	})

	_, err := FetchUsersFrom(context.Background(), http.DefaultClient, srv.URL+"/", "s", 1, 1)
	if err == nil {
		t.Fatal("ожидалась ошибка")
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("calls = %d, ожидалось 1 (без ретраев)", n)
	}
}

func TestFetchUsersFrom_RetriesOn429(t *testing.T) {
	var calls int32
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		writeUsers(t, w, []model.User{{Email: "ok@example.com"}})
	})

	got, err := FetchUsersFrom(context.Background(), http.DefaultClient, srv.URL+"/", "s", 1, 1)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("получено %d, ожидалось 1", len(got))
	}
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Fatalf("calls = %d, ожидалось 2", n)
	}
}

func TestFetchUsersFrom_NoRetryOn404(t *testing.T) {
	var calls int32
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "not found", http.StatusNotFound)
	})

	_, err := FetchUsersFrom(context.Background(), http.DefaultClient, srv.URL+"/", "s", 1, 1)
	if err == nil {
		t.Fatal("ожидалась ошибка")
	}
	if !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("ошибка не содержит статус и тело: %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("calls = %d, ожидалось 1 (без ретраев)", n)
	}
}

func TestFetchUsersFrom_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := FetchUsersFrom(ctx, http.DefaultClient, DefaultBaseURL, "s", 1, 1)
	if err == nil {
		t.Fatal("ожидалась ошибка отменённого контекста")
	}
}

func writeUsers(t *testing.T, w http.ResponseWriter, users []model.User) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]any{"results": users}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		t.Errorf("не удалось записать ответ: %v", err)
	}
}

func TestFetchUsersFrom_InvalidJSON(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	})

	_, err := FetchUsersFrom(context.Background(), http.DefaultClient, srv.URL+"/", "s", 1, 1)
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("ожидалась ошибка разбора JSON, получено: %v", err)
	}
}
