package doctor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeFakeBin creates an executable file named `name` (with .exe on Windows)
// inside dir so exec.LookPath(name) resolves to it. LookPath only checks
// existence + executability, it never runs the binary.
func writeFakeBin(t *testing.T, dir, name string) {
	t.Helper()
	fn := name
	if runtime.GOOS == "windows" {
		fn = name + ".exe"
	}
	p := filepath.Join(dir, fn)
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// isolatedPATH sets PATH to only dir so only the fakes we plant are visible.
func isolatedPATH(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir)
}

func TestCommandAllCheck(t *testing.T) {
	dir := t.TempDir()
	isolatedPATH(t, dir)

	t.Run("all present -> OK", func(t *testing.T) {
		writeFakeBin(t, dir, "alpha")
		writeFakeBin(t, dir, "beta")
		c := commandAllCheck("both", Required, []string{"alpha", "beta"}, "fix")
		if !c.OK {
			t.Fatalf("expected OK, got %+v", c)
		}
		if c.Severity != Required {
			t.Errorf("Severity = %v, want Required", c.Severity)
		}
	})

	t.Run("one missing -> not OK, names missing", func(t *testing.T) {
		writeFakeBin(t, dir, "gamma")
		c := commandAllCheck("pair", Required, []string{"gamma", "missingone"}, "fix-it")
		if c.OK {
			t.Fatal("expected OK=false when one missing")
		}
		if c.Fix != "fix-it" {
			t.Errorf("Fix = %q, want fix-it", c.Fix)
		}
		// detail should call out the missing command
		if !strings.Contains(c.Detail, "missingone") {
			t.Errorf("Detail %q should mention missingone", c.Detail)
		}
	})

	t.Run("none present", func(t *testing.T) {
		c := commandAllCheck("none", Warning, []string{"zzz-not-there"}, "fix")
		if c.OK {
			t.Fatal("expected OK=false when nothing present")
		}
	})
}

func TestCommandAnyCheck(t *testing.T) {
	dir := t.TempDir()
	isolatedPATH(t, dir)

	t.Run("first found wins", func(t *testing.T) {
		writeFakeBin(t, dir, "first")
		c := commandAnyCheck("any", Required, []string{"first", "second"}, "fix")
		if !c.OK {
			t.Fatalf("expected OK, got %+v", c)
		}
		if !strings.Contains(c.Detail, "first") {
			t.Errorf("Detail %q should mention the found cmd", c.Detail)
		}
	})

	t.Run("fallback to second", func(t *testing.T) {
		writeFakeBin(t, dir, "fallback")
		c := commandAnyCheck("any", Required, []string{"zzz-missing", "fallback"}, "fix")
		if !c.OK {
			t.Fatal("expected fallback to be found")
		}
	})

	t.Run("none present -> not OK", func(t *testing.T) {
		c := commandAnyCheck("any", Warning, []string{"nope1", "nope2"}, "fix")
		if c.OK {
			t.Fatal("expected OK=false when none present")
		}
	})
}
