package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "# comment\n\nA_PLAIN=one\nexport B_EXPORT=two\nC_QUOTED=\"three four\"\nD_SINGLE='five'\n" +
		"E_INLINE=six # note\nF_HASH=a#b\nG_EQUALS=x=y\nH_EXISTING=file\nI_EMPTY=\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("H_EXISTING", "real")
	for _, k := range []string{"A_PLAIN", "B_EXPORT", "C_QUOTED", "D_SINGLE", "E_INLINE", "F_HASH", "G_EQUALS", "I_EMPTY"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"A_PLAIN": "one", "B_EXPORT": "two", "C_QUOTED": "three four", "D_SINGLE": "five",
		"E_INLINE": "six", "F_HASH": "a#b", "G_EQUALS": "x=y", "H_EXISTING": "real", "I_EMPTY": "",
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestLoadDotEnvMissingAndMalformed(t *testing.T) {
	dir := t.TempDir()
	if err := LoadDotEnv(filepath.Join(dir, "absent")); err != nil {
		t.Fatalf("missing file must be ignored: %v", err)
	}
	bad := filepath.Join(dir, ".env")
	os.WriteFile(bad, []byte("OK=1\nnot a pair\n"), 0o600)
	if err := LoadDotEnv(bad); err == nil {
		t.Fatal("malformed line accepted")
	}
}
