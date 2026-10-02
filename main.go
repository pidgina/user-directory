package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"proj/clients"
	"proj/model"
	"proj/storage"
)

type config struct {
	limit    int
	pageSize int
	dbPath   string
	seed     string
}

func main() {
	cfg, err := parseFlags()
	if err != nil {
		log.Fatal(err)
	}
	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func parseFlags() (config, error) {
	var cfg config
	flag.IntVar(&cfg.limit, "limit", 1000, "сколько пользователей загрузить")
	flag.IntVar(&cfg.pageSize, "page-size", clients.DefaultResults, "размер страницы запроса")
	flag.StringVar(&cfg.dbPath, "db", "users.db", "файл базы данных")
	flag.StringVar(&cfg.seed, "seed", "", "seed для воспроизводимой загрузки")
	flag.Parse()

	if cfg.limit < 1 {
		return config{}, fmt.Errorf("-limit должен быть больше нуля")
	}
	if cfg.pageSize < 1 || cfg.pageSize > clients.MaxResults {
		return config{}, fmt.Errorf("-page-size должен быть в диапазоне 1..%d", clients.MaxResults)
	}
	return cfg, nil
}

func run(cfg config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if cfg.seed == "" {
		seed, err := randomSeed()
		if err != nil {
			return fmt.Errorf("генерация seed: %w", err)
		}
		cfg.seed = seed
	}
	log.Printf("seed=%s (для повтора: -seed=%s)", cfg.seed, cfg.seed)

	client := &http.Client{Timeout: 20 * time.Second}
	fetch := func(ctx context.Context, page, results int) ([]model.User, error) {
		return clients.FetchUsersFrom(ctx, client, clients.DefaultBaseURL, cfg.seed, page, results)
	}

	db, err := storage.Open(ctx, cfg.dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("закрытие БД: %v", err)
		}
	}()

	raw, fetchErr := collectUsers(ctx, fetch, cfg.limit, cfg.pageSize)
	if fetchErr != nil {
		log.Printf("загрузка остановлена: %v", fetchErr)
	}

	// randomuser.me иногда выдаёт двух разных людей с одинаковым email.
	users := dedupeUsers(raw)
	log.Printf("получено %d, уникальных %d", len(raw), len(users))

	saved, err := db.SaveUsers(ctx, users)
	if err != nil {
		return err
	}

	total, err := db.CountUsers(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Сохранено: %d\n", saved)
	fmt.Printf("Всего в базе: %d\n", total)

	if fetchErr != nil {
		return fmt.Errorf("данные загружены не полностью: %w", fetchErr)
	}
	return nil
}

type pageFetcher func(ctx context.Context, page, results int) ([]model.User, error)

func collectUsers(ctx context.Context, fetch pageFetcher, limit, pageSize int) ([]model.User, error) {
	var all []model.User

	for page := 1; len(all) < limit; page++ {
		if err := ctx.Err(); err != nil {
			return all, err
		}

		want := pageSize
		if left := limit - len(all); left < want {
			want = left
		}

		users, err := fetch(ctx, page, want)
		if err != nil {
			return all, err
		}
		if len(users) == 0 {
			break
		}

		all = append(all, users...)
		log.Printf("страница %d: +%d (всего %d)", page, len(users), len(all))
	}

	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func dedupeUsers(users []model.User) []model.User {
	seen := make(map[string]struct{}, len(users))
	out := make([]model.User, 0, len(users))
	for _, u := range users {
		if _, ok := seen[u.Email]; ok {
			continue
		}
		seen[u.Email] = struct{}{}
		out = append(out, u)
	}
	return out
}

func randomSeed() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
