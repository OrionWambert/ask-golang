package history

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEntry(t *testing.T, s Store, id string) {
	t.Helper()
	body := `{"id":"` + id + `","prompt":"p-` + id + `","answer":"a"}`
	if err := os.WriteFile(filepath.Join(s.Dir, id+".json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSaveThenGet(t *testing.T) {
	s := Store{Dir: filepath.Join(t.TempDir(), "history")}
	e := &Entry{Prompt: "q", Answer: "a"}
	if err := s.Save(e); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Prompt != "q" || got.Answer != "a" || got.Time.IsZero() {
		t.Fatalf("unexpected entry: %+v", got)
	}
}

func TestListIsNewestFirstAndLimited(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	for _, id := range []string{"20260101-000000-000", "20260301-000000-000", "20260201-000000-000"} {
		writeEntry(t, s, id)
	}
	got, err := s.List(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "20260301-000000-000" || got[1].ID != "20260201-000000-000" {
		t.Fatalf("unexpected list: %+v", got)
	}
}

func TestListMissingDirIsEmpty(t *testing.T) {
	got, err := Store{Dir: filepath.Join(t.TempDir(), "none")}.List(0)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, err %v", got, err)
	}
}

func TestGetRejectsPathTraversal(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	for _, id := range []string{"", "../secret", `..\secret`} {
		if _, err := s.Get(id); err == nil {
			t.Errorf("no error for %q", id)
		}
	}
}

func TestClearOnlyRemovesEntries(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	writeEntry(t, s, "20260101-000000-000")
	other := filepath.Join(s.Dir, "keep.txt")
	if err := os.WriteFile(other, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	removed, err := s.Clear()
	if err != nil || removed != 1 {
		t.Fatalf("removed %d, err %v", removed, err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatal(err)
	}
}
