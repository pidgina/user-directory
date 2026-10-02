package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"

	"proj/model"
)

var ErrNotFound = errors.New("пользователь не найден")

type Storage struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    email             TEXT    NOT NULL UNIQUE,
    gender            TEXT    NOT NULL DEFAULT '',
    title             TEXT    NOT NULL DEFAULT '',
    first_name        TEXT    NOT NULL DEFAULT '',
    last_name         TEXT    NOT NULL DEFAULT '',
    city              TEXT    NOT NULL DEFAULT '',
    state             TEXT    NOT NULL DEFAULT '',
    country           TEXT    NOT NULL DEFAULT '',
    postcode          TEXT    NOT NULL DEFAULT '',
    phone             TEXT    NOT NULL DEFAULT '',
    cell              TEXT    NOT NULL DEFAULT '',
    picture_large     TEXT    NOT NULL DEFAULT '',
    picture_medium    TEXT    NOT NULL DEFAULT '',
    picture_thumbnail TEXT    NOT NULL DEFAULT '',
    nat               TEXT    NOT NULL DEFAULT '',
    created_at        TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_users_last_name ON users(last_name);
CREATE INDEX IF NOT EXISTS idx_users_country   ON users(country);
`

func Open(ctx context.Context, path string) (*Storage, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("открытие БД: %w", err)
	}

	// modernc-драйвер плохо переносит параллельную запись, одного соединения хватает.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("БД недоступна: %w", err)
	}

	s := &Storage{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Настройки задаются в строке подключения: драйвер применяет их к каждому
// соединению, включая пересозданные внутри пула.
func dsn(path string) string {
	sep := "?"
	if strings.ContainsRune(path, '?') {
		sep = "&"
	}
	return path + sep + "_busy_timeout=5000&_journal_mode=WAL"
}

func (s *Storage) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("создание схемы: %w", err)
	}
	return nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

// SaveUsers пишет пачку в одной транзакции. Повторный email обновляет запись,
// а не создаёт дубль.
func (s *Storage) SaveUsers(ctx context.Context, users []model.User) (int, error) {
	if len(users) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("начало транзакции: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO users (
            email, gender, title, first_name, last_name,
            city, state, country, postcode,
            phone, cell,
            picture_large, picture_medium, picture_thumbnail,
            nat
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(email) DO UPDATE SET
            gender            = excluded.gender,
            title             = excluded.title,
            first_name        = excluded.first_name,
            last_name         = excluded.last_name,
            city              = excluded.city,
            state             = excluded.state,
            country           = excluded.country,
            postcode          = excluded.postcode,
            phone             = excluded.phone,
            cell              = excluded.cell,
            picture_large     = excluded.picture_large,
            picture_medium    = excluded.picture_medium,
            picture_thumbnail = excluded.picture_thumbnail,
            nat               = excluded.nat,
            updated_at        = datetime('now')
    `)
	if err != nil {
		return 0, fmt.Errorf("подготовка запроса: %w", err)
	}
	defer stmt.Close()

	saved := 0
	for _, u := range users {
		_, err := stmt.ExecContext(ctx,
			u.Email,
			u.Gender,
			u.Name.Title,
			u.Name.First,
			u.Name.Last,
			u.Location.City,
			u.Location.State,
			u.Location.Country,
			u.Location.Postcode,
			u.Phone,
			u.Cell,
			u.Picture.Large,
			u.Picture.Medium,
			u.Picture.Thumbnail,
			u.Nat,
		)
		if err != nil {
			return saved, fmt.Errorf("сохранение %s: %w", u.Email, err)
		}
		saved++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return saved, nil
}

func (s *Storage) CountUsers(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("подсчёт: %w", err)
	}
	return n, nil
}

func (s *Storage) GetUser(ctx context.Context, id int64) (model.User, error) {
	const query = `
        SELECT
            id, email, gender, title, first_name, last_name,
            city, state, country, postcode,
            phone, cell,
            picture_large, picture_medium, picture_thumbnail,
            nat
        FROM users
        WHERE id = ?
    `

	var u model.User
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.Gender,
		&u.Name.Title,
		&u.Name.First,
		&u.Name.Last,
		&u.Location.City,
		&u.Location.State,
		&u.Location.Country,
		&u.Location.Postcode,
		&u.Phone,
		&u.Cell,
		&u.Picture.Large,
		&u.Picture.Medium,
		&u.Picture.Thumbnail,
		&u.Nat,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("выборка id=%d: %w", id, err)
	}
	return u, nil
}
