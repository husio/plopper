package plopper

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type PlopStore interface {
	Create(context.Context, string, string) (PlopID, error)
	ListPlops(context.Context, time.Time, int) ([]*Plop, error)
	Plop(context.Context, PlopID) (*Plop, error)
	Close() error
}

var (
	ErrStore    = errors.New("store")
	ErrNotFound = fmt.Errorf("%w: not found", ErrStore)
)

type sqlPlopStore struct {
	db *sql.DB
}

func OpenSQLitePlopStore(dbPath string) (PlopStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open SQLite database: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS
		plops (
			id BLOB PRIMARY KEY CHECK (length(id) = 16),
			author_id TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			content TEXT NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate database: %w", err)
	}

	return &sqlPlopStore{db: db}, nil
}

func (s *sqlPlopStore) Close() error {
	return s.db.Close()
}

func (s *sqlPlopStore) Plop(ctx context.Context, id PlopID) (*Plop, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT created_at, content FROM plops WHERE id = ? LIMIT 1
	`, id)
	p := Plop{ID: id}
	switch err := row.Scan(&p.CreatedAt, &p.Content); {
	case err == nil:
		return &p, nil
	case err == sql.ErrNoRows:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (s *sqlPlopStore) Create(ctx context.Context, authorID string, content string) (PlopID, error) {
	id := newPlopID()
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO plops (id, author_id, created_at, content) VALUES (?, ?, ?, ?)
	`, id, authorID, now, content)
	return id, err
}

func newPlopID() PlopID {
	id := make(PlopID, 16)
	if _, err := rand.Read(id); err != nil {
		panic(err)
	}
	return id
}

func (s *sqlPlopStore) ListPlops(ctx context.Context, olderThan time.Time, limit int) ([]*Plop, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, author_id, created_at, content
		FROM plops
		WHERE created_at < ?
		ORDER BY created_at DESC
		LIMIT ?
	`, olderThan, limit)

	if err != nil {
		return nil, fmt.Errorf("cannot query plops: %w", err)
	}

	results := make([]*Plop, 0, limit)
	for rows.Next() {
		var p Plop
		if err := rows.Scan(&p.ID, &p.AuthorID, &p.CreatedAt, &p.Content); err != nil {
			return results, fmt.Errorf("cannot scan plop entry: %w", err)
		}
		results = append(results, &p)
	}

	return results, nil

}

type Plop struct {
	ID        PlopID
	AuthorID  string
	CreatedAt time.Time
	Content   string
}

type PlopID []byte

func (id PlopID) String() string {
	return hex.EncodeToString(id)
}
