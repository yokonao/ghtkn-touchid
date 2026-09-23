// Package passphrase keeps the ghtkn agent passphrase in Keychain behind Touch ID.
//
// A reset stages the new passphrase as pending, then promotes it to committed and
// finally to active, so an interrupted reset leaves a candidate that still matches
// the agent.
package passphrase

import (
	"errors"
	"fmt"
	"os/user"

	"github.com/yokonao/ghtkn-touchid/go/internal/keychain"
	"github.com/yokonao/ghtkn-touchid/go/internal/touchid"
)

const (
	activeService    = "ghtkn-touchid.agent-passphrase"
	pendingService   = activeService + ".pending"
	committedService = activeService + ".committed"
)

// ErrKeptCommitted means the new passphrase is stored but was not promoted to
// active; Load still finds it.
var ErrKeptCommitted = errors.New("kept the committed passphrase for recovery")

type items struct {
	active, pending, committed keychain.Item
}

func itemsForUser() (items, error) {
	u, err := user.Current()
	if err != nil {
		return items{}, err
	}
	item := func(service string) keychain.Item {
		return keychain.Item{Service: service, Account: u.Username}
	}
	return items{item(activeService), item(pendingService), item(committedService)}, nil
}

// Load requires Touch ID and returns every stored candidate, most recent first.
func Load() ([][]byte, error) {
	if err := touchid.Authenticate("Unlock the ghtkn agent"); err != nil {
		return nil, err
	}
	i, err := itemsForUser()
	if err != nil {
		return nil, err
	}
	var candidates [][]byte
	var firstErr error
	for _, item := range []keychain.Item{i.committed, i.active, i.pending} {
		data, err := keychain.Read(item)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if data != nil {
			candidates = append(candidates, data)
		}
	}
	if len(candidates) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, errors.New("the ghtkn passphrase is not stored in Keychain")
	}
	return candidates, nil
}

// Stage requires Touch ID and stores the passphrase as pending.
func Stage(passphrase []byte) error {
	if err := touchid.Authenticate("Reset the ghtkn agent passphrase"); err != nil {
		return err
	}
	i, err := itemsForUser()
	if err != nil {
		return err
	}
	if err := keychain.Delete(i.pending); err != nil {
		return err
	}
	return keychain.Upsert(i.pending, passphrase)
}

// Commit promotes the pending passphrase to active. An error wrapping
// ErrKeptCommitted is a warning: the passphrase is safe as committed.
func Commit() error {
	i, err := itemsForUser()
	if err != nil {
		return err
	}
	if err := keychain.Delete(i.committed); err != nil {
		return err
	}
	if err := keychain.Rename(i.pending, committedService); err != nil {
		return err
	}
	err = keychain.Delete(i.active)
	if err == nil {
		err = keychain.Rename(i.committed, activeService)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrKeptCommitted, err)
	}
	return nil
}
