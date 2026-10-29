package fomod

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/Rethunk-Tech/go-safefs"
)

// ErrTooLarge is returned by Open for a config larger than the read cap.
var ErrTooLarge = errors.New("fomod: config exceeds the read cap")

const configPath = "fomod/moduleconfig.xml"

// maxConfigBytes bounds a ModuleConfig.xml read from an untrusted archive; real configs are orders of magnitude
// smaller. The decoded text of a UTF-16 file is at most 1.5 times its size, so the parser's input is bounded too.
const maxConfigBytes = 16 << 20

// Find returns the slash path, relative to fsys, of fomod/ModuleConfig.xml, or "" when there is none. The fomod folder
// may sit at the root or one folder down; names are matched case-insensitively, preferring an exact spelling.
func Find(fsys fs.FS) (string, error) {
	return findAtDepth(fsys, ".", 0)
}

func findAtDepth(fsys fs.FS, dir string, depth int) (string, error) {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		return "", fmt.Errorf("fomod: %w", err)
	}
	ents, err := fs.ReadDir(sub, ".")
	if err != nil {
		return "", fmt.Errorf("fomod: %w", err)
	}
	rel, err := safefs.ResolveFold(sub, configPath)
	if err == nil {
		return path.Join(dir, rel), nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("fomod: %w", err)
	}
	if depth >= 1 {
		return "", nil
	}
	for _, e := range ents {
		if !e.IsDir() || strings.EqualFold(e.Name(), "fomod") {
			continue
		}
		p, err := findAtDepth(fsys, path.Join(dir, e.Name()), depth+1)
		if err != nil {
			return "", err
		}
		if p != "" {
			return p, nil
		}
	}
	return "", nil
}

// Open finds and parses the config in fsys. ok is false when there is none; file is the path Find returned. A config
// over the read cap fails with ErrTooLarge.
func Open(fsys fs.FS) (cfg Config, file string, ok bool, err error) {
	file, err = Find(fsys)
	if err != nil || file == "" {
		return Config{}, "", false, err
	}
	f, err := fsys.Open(file)
	if err != nil {
		return Config{}, file, false, fmt.Errorf("fomod: %w", err)
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, maxConfigBytes+1))
	if err != nil {
		return Config{}, file, false, fmt.Errorf("fomod: %w", err)
	}
	if len(raw) > maxConfigBytes {
		return Config{}, file, false, ErrTooLarge
	}
	cfg, err = Parse(raw)
	if err != nil {
		return Config{}, file, false, err
	}
	return cfg, file, true, nil
}
