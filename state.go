package core

import (
	"errors"
	"fmt"
)

// State is what has happened to a cited record since it was published.
type State string

const (
	StateCurrent   State = "current"
	StateRevised   State = "revised"
	StateCorrected State = "corrected"
	StateRetracted State = "retracted"
	StateWithdrawn State = "withdrawn"
)

// ErrNotInChain means the cited record is not in the current record's history.
var ErrNotInChain = errors.New("cited record is not in the current record's history")

// maxChainDepth bounds how far ResolveState walks, so a malicious chain can't loop forever.
const maxChainDepth = 10000

// ResolveState walks supersedes back from current to citedID and reports the
// most serious change on the way: withdrawn, then retracted, then corrected,
// then revised. It also returns the records newer than the cited one, newest
// first, so callers can show their notes.
//
// load fetches a record by ID. It must return verified records.
func ResolveState(citedID string, current *Record, load func(id string) (*Record, error)) (State, []*Record, error) {
	if current.ID == citedID {
		return StateCurrent, nil, nil
	}
	var newer []*Record
	worst := 0
	r := current
	for range maxChainDepth {
		newer = append(newer, r)
		worst = max(worst, severity(r.Change))
		switch r.Supersedes {
		case "":
			return "", newer, ErrNotInChain
		case citedID:
			return stateFor(worst), newer, nil
		}
		prev, err := load(r.Supersedes)
		if err != nil {
			return "", newer, fmt.Errorf("load %s: %w", r.Supersedes, err)
		}
		if prev.ID != r.Supersedes {
			return "", newer, fmt.Errorf("load %s: got record %s", r.Supersedes, prev.ID)
		}
		r = prev
	}
	return "", newer, fmt.Errorf("supersedes chain longer than %d records", maxChainDepth)
}

func severity(c Change) int {
	switch c {
	case ChangeRevision:
		return 1
	case ChangeCorrection:
		return 2
	case ChangeRetraction:
		return 3
	case ChangeWithdrawal:
		return 4
	}
	return 0
}

func stateFor(severity int) State {
	switch severity {
	case 2:
		return StateCorrected
	case 3:
		return StateRetracted
	case 4:
		return StateWithdrawn
	}
	return StateRevised
}
