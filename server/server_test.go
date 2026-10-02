package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"proj/model"
	"proj/storage"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()

	st, err := storage.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if _, err := st.SaveUsers(context.Background(), []model.User{
		{
			Email: "jane.doe@example.com",
			Name:  model.Name{Title: "Ms", First: "Jane", Last: "Doe"},
		},
	}); err != nil {
		t.Fatalf("SaveUsers: %v", err)
	}

	srv, err := New(st)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	return srv.Handler()
}

func postForm(t *testing.T, h http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestIndex_ShowsForm(t *testing.T) {
	h := newTestHandler(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код = %d, ожидался 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `<form method="post"`) {
		t.Fatal("на главной нет формы")
	}
}

func TestLookup_Found(t *testing.T) {
	h := newTestHandler(t)

	rec := postForm(t, h, "/", url.Values{"id": {"1"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("код = %d, ожидался 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "jane.doe@example.com") || !strings.Contains(body, "Jane Doe") {
		t.Fatalf("в ответе нет данных пользователя:\n%s", body)
	}
}

func TestLookup_NotFound(t *testing.T) {
	h := newTestHandler(t)

	rec := postForm(t, h, "/", url.Values{"id": {"999"}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("код = %d, ожидался 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "не найден") {
		t.Fatal("нет сообщения о ненайденном пользователе")
	}
}

func TestLookup_InvalidID(t *testing.T) {
	h := newTestHandler(t)

	for _, id := range []string{"", "abc", "0", "-3"} {
		rec := postForm(t, h, "/", url.Values{"id": {id}})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("id=%q: код = %d, ожидался 400", id, rec.Code)
		}
	}
}

func TestMethodNotAllowed(t *testing.T) {
	h := newTestHandler(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("код = %d, ожидался 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != "GET, POST" {
		t.Fatalf("Allow = %q, ожидалось \"GET, POST\"", allow)
	}
}

func TestUnknownPath(t *testing.T) {
	h := newTestHandler(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("код = %d, ожидался 404", rec.Code)
	}
}
