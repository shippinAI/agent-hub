package secretbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSealOpenAndKeyFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "k", "secret.key")
	b, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("key file mode %v", st.Mode().Perm())
	}
	ct, err := b.Seal([]byte("sk-live-123"), []byte("row1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.OpenBytes(ct, []byte("row2")); err == nil {
		t.Fatal("aad must bind ciphertext to its row")
	}
	b2, _ := Open(p)
	got, err := b2.OpenBytes(ct, []byte("row1"))
	if err != nil || string(got) != "sk-live-123" {
		t.Fatalf("reopen: %q %v", got, err)
	}
	if fp := Fp([]byte("sk-live-123")); len(fp) != 8 {
		t.Fatalf("fp %q", fp)
	}
	os.Chmod(p, 0o644)
	if _, err := Open(p); err == nil {
		t.Fatal("world-readable key file must be refused")
	}
}
