package logs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPathsUserInputs(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "日志 [production]")
	if err := os.MkdirAll(filepath.Join(directory, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(directory, "app[01].log")
	if err := os.WriteFile(file, []byte("ERROR failed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string]string{
		"literal brackets":            file,
		"directory":                   directory,
		"double quotes":               `"` + file + `"`,
		"single quotes":               `'` + file + `'`,
		"wildcard bracket parent":     filepath.Join(directory, "*.log"),
		"wildcard includes directory": filepath.Join(directory, "*"),
	} {
		t.Run(name, func(t *testing.T) {
			paths, err := ExpandPaths([]string{input})
			if err != nil || len(paths) != 1 || paths[0] != file {
				t.Fatalf("got %v, %v; want %s", paths, err, file)
			}
		})
	}
}
