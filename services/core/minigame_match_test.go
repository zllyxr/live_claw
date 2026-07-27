package main

import (
	"testing"
	"time"
)

func TestTableMatcherFillsThenRotatesAcrossThousandTables(t *testing.T) {
	matcher := newTableMatcher(1000)
	now := time.Unix(1_800_000_000, 0)
	if got := matcher.assign("ddz", 101, 3, now); got != 1 {
		t.Fatalf("first player table=%d, want 1", got)
	}
	if got := matcher.assign("ddz", 102, 3, now); got != 1 {
		t.Fatalf("second player table=%d, want 1", got)
	}
	if got := matcher.assign("ddz", 103, 3, now); got != 1 {
		t.Fatalf("third player table=%d, want 1", got)
	}
	if got := matcher.assign("ddz", 104, 3, now); got != 2 {
		t.Fatalf("next table=%d, want 2", got)
	}
	if got := matcher.assign("mahjong", 201, 4, now); got != 1 {
		t.Fatalf("each game must own an independent 1000-table pool; got %d", got)
	}
	if got := matcher.assign("ddz", 101, 3, now.Add(time.Minute)); got != 1 {
		t.Fatalf("same user should keep its live assignment; got %d", got)
	}
}

func TestTableMatcherStartsANewTableAfterHumanWindow(t *testing.T) {
	matcher := newTableMatcher(1000)
	now := time.Unix(1_800_000_000, 0)

	if got := matcher.assign("paodekuai", 301, 3, now); got != 1 {
		t.Fatalf("first player table=%d, want 1", got)
	}
	if got := matcher.assign("paodekuai", 302, 3, now.Add(matchHumanWindow-time.Millisecond)); got != 1 {
		t.Fatalf("player inside aggregation window table=%d, want 1", got)
	}
	if got := matcher.assign("paodekuai", 303, 3, now.Add(matchHumanWindow+time.Millisecond)); got != 2 {
		t.Fatalf("player after aggregation window table=%d, want 2", got)
	}
	if got := matcher.assign("paodekuai", 301, 3, now.Add(time.Minute)); got != 1 {
		t.Fatalf("reconnecting player must keep original table, got %d", got)
	}
}
