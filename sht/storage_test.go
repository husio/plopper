package sht

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestCreateAndList(t *testing.T) {
	ctx := context.Background()

	store, err := OpenSQLitePlopStore(":memory:")
	if err != nil {
		t.Fatalf("cannot open plop store: %s", err)
	}
	defer store.Close()

	firstID, err := store.Create(ctx, "first")
	if err != nil {
		t.Fatalf("cannot create first plop: %s", err)
	}
	t.Logf("first plop ID: %s", firstID)

	if first, err := store.Plop(ctx, firstID); err != nil {
		t.Fatalf("cannot fetch first plop: %s", err)
	} else if !bytes.Equal(first.ID, firstID) || first.Content != "first" {
		t.Fatalf("unexpected plop: %+v", first)
	}

	secondID, err := store.Create(ctx, "second")
	if err != nil {
		t.Fatalf("cannot create second plop: %s", err)
	}
	t.Logf("second plop ID: %s", secondID)

	if second, err := store.Plop(ctx, secondID); err != nil {
		t.Fatalf("cannot fetch second plop: %s", err)
	} else if !bytes.Equal(second.ID, secondID) || second.Content != "second" {
		t.Fatalf("unexpected plop: %+v", second)
	}

	plops, err := store.ListPlops(ctx, time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("cannot list plops: %s", err)
	}
	if len(plops) != 2 {
		t.Fatalf("want 2 plops, got %d", len(plops))
	}
	if !bytes.Equal(plops[0].ID, secondID) || !bytes.Equal(plops[1].ID, firstID) {
		t.Fatalf("invalid listing order: 0:%s 1:%s", plops[0].ID, plops[1].ID)
	}
}
