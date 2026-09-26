// Package keychain stores generic passwords in the login Keychain, where
// SecItemAdd's default access list trusts only the running binary. That Keychain
// needs no keychain-access-groups entitlement, which ad-hoc signed binaries
// cannot carry.
package keychain

import (
	"errors"
	"fmt"
	"maps"

	"github.com/yokonao/appleframeworks/security"
)

type Item struct {
	Service string
	Account string
}

func query(item Item, extra security.Attrs) security.Attrs {
	q := security.Attrs{
		security.Class:       security.ClassGenericPassword,
		security.AttrService: item.Service,
		security.AttrAccount: item.Account,
	}
	maps.Copy(q, extra)
	return q
}

// Read returns nil without an error when the item does not exist.
func Read(item Item) ([]byte, error) {
	data, err := security.CopyMatching(query(item, security.Attrs{security.ReturnData: true, security.MatchLimit: security.MatchLimitOne}))
	if errors.Is(err, security.ErrItemNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s from Keychain: %w", item.Service, err)
	}
	if len(data.([]byte)) == 0 {
		return nil, errors.New("the Keychain item " + item.Service + " is empty")
	}
	return data.([]byte), nil
}

// Add stores a new item.
func Add(item Item, value []byte) error {
	if _, err := security.Add(query(item, security.Attrs{security.ValueData: value})); err != nil {
		return fmt.Errorf("store %s in Keychain: %w", item.Service, err)
	}
	return nil
}

// Delete succeeds when the item does not exist.
func Delete(item Item) error {
	if err := security.Delete(query(item, nil)); err != nil && !errors.Is(err, security.ErrItemNotFound) {
		return fmt.Errorf("remove %s from Keychain: %w", item.Service, err)
	}
	return nil
}

// Rename moves the item to another service under the same account.
func Rename(item Item, service string) error {
	if err := security.Update(query(item, nil), security.Attrs{security.AttrService: service}); err != nil {
		return fmt.Errorf("rename %s to %s in Keychain: %w", item.Service, service, err)
	}
	return nil
}
