package storage

import (
	"context"
	"errors"
	"testing"

	"proj/model"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	s, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func user(email, first, last string) model.User {
	u := model.User{Email: email}
	u.Name.First = first
	u.Name.Last = last
	u.Location.Country = "United Kingdom"
	u.Nat = "GB"
	return u
}

func count(t *testing.T, s *Storage) int {
	t.Helper()
	n, err := s.CountUsers(context.Background())
	if err != nil {
		t.Fatalf("CountUsers: %v", err)
	}
	return n
}

func TestSaveUsers_Insert(t *testing.T) {
	s := newTestStorage(t)

	saved, err := s.SaveUsers(context.Background(), []model.User{
		user("a@example.com", "A", "One"),
		user("b@example.com", "B", "Two"),
	})
	if err != nil {
		t.Fatalf("SaveUsers: %v", err)
	}
	if saved != 2 {
		t.Fatalf("saved = %d, ожидалось 2", saved)
	}
	if got := count(t, s); got != 2 {
		t.Fatalf("count = %d, ожидалось 2", got)
	}
}

func TestSaveUsers_UpsertNoDuplicates(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	if _, err := s.SaveUsers(ctx, []model.User{user("a@example.com", "Old", "Name")}); err != nil {
		t.Fatalf("первая вставка: %v", err)
	}
	if _, err := s.SaveUsers(ctx, []model.User{user("a@example.com", "New", "Name")}); err != nil {
		t.Fatalf("повторная вставка: %v", err)
	}

	if got := count(t, s); got != 1 {
		t.Fatalf("count = %d, ожидалось 1 (upsert)", got)
	}

	var first string
	if err := s.db.QueryRowContext(ctx, `SELECT first_name FROM users WHERE email = ?`, "a@example.com").Scan(&first); err != nil {
		t.Fatalf("выборка: %v", err)
	}
	if first != "New" {
		t.Fatalf("first_name = %q, ожидалось New (обновление)", first)
	}
}

func TestSaveUsers_DuplicateWithinBatch(t *testing.T) {
	s := newTestStorage(t)

	saved, err := s.SaveUsers(context.Background(), []model.User{
		user("dup@example.com", "First", "Variant"),
		user("dup@example.com", "Second", "Variant"),
	})
	if err != nil {
		t.Fatalf("SaveUsers: %v", err)
	}
	if saved != 2 {
		t.Fatalf("saved = %d, ожидалось 2 обработанных", saved)
	}
	if got := count(t, s); got != 1 {
		t.Fatalf("count = %d, ожидалось 1", got)
	}
}

func TestSaveUsers_Empty(t *testing.T) {
	s := newTestStorage(t)

	saved, err := s.SaveUsers(context.Background(), nil)
	if err != nil {
		t.Fatalf("SaveUsers: %v", err)
	}
	if saved != 0 {
		t.Fatalf("saved = %d, ожидалось 0", saved)
	}
}

func TestOpen_AppliesPragmas(t *testing.T) {
	s := newTestStorage(t)

	var busyTimeout int
	if err := s.db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("PRAGMA busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d, ожидалось 5000", busyTimeout)
	}
}

func TestGetUser(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	if _, err := s.SaveUsers(ctx, []model.User{user("look@example.com", "Look", "Me")}); err != nil {
		t.Fatalf("SaveUsers: %v", err)
	}

	got, err := s.GetUser(ctx, 1)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Email != "look@example.com" || got.Name.First != "Look" {
		t.Fatalf("неожиданный пользователь: %+v", got)
	}

	if _, err := s.GetUser(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ожидался ErrNotFound, получено: %v", err)
	}
}

func TestSaveUsers_ContextCancelled(t *testing.T) {
	s := newTestStorage(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := s.SaveUsers(ctx, []model.User{user("a@example.com", "A", "One")}); err == nil {
		t.Fatal("ожидалась ошибка отменённого контекста")
	}
}
