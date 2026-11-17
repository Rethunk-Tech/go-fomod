package fomod

import (
	"encoding/binary"
	"errors"
	"io/fs"
	"os"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"unicode/utf16"
)

func FuzzParse(f *testing.F) {
	seedModuleConfig(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		xml := decodeXML(data)
		if len(xml) > 2*len(data)+16 {
			t.Fatalf("decodeXML grew %d to %d", len(data), len(xml))
		}
		legacy := decodeLegacy(xml)
		if len(legacy) > 3*len(xml)+16 {
			t.Fatalf("decodeLegacy grew %d to %d", len(xml), len(legacy))
		}
		cfg, err := Parse(data)
		if err != nil {
			return
		}
		assertParsed(t, cfg)
	})
}

func FuzzOpen(f *testing.F) {
	seedModuleConfig(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		fsys := fstest.MapFS{
			"fomod/ModuleConfig.xml": &fstest.MapFile{Data: data},
		}
		cfg, _, ok, err := Open(fsys)
		if len(data) > maxConfigBytes {
			if !errors.Is(err, ErrTooLarge) {
				t.Fatalf("Open: %v, want ErrTooLarge", err)
			}
			return
		}
		if err != nil || !ok {
			return
		}
		assertParsed(t, cfg)
	})
}

func FuzzFind(f *testing.F) {
	for _, c := range [][2]string{
		{"fomod/ModuleConfig.xml", ""},
		{"FoMoD/MODULECONFIG.XML", ""},
		{"My Mod/fomod/ModuleConfig.xml", ""},
		{"a/b/fomod/ModuleConfig.xml", ""},
		{"fomod/info.xml", ""},
		{"../fomod/ModuleConfig.xml", "fomod/ModuleConfig.xml"},
		{"a/fomod/ModuleConfig.xml", "fomod/../secret.xml"},
	} {
		f.Add(c[0], c[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		fsys := fstest.MapFS{}
		put := func(name string) {
			name = strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/")
			if name == "" || !fs.ValidPath(name) {
				return
			}
			fsys[name] = &fstest.MapFile{Data: []byte("<config/>")}
		}
		put(a)
		put(b)
		got, err := Find(fsys)
		if err != nil || got == "" {
			return
		}
		if !fs.ValidPath(got) || path.IsAbs(got) || slices.Contains(strings.Split(got, "/"), "..") {
			t.Fatalf("Find escaped: %q", got)
		}
		if !strings.EqualFold(path.Base(got), "moduleconfig.xml") {
			t.Fatalf("Find = %q, not a ModuleConfig.xml", got)
		}
		dir := path.Dir(got)
		if !strings.EqualFold(path.Base(dir), "fomod") {
			t.Fatalf("Find = %q, not under fomod/", got)
		}
		if rest := path.Dir(dir); rest != "." && path.Dir(rest) != "." {
			t.Fatalf("Find went more than one folder down: %q", got)
		}
		if _, err := fs.Stat(fsys, got); err != nil {
			t.Fatalf("Find = %q, missing: %v", got, err)
		}
	})
}

func FuzzGameDep(f *testing.F) {
	for _, v := range []string{"1.6.15", "1.6.0", "1.6.15.0", "1.5.0.0", "1.6.15-beta", "v1.7.0", "not a version", "", "9.0"} {
		f.Add(v, "1.0")
		f.Add("1.0", v)
		f.Add(v, v)
	}
	f.Fuzz(func(t *testing.T, installed, minimum string) {
		got := gameDepOK(installed, minimum)
		switch {
		case minimum == "" && !got:
			t.Fatalf("gameDepOK(%q, %q) = false, empty minimum is met", installed, minimum)
		case installed == "" && minimum != "" && got:
			t.Fatalf("gameDepOK(%q, %q) = true, unknown installed is unmet", installed, minimum)
		}
	})
}

func FuzzOpEscapes(f *testing.F) {
	f.Add("a/b.esp", "", false)
	f.Add("a..b/c", "", false)
	f.Add("../x", "", false)
	f.Add(`a\..\..\x`, "", false)
	f.Add("/etc/passwd", "", false)
	f.Add(`C:\x`, "", false)
	f.Add("ok", "../../x", true)
	f.Add("ok", "", true)
	f.Fuzz(func(t *testing.T, source, dest string, hasDest bool) {
		op := Op{Source: source}
		if hasDest {
			op.Destination = &dest
		}
		cfg := Config{Required: []Op{op, {Source: "safe.esp"}}}
		ops, skipped := ResolveSafe(cfg, nil, EvalContext{})
		for _, o := range ops {
			if o.Escapes() {
				t.Fatalf("ResolveSafe kept %+v", o)
			}
		}
		for _, o := range skipped {
			if !o.Escapes() {
				t.Fatalf("ResolveSafe skipped %+v", o)
			}
		}
		if n := len(Resolve(cfg, nil, EvalContext{})); len(ops)+len(skipped) != n {
			t.Fatalf("ResolveSafe %d+%d != Resolve %d", len(ops), len(skipped), n)
		}
	})
}

func seedModuleConfig(f *testing.F) {
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
	f.Add([]byte(`<config><requiredInstallFiles><file source="../x"/><file source="ok.esp"/></requiredInstallFiles></config>`))
}

func assertParsed(t *testing.T, cfg Config) {
	t.Helper()
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
			safe, skipped := ResolveSafe(cfg, ch, ctx)
			if len(safe)+len(skipped) != len(ops) {
				t.Fatalf("ResolveSafe %d+%d != Resolve %d", len(safe), len(skipped), len(ops))
			}
			for _, op := range safe {
				if op.Escapes() {
					t.Fatalf("ResolveSafe kept %+v", op)
				}
			}
			kept := Keep(cfg, ch)
			if again := Keep(cfg, kept); !reflect.DeepEqual(kept, again) {
				t.Fatalf("Keep is not idempotent: %v then %v", kept, again)
			}
		}
	}
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
