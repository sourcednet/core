package core

import (
	"errors"
	"testing"
)

// chain builds records r0 <- r1 <- ... with the given changes; changes[i] is
// the change r(i+1) declares.
func chain(changes ...Change) []*Record {
	id := func(i int) string { return RecordIDPrefix + repeat(string(rune('a'+i)), 64) }
	rs := []*Record{{ID: id(0)}}
	for i, c := range changes {
		rs = append(rs, &Record{ID: id(i + 1), Supersedes: id(i), Change: c})
	}
	return rs
}

func loader(rs []*Record) func(string) (*Record, error) {
	return func(id string) (*Record, error) {
		for _, r := range rs {
			if r.ID == id {
				return r, nil
			}
		}
		return nil, errors.New("not found")
	}
}

func TestResolveState(t *testing.T) {
	rs := chain(ChangeRevision, ChangeCorrection, ChangeRevision)
	current := rs[len(rs)-1]
	tests := []struct {
		cited     int
		want      State
		wantNewer int
	}{
		{cited: 3, want: StateCurrent, wantNewer: 0},
		{cited: 2, want: StateRevised, wantNewer: 1},
		{cited: 1, want: StateCorrected, wantNewer: 2},
		{cited: 0, want: StateCorrected, wantNewer: 3},
	}
	for _, tt := range tests {
		got, newer, err := ResolveState(rs[tt.cited].ID, current, loader(rs))
		if err != nil {
			t.Fatalf("cited r%d: %v", tt.cited, err)
		}
		if got != tt.want || len(newer) != tt.wantNewer {
			t.Errorf("cited r%d: got %s with %d newer, want %s with %d", tt.cited, got, len(newer), tt.want, tt.wantNewer)
		}
	}
}

func TestResolveStateMostSeriousWins(t *testing.T) {
	for _, tt := range []struct {
		changes []Change
		want    State
	}{
		{[]Change{ChangeRetraction, ChangeCorrection, ChangeRevision}, StateRetracted},
		{[]Change{ChangeCorrection, ChangeWithdrawal}, StateWithdrawn},
		{[]Change{ChangeRevision, ChangeRevision}, StateRevised},
	} {
		rs := chain(tt.changes...)
		got, _, err := ResolveState(rs[0].ID, rs[len(rs)-1], loader(rs))
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Errorf("changes %v: got %s, want %s", tt.changes, got, tt.want)
		}
	}
}

func TestResolveStateNotInChain(t *testing.T) {
	rs := chain(ChangeRevision)
	_, _, err := ResolveState(RecordIDPrefix+repeat("z", 64), rs[1], loader(rs))
	if !errors.Is(err, ErrNotInChain) {
		t.Fatalf("got %v, want ErrNotInChain", err)
	}
}

func TestResolveStateRejectsWrongRecordFromLoader(t *testing.T) {
	rs := chain(ChangeRevision, ChangeRevision)
	bad := func(string) (*Record, error) { return rs[0], nil } // always returns r0
	if _, _, err := ResolveState(rs[0].ID, rs[2], bad); err == nil {
		t.Fatal("expected an error when the loader returns the wrong record")
	}
}
