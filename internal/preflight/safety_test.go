package preflight

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestVerifierCannotSpawnProcesses proves the preflight verifier is structurally
// incapable of running docker/Dockhand/systemctl/Maestro lifecycle mutations:
// no non-test source file in the verifier (the internal/preflight package or the
// ok-folio-preflight CLI) imports os/exec or os/syscall. Parsing the AST keeps
// the check on real imports, not comment/string mentions of those tools.
func TestVerifierCannotSpawnProcesses(t *testing.T) {
	root, err := FindRepoRoot(".")
	if err != nil {
		t.Fatalf("FindRepoRoot: %v", err)
	}
	forbidden := map[string]bool{
		"os/exec":          true,
		"syscall":          true,
		"golang.org/x/sys": true,
	}
	dirs := []string{
		filepath.Join(root, "internal", "preflight"),
		filepath.Join(root, "cmd", "ok-folio-preflight"),
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			for _, imp := range file.Imports {
				pkg, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					t.Fatalf("unquote import in %s: %v", path, err)
				}
				if forbidden[pkg] {
					t.Errorf("%s imports %q; the verifier must not be able to spawn processes or make syscalls", path, pkg)
				}
			}
		}
	}
}
