package cybercat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveCachedAvatarsKeepsCurrentImageAndRemovesOldOnes(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "猫娘42_123_aaaaaaaaaaaa.png")
	old := filepath.Join(dir, "布偶猫42_123_bbbbbbbbbbbb.png")
	otherUser := filepath.Join(dir, "猫娘99_123_cccccccccccc.png")
	for _, name := range []string{keep, old, otherUser} {
		if err := os.WriteFile(name, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	if err := removeCachedAvatars(dir, 42, 123, filepath.Base(keep)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path   string
		exists bool
	}{
		{keep, true},
		{old, false},
		{otherUser, true},
	} {
		_, err := os.Stat(tc.path)
		if (err == nil) != tc.exists {
			t.Errorf("%s exists=%v, want %v (err=%v)", tc.path, err == nil, tc.exists, err)
		}
	}
}
