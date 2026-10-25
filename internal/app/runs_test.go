package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAppendAndRead writes two runs and reads them back with Latest.
func TestAppendAndRead(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, time.October, 10, 9, 0, 0, 0, time.UTC)
	first := Run{
		Time: at, Key: RunKey("checkout/card-rejected", "expired-cards"), Result: RunResultFail,
		Author: "Sam", Values: []RunPair{{"card", "expired"}},
		Fingerprint: []RunPair{{"steps", "1a2b"}}, Note: "tab\there",
	}
	second := first
	second.Time, second.Result, second.Note = at.Add(time.Minute), RunResultPass, ""
	for _, run := range []Run{second, first} {
		if err := AppendRun(root, run); err != nil {
			t.Fatal(err)
		}
	}
	text, err := os.ReadFile(filepath.Join(root, "runs", "2026-10-10.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "\tcard=expired\tfp:steps=1a2b\ttab here\n") {
		t.Errorf("run file = %q", text)
	}
	all, err := ReadRuns(root)
	if err != nil || len(all) != 2 {
		t.Fatalf("Read = %+v, %v", all, err)
	}
	latest, ok := LatestRun(all, "checkout/card-rejected/expired-cards",
		[]RunPair{{"card", "expired"}})
	if !ok || latest.Result != RunResultPass || latest.Fingerprint[0].Value != "1a2b" {
		t.Errorf("Latest = %+v, %v", latest, ok)
	}
	if _, ok := LatestRun(all, "checkout/card-rejected/expired-cards", nil); ok {
		t.Error("Latest matched a run with other row values")
	}
}

// TestReadSkipsBadLines keeps the good runs when a line can't be read.
func TestReadSkipsBadLines(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(RunsDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	text := "<<<<<<< HEAD\n2026-10-10T09:00:00Z\tsign-in/magic-link\tpass\tAli\t\n"
	if err := os.WriteFile(filepath.Join(RunsDir(root), "2026-10-10.log"), []byte(text),
		0o644); err != nil {
		t.Fatal(err)
	}
	all, err := ReadRuns(root)
	if err != nil || len(all) != 1 || all[0].Author != "Ali" {
		t.Errorf("Read = %+v, %v", all, err)
	}
}

// TestReadNoRuns reads a folder with no runs/ as no runs.
func TestReadNoRuns(t *testing.T) {
	all, err := ReadRuns(t.TempDir())
	if err != nil || len(all) != 0 {
		t.Errorf("Read = %+v, %v", all, err)
	}
}
