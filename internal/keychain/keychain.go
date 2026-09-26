// Package keychain stores generic passwords in the default (login) Keychain with
// an access list that trusts only the running binary. That Keychain needs no
// keychain-access-groups entitlement, which ad-hoc signed binaries cannot carry.
// Its APIs are all deprecated, but it is the only one usable without it.
package keychain

import (
	"errors"
	"fmt"
	"maps"

	"github.com/yokonao/appleframeworks/security"
	"github.com/yokonao/appleframeworks/security/legacy"
)

type Item struct {
	Service string
	Account string
}

// query matches the item in the default Keychain.
func query(item Item, extra security.Attrs) (security.Attrs, error) {
	keychain, err := legacy.DefaultKeychain()
	if err != nil {
		return nil, err
	}
	q := security.Attrs{
		security.Class:       security.ClassGenericPassword,
		security.AttrService: item.Service,
		security.AttrAccount: item.Account,
		security.UseKeychain: keychain,
	}
	maps.Copy(q, extra)
	return q, nil
}

// Read returns nil without an error when the item does not exist.
func Read(item Item) ([]byte, error) {
	q, err := query(item, security.Attrs{security.ReturnData: true, security.MatchLimit: security.MatchLimitOne})
	var data any
	if err == nil {
		data, err = security.CopyMatching(q)
	}
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

// Upsert writes the value and resets the access list to the running binary.
func Upsert(item Item, value []byte) error {
	err := func() error {
		access, err := legacy.NewAccess("ghtkn agent passphrase")
		if err != nil {
			return err
		}
		attrs := security.Attrs{security.ValueData: value, security.AttrAccess: access}
		q, err := query(item, nil)
		if err != nil {
			return err
		}
		err = security.Update(q, attrs)
		if errors.Is(err, security.ErrItemNotFound) {
			maps.Copy(q, attrs)
			_, err = security.Add(q)
		}
		return err
	}()
	if err != nil {
		return fmt.Errorf("store %s in Keychain: %w", item.Service, err)
	}
	return nil
}

// Delete succeeds when the item does not exist.
func Delete(item Item) error {
	q, err := query(item, nil)
	if err == nil {
		err = security.Delete(q)
	}
	if err != nil && !errors.Is(err, security.ErrItemNotFound) {
		return fmt.Errorf("remove %s from Keychain: %w", item.Service, err)
	}
	return nil
}

// Rename moves the item to another service under the same account.
func Rename(item Item, service string) error {
	q, err := query(item, nil)
	if err == nil {
		err = security.Update(q, security.Attrs{security.AttrService: service})
	}
	if err != nil {
		return fmt.Errorf("rename %s to %s in Keychain: %w", item.Service, service, err)
	}
	return nil
}
