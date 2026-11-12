package fomod

import (
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"
	"testing/fstest"
)

func FuzzParse(f *testing.F) {
	seedParse(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxConfigBytes {
			t.Skip()
		}
		cfg, err := Parse(data)
		if err != nil {
			return
		}
		exercise(cfg)
	})
}

func FuzzOpen(f *testing.F) {
	seedParse(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxConfigBytes+1 {
			t.Skip()
		}
		fsys := fstest.MapFS{
			"fomod/ModuleConfig.xml": &fstest.MapFile{Data: data},
		}
		cfg, _, ok, err := Open(fsys)
		if err != nil || !ok {
			return
		}
		exercise(cfg)
	})
}

func seedParse(f *testing.F) {
	f.Add([]byte(`<config><moduleName>x</moduleName></config>`))
	f.Add([]byte("\xFF\xFE<\x00c\x00o\x00n\x00f\x00i\x00g\x00>\x00<\x00/\x00c\x00o\x00n\x00f\x00i\x00g\x00>\x00"))
	_ = fs.WalkDir(osTestdata(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.EqualFold(path.Ext(p), ".xml") {
			return nil
		}
		b, err := fs.ReadFile(os.DirFS("testdata"), p)
		if err != nil || len(b) == 0 || len(b) > maxConfigBytes {
			return nil
		}
		f.Add(b)
		return nil
	})
}

func osTestdata() fs.FS { return os.DirFS("testdata") }

func exercise(cfg Config) {
	ctx := EvalContext{}
	ch := Choices{}
	flags := FlagsFrom(cfg, ch, ctx)
	_ = VisibleSteps(cfg, flags, ctx)
	_ = Match(cfg, ch, ctx)
	_ = Resolve(cfg, ch, ctx)
	_ = Unanswered(Config{}, cfg, ch, ctx)
	_ = Keep(cfg, ch)
	for _, st := range cfg.Steps {
		_ = st.Visible.Eval(flags, ctx)
		for _, g := range st.Groups {
			for _, p := range g.Plugins {
				_ = PluginType(p, flags, ctx)
			}
		}
	}
}
