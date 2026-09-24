//go:build manual

package touchid

import "testing"

func TestManual(t *testing.T) { t.Log(Authenticate("purego manual test")) }
