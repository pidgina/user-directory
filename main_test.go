package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"proj/model"
)

func makeUsers(n int) []model.User {
	users := make([]model.User, n)
	for i := range users {
		users[i].Email = fmt.Sprintf("user%d@example.com", i)
	}
	return users
}

func TestCollectUsers_Pagination(t *testing.T) {
	var calls []int
	fetch := func(_ context.Context, page, results int) ([]model.User, error) {
		calls = append(calls, results)
		return makeUsers(results), nil
	}

	users, err := collectUsers(context.Background(), fetch, 25, 10)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(users) != 25 {
		t.Fatalf("len(users) = %d, ожидалось 25", len(users))
	}
	wantCalls := []int{10, 10, 5}
	if fmt.Sprint(calls) != fmt.Sprint(wantCalls) {
		t.Fatalf("вызовы = %v, ожидалось %v", calls, wantCalls)
	}
}

func TestCollectUsers_StopsOnEmptyPage(t *testing.T) {
	fetch := func(_ context.Context, page, results int) ([]model.User, error) {
		if page == 1 {
			return makeUsers(4), nil
		}
		return nil, nil
	}

	users, err := collectUsers(context.Background(), fetch, 100, 10)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(users) != 4 {
		t.Fatalf("len(users) = %d, ожидалось 4", len(users))
	}
}

func TestCollectUsers_PartialOnError(t *testing.T) {
	boom := errors.New("network")
	fetch := func(_ context.Context, page, results int) ([]model.User, error) {
		if page == 1 {
			return makeUsers(results), nil
		}
		return nil, boom
	}

	users, err := collectUsers(context.Background(), fetch, 100, 10)
	if !errors.Is(err, boom) {
		t.Fatalf("ожидалась ошибка %v, получена %v", boom, err)
	}
	if len(users) != 10 {
		t.Fatalf("len(users) = %d, ожидалось 10 частичных", len(users))
	}
}

func TestCollectUsers_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fetch := func(_ context.Context, page, results int) ([]model.User, error) {
		return makeUsers(results), nil
	}

	if _, err := collectUsers(ctx, fetch, 100, 10); err == nil {
		t.Fatal("ожидалась ошибка отменённого контекста")
	}
}

func TestCollectUsers_ZeroLimitNotCalled(t *testing.T) {
	fetch := func(_ context.Context, page, results int) ([]model.User, error) {
		t.Fatal("fetch не должен вызываться при limit=0")
		return nil, nil
	}

	if _, err := collectUsers(context.Background(), fetch, 0, 10); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
}

func TestDedupeUsers(t *testing.T) {
	users := []model.User{
		{Email: "a@example.com"},
		{Email: "b@example.com"},
		{Email: "a@example.com"},
		{Email: "c@example.com"},
		{Email: "b@example.com"},
	}

	got := dedupeUsers(users)
	if len(got) != 3 {
		t.Fatalf("len = %d, ожидалось 3", len(got))
	}
	for i, want := range []string{"a@example.com", "b@example.com", "c@example.com"} {
		if got[i].Email != want {
			t.Fatalf("got[%d] = %q, ожидалось %q (порядок)", i, got[i].Email, want)
		}
	}
}

func TestRandomSeed_Unique(t *testing.T) {
	a, err := randomSeed()
	if err != nil {
		t.Fatalf("randomSeed: %v", err)
	}
	b, err := randomSeed()
	if err != nil {
		t.Fatalf("randomSeed: %v", err)
	}
	if a == "" || a == b {
		t.Fatalf("seed не уникален: %q == %q", a, b)
	}
}
