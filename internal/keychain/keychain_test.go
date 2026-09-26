package keychain

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	item := Item{Service: fmt.Sprintf("ghtkn-touchid.test.%d", os.Getpid()), Account: "test"}
	renamed := item.Service + ".renamed"
	t.Cleanup(func() {
		Delete(item)
		Delete(Item{Service: renamed, Account: item.Account})
	})

	if data, err := Read(item); data != nil || err != nil {
		t.Fatalf("Read of a missing item = %q, %v", data, err)
	}
	if err := Add(item, []byte("secret")); err != nil {
		t.Fatal(err)
	}
	if data, err := Read(item); string(data) != "secret" || err != nil {
		t.Fatalf("Read = %q, %v", data, err)
	}
	if err := Rename(item, renamed); err != nil {
		t.Fatal(err)
	}
	if data, err := Read(Item{Service: renamed, Account: item.Account}); string(data) != "secret" || err != nil {
		t.Fatalf("Read after Rename = %q, %v", data, err)
	}
	if err := Delete(Item{Service: renamed, Account: item.Account}); err != nil {
		t.Fatal(err)
	}
	if err := Delete(item); err != nil {
		t.Fatalf("Delete of a missing item: %v", err)
	}
	if err := Rename(item, renamed); err == nil || !strings.Contains(err.Error(), "could not be found") {
		t.Fatalf("Rename of a missing item: %v", err)
	}
}
