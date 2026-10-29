package fomod

import (
	"encoding/binary"
	"io/fs"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

func FuzzParse(f *testing.F) {
	for _, name := range []string{"choose-one.xml", "flags.xml", "required.xml", "folder.xml"} {
		b, err := fs.ReadFile(os.DirFS("testdata"), name)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
		for _, order := range []binary.AppendByteOrder{binary.LittleEndian, binary.BigEndian} {
			wide := utf16Bytes(string(b), order)
			f.Add(wide)
			bom := []byte{0xFF, 0xFE}
			if order == binary.BigEndian {
				bom = []byte{0xFE, 0xFF}
			}
			f.Add(append(bom, wide...))
			f.Add(utf16Bytes(strings.TrimPrefix(string(b), `<?xml version="1.0" encoding="UTF-8"?>`), order))
		}
	}
	f.Add([]byte(`<?xml version="1.0"?><!DOCTYPE c [<!ENTITY a "x"><!ENTITY b "&a;&a;">]><config><moduleName>&b;</moduleName></config>`))
	f.Add([]byte(`<!DOCTYPE c [<!ENTITY x SYSTEM "file:///etc/passwd">]><config><moduleName>&x;</moduleName></config>`))
	f.Add([]byte("<config><installSteps><installStep name=\"S\"><visible>" + strings.Repeat("<dependencies>", 10001) +
		strings.Repeat("</dependencies>", 10001) + "</visible></installStep></installSteps></config>"))
	f.Add([]byte("<config><moduleName>Caf\xe9\x93</moduleName></config>"))

	f.Fuzz(func(t *testing.T, data []byte) {
		cfg, err := Parse(data)
		if err != nil {
			return
		}
		checkOrder(t, cfg)
		ctxs := []EvalContext{{}, {GameVersion: "1.6.15", Files: func(string) string { return FileActive }}}
		for _, ctx := range ctxs {
			for _, ch := range choicesFor(cfg) {
				flags := FlagsFrom(cfg, ch, ctx)
				VisibleSteps(cfg, flags, ctx)
				Match(cfg, ch, ctx)
				Unanswered(cfg, cfg, ch, ctx)
				Unanswered(withoutLastGroup(cfg), cfg, ch, ctx)
				ops := Resolve(cfg, ch, ctx)
				if !slices.IsSortedFunc(ops, func(a, b Op) int { return cmpInt(a.Priority, b.Priority) }) {
					t.Fatalf("Resolve is not sorted by priority: %+v", ops)
				}
				kept := Keep(cfg, ch)
				if again := Keep(cfg, kept); !reflect.DeepEqual(kept, again) {
					t.Fatalf("Keep is not idempotent: %v then %v", kept, again)
				}
			}
		}
	})
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// choicesFor derives the answers a fuzz input is replayed with: none, and each group's first plugin.
func choicesFor(cfg Config) []Choices {
	first := Choices{}
	for _, s := range cfg.Steps {
		for _, g := range s.Groups {
			if len(g.Plugins) == 0 {
				continue
			}
			if first[s.Name] == nil {
				first[s.Name] = map[string][]string{}
			}
			first[s.Name][g.Name] = []string{g.Plugins[0].Name}
		}
	}
	return []Choices{nil, first}
}

func checkOrder(t *testing.T, cfg Config) {
	t.Helper()
	check := func(what, order string, names []string) {
		key := func(s string) []uint16 { return utf16.Encode([]rune(s)) }
		less := func(a, b string) int { return slices.Compare(key(a), key(b)) }
		switch order {
		case OrderAscending:
			if !slices.IsSortedFunc(names, less) {
				t.Fatalf("%s not ascending: %q", what, names)
			}
		case OrderDescending:
			if !slices.IsSortedFunc(names, func(a, b string) int { return less(b, a) }) {
				t.Fatalf("%s not descending: %q", what, names)
			}
		case OrderExplicit:
		default:
			t.Fatalf("%s has order %q", what, order)
		}
	}
	var steps []string
	for _, s := range cfg.Steps {
		steps = append(steps, s.Name)
		var groups []string
		for _, g := range s.Groups {
			groups = append(groups, g.Name)
			var plugins []string
			for _, p := range g.Plugins {
				plugins = append(plugins, p.Name)
			}
			check("plugins", g.PluginOrder, plugins)
		}
		check("groups", s.GroupOrder, groups)
	}
	check("steps", cfg.StepOrder, steps)
}

type byteReader struct{ data []byte }

func (r *byteReader) next() byte {
	if len(r.data) == 0 {
		return 0
	}
	b := r.data[0]
	r.data = r.data[1:]
	return b
}

var evalFlags = map[string]string{"f0": "1", "f1": "2"}

func evalFiles(name string) string {
	switch name {
	case "a":
		return FileActive
	case "b":
		return FileInactive
	}
	return FileMissing
}

// buildComposite reads a dependency tree from r and returns it with the value the definition gives it: And holds iff
// every child does, Or iff some child does, an empty And holds and an empty Or does not.
func buildComposite(r *byteReader, depth int) (Composite, bool) {
	c := Composite{Operator: "And"}
	or := r.next()&1 == 1
	if or {
		c.Operator = "or"
	}
	var vals []bool
	for range int(r.next() % 5) {
		switch r.next() % 4 {
		case 0:
			d := FlagDep{Flag: "f" + string('0'+r.next()%3), Value: string('0' + r.next()%3)}
			c.Flags = append(c.Flags, d)
			vals = append(vals, evalFlags[d.Flag] == d.Value)
		case 1:
			d := FileDep{File: string('a' + r.next()%3), State: []string{FileActive, FileInactive, FileMissing}[r.next()%3]}
			c.Files = append(c.Files, d)
			vals = append(vals, evalFiles(d.File) == d.State)
		case 2:
			n := r.next() % 3
			c.Games = append(c.Games, GameDep{Version: []string{"0.5", "1.0", "2.0"}[n]})
			vals = append(vals, n < 2)
		default:
			if depth < 6 {
				n, v := buildComposite(r, depth+1)
				c.Nested = append(c.Nested, n)
				vals = append(vals, v)
			}
		}
	}
	if or {
		return c, slices.Contains(vals, true)
	}
	return c, !slices.Contains(vals, false)
}

func FuzzEval(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 3, 0, 0, 1, 1, 0, 0, 2, 1})
	f.Add([]byte{1, 4, 3, 0, 1, 2, 2, 2, 0, 3, 1, 3, 0, 0, 1})
	f.Fuzz(func(t *testing.T, data []byte) {
		c, want := buildComposite(&byteReader{data}, 0)
		ctx := EvalContext{Files: evalFiles, GameVersion: "1.0"}
		if got := c.Eval(evalFlags, ctx); got != want {
			t.Fatalf("Eval = %v, want %v for %+v", got, want, c)
		}
	})
}
