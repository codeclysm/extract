package extract

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSafeJoin(t *testing.T) {
	ok := func(parent, subdir string) {
		_, err := safeJoin(parent, subdir)
		require.NoError(t, err, "joining '%s' and '%s'", parent, subdir)
	}
	ko := func(parent, subdir string) {
		_, err := safeJoin(parent, subdir)
		require.Error(t, err, "joining '%s' and '%s'", parent, subdir)
	}
	ok("/", "more/path")
	ok("/path", "more/path")
	ok("/path/", "more/path")
	ok("/path/subdir", "more/path")
	ok("/path/subdir/", "more/path")

	ok("/", "..") // ! since we are extracting to / is ok-ish to accept ".."?
	ko("/path", "..")
	ko("/path/", "..")
	ko("/path/subdir", "..")
	ko("/path/subdir/", "..")

	ok("/", "../pathpath") // ! since we are extracting to / is ok-ish to accept "../pathpath"?
	ko("/path", "../pathpath")
	ko("/path/", "../pathpath")
	ko("/path/subdir", "../pathpath")
	ko("/path/subdir/", "../pathpath")
}
