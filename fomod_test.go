package fomod

import (
	"encoding/binary"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"unicode/utf16"
	"unicode/utf8"
)

func loadFixture(t *testing.T, name string) Config {
	t.Helper()
	b, err := fs.ReadFile(os.DirFS("testdata"), name)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestChooseOne(t *testing.T) {
	cfg := loadFixture(t, "choose-one.xml")
	ops := Resolve(cfg, Choices{"Options": {"Pack": {"Beta"}}}, EvalContext{})
	if len(ops) != 1 || ops[0].Source != "beta/manifest.json" {
		t.Fatalf("ops = %+v", ops)
	}
	if Match(cfg, Choices{"Options": {"Pack": {"Nope"}}}, EvalContext{}) {
		t.Fatal("vanished choice matched")
	}
	if !Match(cfg, Choices{"Options": {"Pack": {"Alpha"}}}, EvalContext{}) {
		t.Fatal("alpha should match")
	}
}

func TestFlagsConditionalInstall(t *testing.T) {
	cfg := loadFixture(t, "flags.xml")
	on := Resolve(cfg, Choices{"Pick": {"Mode": {"On"}}}, EvalContext{})
	if len(on) != 1 || on[0].Source != "extra.txt" {
		t.Fatalf("flag on: %+v", on)
	}
	off := Resolve(cfg, Choices{"Pick": {"Mode": {"Off"}}}, EvalContext{})
	if len(off) != 0 {
		t.Fatalf("flag off: %+v", off)
	}
}

func TestRequiredFiles(t *testing.T) {
	cfg := loadFixture(t, "required.xml")
	none := Resolve(cfg, Choices{"More": {"Extra": nil}}, EvalContext{})
	if len(none) != 1 || none[0].Source != "core.txt" {
		t.Fatalf("required only: %+v", none)
	}
	both := Resolve(cfg, Choices{"More": {"Extra": {"Addon"}}}, EvalContext{})
	if len(both) != 2 {
		t.Fatalf("with addon: %+v", both)
	}
}

func TestRequiredPluginIsResolvedWithoutChoice(t *testing.T) {
	cfg, err := Parse([]byte(`<config><installSteps><installStep name="S"><optionalFileGroups>
		<group name="G" type="SelectExactlyOne"><plugins>
			<plugin name="Required"><files><file source="required.txt" destination="required.txt"/></files>
				<typeDescriptor><type name="Required"/></typeDescriptor></plugin>
			<plugin name="Other"><files><file source="other.txt" destination="other.txt"/></files>
				<typeDescriptor><type name="Optional"/></typeDescriptor></plugin>
		</plugins></group></optionalFileGroups></installStep></installSteps></config>`))
	if err != nil {
		t.Fatal(err)
	}
	ops := Resolve(cfg, nil, EvalContext{})
	if len(ops) != 1 || ops[0].Source != "required.txt" {
		t.Fatalf("required plugin = %+v", ops)
	}
	if !Match(cfg, nil, EvalContext{}) {
		t.Fatal("required-only group should match")
	}
}

func TestUnchosenPluginsInstallTheirAlwaysAndUsableFiles(t *testing.T) {
	cfg, err := Parse([]byte(`<config><installSteps><installStep name="S"><optionalFileGroups>
		<group name="G" type="SelectAny"><plugins order="Explicit">
			<plugin name="Usable"><files><file source="always.txt" alwaysInstall="true"/><file source="usable.txt" installIfUsable="1"/>
				<file source="chosen.txt"/></files><typeDescriptor><type name="Optional"/></typeDescriptor></plugin>
			<plugin name="Unusable"><files><file source="always2.txt" alwaysInstall="true"/><file source="usable2.txt" installIfUsable="true"/></files>
				<typeDescriptor><type name="NotUsable"/></typeDescriptor></plugin>
		</plugins></group></optionalFileGroups></installStep></installSteps></config>`))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, op := range Resolve(cfg, Choices{"S": {"G": nil}}, EvalContext{}) {
		got = append(got, op.Source)
	}
	if strings.Join(got, ",") != "always.txt,usable.txt,always2.txt" {
		t.Fatalf("ops = %v", got)
	}
}

func TestOpenFindsWrappedUTF16Config(t *testing.T) {
	raw := append([]byte{0xFF, 0xFE}, utf16Bytes(`<?xml version="1.0"?><config><moduleName>Wide</moduleName></config>`, binary.LittleEndian)...)
	fsys := fstest.MapFS{"inner/fomod/ModuleConfig.xml": {Data: raw}}
	cfg, path, ok, err := Open(fsys)
	if err != nil || !ok || path != "inner/fomod/ModuleConfig.xml" || cfg.ModuleName != "Wide" {
		t.Fatalf("Open = %+v %q %v %v", cfg, path, ok, err)
	}
}

func TestFileDependencyUnmetWithoutIndex(t *testing.T) {
	cfg, err := Parse([]byte(`<config><moduleName>D</moduleName>
		<installSteps><installStep name="S"><visible>
			<fileDependency file="other.dll" state="Active"/>
		</visible><optionalFileGroups><group name="G" type="SelectAny">
			<plugins><plugin name="P"><files><file source="p.txt" destination="p.txt"/></files>
			<typeDescriptor><type name="Optional"/></typeDescriptor></plugin></plugins>
		</group></optionalFileGroups></installStep></installSteps></config>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(VisibleSteps(cfg, nil, EvalContext{})) != 0 {
		t.Fatal("missing file should hide the step")
	}
	files := FileIndex(func(name string) string {
		if name == "other.dll" {
			return FileActive
		}
		return FileMissing
	})
	if len(VisibleSteps(cfg, nil, EvalContext{Files: files})) != 1 {
		t.Fatal("present file should show the step")
	}
}

func TestGameDependencyUnknownVersionUnmet(t *testing.T) {
	cfg, err := Parse([]byte(`<config><moduleName>D</moduleName>
		<installSteps><installStep name="S"><visible>
			<gameDependency version="1.6.0"/>
		</visible><optionalFileGroups><group name="G" type="SelectAny">
			<plugins><plugin name="P"><files><file source="p.txt" destination="p.txt"/></files>
			<typeDescriptor><type name="Optional"/></typeDescriptor></plugin></plugins>
		</group></optionalFileGroups></installStep></installSteps></config>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(VisibleSteps(cfg, nil, EvalContext{})) != 0 {
		t.Fatal("unknown game version should hide the step")
	}
}

func TestGameDependencyMetWhenInstalledVersionSufficient(t *testing.T) {
	cfg, err := Parse([]byte(`<config><moduleName>D</moduleName>
		<installSteps><installStep name="S"><visible>
			<gameDependency version="1.6.0"/>
		</visible><optionalFileGroups><group name="G" type="SelectAny">
			<plugins><plugin name="P"><files><file source="p.txt" destination="p.txt"/></files>
			<typeDescriptor><type name="Optional"/></typeDescriptor></plugin></plugins>
		</group></optionalFileGroups></installStep></installSteps></config>`))
	if err != nil {
		t.Fatal(err)
	}
	ctx := EvalContext{GameVersion: "1.6.15"}
	if len(VisibleSteps(cfg, nil, ctx)) != 1 {
		t.Fatal("installed game version should satisfy minimum")
	}
	if len(VisibleSteps(cfg, nil, EvalContext{GameVersion: "1.5.0"})) != 0 {
		t.Fatal("older game version should not satisfy minimum")
	}
}

func TestResolveSortsExtremePriorities(t *testing.T) {
	cfg := Config{Required: []Op{{Source: "high", Priority: math.MaxInt}, {Source: "low", Priority: -1}}}
	ops := Resolve(cfg, nil, EvalContext{})
	if ops[0].Source != "low" || ops[1].Source != "high" {
		t.Fatalf("ops = %+v", ops)
	}
}

func TestFolderElementSetsFolder(t *testing.T) {
	cfg := loadFixture(t, "folder.xml")
	if len(cfg.Required) != 1 || !cfg.Required[0].Folder || cfg.Required[0].Source != "pack" {
		t.Fatalf("required = %+v", cfg.Required)
	}
}

func TestRequiredPluginNotUsableUnderFlagsMatches(t *testing.T) {
	cfg, err := Parse([]byte(`<config><installSteps>
		<installStep name="A"><optionalFileGroups><group name="G" type="SelectExactlyOne"><plugins>
			<plugin name="On"><conditionFlags><flag name="f">1</flag></conditionFlags>
				<typeDescriptor><type name="Optional"/></typeDescriptor></plugin>
		</plugins></group></optionalFileGroups></installStep>
		<installStep name="B"><optionalFileGroups><group name="H" type="SelectAny"><plugins>
			<plugin name="R"><files><file source="r.txt"/></files><typeDescriptor><dependencyType>
				<defaultType name="Required"/><patterns><pattern><dependencies><flagDependency flag="f" value="1"/></dependencies>
				<type name="NotUsable"/></pattern></patterns></dependencyType></typeDescriptor></plugin>
		</plugins></group></optionalFileGroups></installStep></installSteps></config>`))
	if err != nil {
		t.Fatal(err)
	}
	choices := Choices{"A": {"G": {"On"}}}
	if !Match(cfg, choices, EvalContext{}) {
		t.Fatal("a plugin NotUsable under the flags must not be forced in, so the choices still match")
	}
	for _, op := range Resolve(cfg, choices, EvalContext{}) {
		if op.Source == "r.txt" {
			t.Fatal("NotUsable plugin installed")
		}
	}
}

func TestLargeGroupIsLinear(t *testing.T) {
	perOp := func(n int) float64 {
		g := Group{Name: "G", Type: SelectAny}
		names := make([]string, n)
		for i := range n {
			names[i] = "p" + strconv.Itoa(i)
			g.Plugins = append(g.Plugins, Plugin{Name: names[i], TypeName: TypeOptional, Files: []Op{{Source: names[i]}}})
		}
		cfg := Config{Steps: []Step{{Name: "S", Groups: []Group{g}}}}
		choices := Choices{"S": {"G": names}}
		if !Match(cfg, choices, EvalContext{}) {
			t.Fatal("no match")
		}
		if got := len(Resolve(cfg, choices, EvalContext{})); got != n {
			t.Fatalf("ops = %d", got)
		}
		r := testing.Benchmark(func(b *testing.B) {
			for b.Loop() {
				Match(cfg, choices, EvalContext{})
				Resolve(cfg, choices, EvalContext{})
			}
		})
		return float64(r.T) / float64(r.N)
	}
	// A ratio rather than a wall-clock limit, so a loaded runner passes: 8x the plugins costs about 8x when linear
	// and 64x when quadratic. testing.Benchmark collects garbage before timing and averages over many runs, so the
	// setup's garbage and one stall on a busy runner do not count against the larger size.
	small, large := perOp(5000), perOp(40000)
	if ratio := large / small; ratio > 24 {
		t.Fatalf("8x the plugins took %.1fx as long (%.0fns vs %.0fns per run)", ratio, large, small)
	}
}

func utf16Bytes(s string, order binary.AppendByteOrder) []byte {
	var raw []byte
	for _, u := range utf16.Encode([]rune(s)) {
		raw = order.AppendUint16(raw, u)
	}
	return raw
}

func TestDecodeXMLBOMlessUTF16(t *testing.T) {
	body := `<config><moduleName>Wide</moduleName></config>`
	for _, doc := range []string{`<?xml version="1.0" encoding="utf-16"?>` + body, body, "\n  <?xml version=\"1.0\"?>" + body} {
		for name, order := range map[string]binary.AppendByteOrder{"LE": binary.LittleEndian, "BE": binary.BigEndian} {
			cfg, err := Parse(utf16Bytes(doc, order))
			if err != nil || cfg.ModuleName != "Wide" {
				t.Fatalf("%s %q: %+v %v", name, doc, cfg, err)
			}
		}
	}
}

func TestParseDeclaredAndUndeclaredLegacyEncodings(t *testing.T) {
	for _, decl := range []string{`encoding="utf-16"`, `encoding="UTF-16"`} {
		cfg, err := Parse([]byte(`<?xml version="1.0" ` + decl + `?><config><moduleName>Caf` + "é" + `</moduleName></config>`))
		if err != nil || cfg.ModuleName != "Café" {
			t.Fatalf("%s: %+v %v", decl, cfg, err)
		}
	}
	for _, decl := range []string{`<?xml version="1.0" encoding="windows-1252"?>`, `<?xml version="1.0" encoding="ISO-8859-1"?>`, ""} {
		cfg, err := Parse([]byte(decl + "<config><moduleName>Caf\xe9</moduleName></config>"))
		if err != nil || cfg.ModuleName != "Café" {
			t.Fatalf("%q: %+v %v", decl, cfg, err)
		}
	}
}

func TestParseDecidesEncodingByBytes(t *testing.T) {
	for _, c := range []struct{ doc, want string }{
		{"<?xml version=\"1.0\" encoding=\"windows-1252\"?><config><moduleName>W\u00efde</moduleName></config>", "W\u00efde"},
		{"<?xml version=\"1.0\" encoding=\"utf-8\"?><config><moduleName>Caf\xe9</moduleName></config>", "Caf\u00e9"},
		{"<?xml version=\"1.0\" encoding=\"us-ascii\"?><config><moduleName>Plain</moduleName></config>", "Plain"},
		{"<?xml version=\"1.0\" encoding=\"windows-1252\"?><config><moduleName>Caf\xe9</moduleName></config>", "Caf\u00e9"},
	} {
		cfg, err := Parse([]byte(c.doc))
		if err != nil || cfg.ModuleName != c.want {
			t.Fatalf("%q: %+v %v", c.doc, cfg, err)
		}
	}
}

func TestOpenRefusesOversizedConfig(t *testing.T) {
	fsys := fstest.MapFS{"fomod/ModuleConfig.xml": {Data: make([]byte, maxConfigBytes+1)}}
	if _, _, _, err := Open(fsys); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v", err)
	}
	fsys["fomod/ModuleConfig.xml"] = &fstest.MapFile{Data: append([]byte("<config/>"), make([]byte, maxConfigBytes-9)...)}
	if _, _, ok, err := Open(fsys); err != nil || !ok {
		t.Fatalf("config at the cap: ok=%v err=%v", ok, err)
	}
}

func orderConfig(t *testing.T, steps, groups, plugins string) Config {
	t.Helper()
	attr := func(o string) string {
		if o == "" {
			return ""
		}
		return ` order="` + o + `"`
	}
	plug := func(n string) string {
		return `<plugin name="` + n + `"><files><file source="` + n + `" priority="1"/></files>` +
			`<typeDescriptor><type name="Optional"/></typeDescriptor></plugin>`
	}
	grp := func(n string) string {
		return `<group name="` + n + `" type="SelectAny"><plugins` + attr(plugins) + `>` + plug("b") + plug("C") + plug("a") + `</plugins></group>`
	}
	step := func(n string) string {
		return `<installStep name="` + n + `"><optionalFileGroups` + attr(groups) + `>` + grp("y") + grp("z") + grp("x") +
			`</optionalFileGroups></installStep>`
	}
	cfg, err := Parse([]byte(`<config><installSteps` + attr(steps) + `>` + step("2") + step("3") + step("1") + `</installSteps></config>`))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestOrderAttribute(t *testing.T) {
	for _, c := range []struct {
		order, wantOrder string
	}{
		{"", OrderAscending},
		{"Ascending", OrderAscending},
		{"Descending", OrderDescending},
		{"Explicit", OrderExplicit},
	} {
		// Capitals sort before lowercase, as in MO2.
		want := map[string]string{"": "Cab", "Ascending": "Cab", "Descending": "baC", "Explicit": "bCa"}[c.order]
		t.Run(c.order, func(t *testing.T) {
			steps := orderConfig(t, c.order, "", "")
			if got := stepNames(steps); got != orderOf(c.order, "231", "123") {
				t.Fatalf("steps = %s", got)
			}
			if steps.StepOrder != c.wantOrder {
				t.Fatalf("StepOrder = %s", steps.StepOrder)
			}
			groups := orderConfig(t, "Explicit", c.order, "")
			if got := groupNames(groups.Steps[0]); got != orderOf(c.order, "yzx", "xyz") {
				t.Fatalf("groups = %s", got)
			}
			if groups.Steps[0].GroupOrder != c.wantOrder {
				t.Fatalf("GroupOrder = %s", groups.Steps[0].GroupOrder)
			}
			plugins := orderConfig(t, "Explicit", "Explicit", c.order)
			g := plugins.Steps[0].Groups[0]
			var got string
			for _, p := range g.Plugins {
				got += p.Name
			}
			if got != want {
				t.Fatalf("plugins = %s, want %s", got, want)
			}
			if g.PluginOrder != c.wantOrder {
				t.Fatalf("PluginOrder = %s", g.PluginOrder)
			}
		})
	}
}

// orderOf is the document-order result for Explicit, the ascending one for Ascending or unset, else descending.
func orderOf(order, doc, asc string) string {
	switch order {
	case "Explicit":
		return doc
	case "Descending":
		b := []byte(asc)
		slices.Reverse(b)
		return string(b)
	default:
		return asc
	}
}

func stepNames(c Config) string {
	var s string
	for _, st := range c.Steps {
		s += st.Name
	}
	return s
}

func groupNames(st Step) string {
	var s string
	for _, g := range st.Groups {
		s += g.Name
	}
	return s
}

func TestOrderDecidesResolveTie(t *testing.T) {
	cfg, err := Parse([]byte(`<config><installSteps><installStep name="S"><optionalFileGroups>` +
		`<group name="G" type="SelectAll"><plugins>` +
		`<plugin name="b"><files><file source="b" destination="x" priority="1"/></files></plugin>` +
		`<plugin name="a"><files><file source="a" destination="x" priority="1"/></files></plugin>` +
		`</plugins></group></optionalFileGroups></installStep></installSteps></config>`))
	if err != nil {
		t.Fatal(err)
	}
	ops := Resolve(cfg, Choices{}, EvalContext{})
	if len(ops) != 2 || ops[0].Source != "a" || ops[1].Source != "b" {
		t.Fatalf("ops = %+v", ops)
	}
}

func TestFindPrefersTheExactNameAmongCaseVariants(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"FOMOD/ModuleConfig.xml", "fomod/moduleconfig.xml"} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("<config/>"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if entries, _ := os.ReadDir(root); len(entries) < 2 {
		t.Skip("this file system folds case, so it cannot hold both spellings")
	}
	got, err := Find(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	if want := "fomod/moduleconfig.xml"; got != want {
		t.Fatalf("Find = %q, want %q", got, want)
	}
}

func TestFindDepth(t *testing.T) {
	x := &fstest.MapFile{Data: []byte("<config/>")}
	for _, c := range []struct {
		name string
		fsys fstest.MapFS
		want string
	}{
		{"root", fstest.MapFS{"fomod/ModuleConfig.xml": x}, "fomod/ModuleConfig.xml"},
		{"root, folded", fstest.MapFS{"FoMoD/MODULECONFIG.XML": x}, "FoMoD/MODULECONFIG.XML"},
		{"wrapper", fstest.MapFS{"My Mod/fomod/ModuleConfig.xml": x}, "My Mod/fomod/ModuleConfig.xml"},
		{"root wins over wrapper", fstest.MapFS{"fomod/ModuleConfig.xml": x, "a/fomod/ModuleConfig.xml": x}, "fomod/ModuleConfig.xml"},
		{"two folders down", fstest.MapFS{"a/b/fomod/ModuleConfig.xml": x}, ""},
		{"fomod folder without a config", fstest.MapFS{"fomod/info.xml": x}, ""},
		{"empty", fstest.MapFS{}, ""},
	} {
		got, err := Find(c.fsys)
		if err != nil || got != c.want {
			t.Errorf("%s: Find = %q %v, want %q", c.name, got, err, c.want)
		}
	}
	if _, _, ok, err := Open(fstest.MapFS{}); ok || err != nil {
		t.Fatalf("Open of an empty tree: ok=%v err=%v", ok, err)
	}
}

func TestWindows1252Table(t *testing.T) {
	want := map[byte]rune{
		0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„', 0x85: '…', 0x86: '†', 0x87: '‡',
		0x88: 'ˆ', 0x89: '‰', 0x8A: 'Š', 0x8B: '‹', 0x8C: 'Œ', 0x8E: 'Ž', 0x91: '‘',
		0x92: '’', 0x93: '“', 0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—', 0x98: '˜',
		0x99: '™', 0x9A: 'š', 0x9B: '›', 0x9C: 'œ', 0x9E: 'ž', 0x9F: 'Ÿ',
		0x81: '�', 0x8D: '�', 0x8F: '�', 0x90: '�', 0x9D: '�',
	}
	for b := byte(0x80); b <= 0x9F; b++ {
		if got, _ := utf8.DecodeRune(decodeLegacy([]byte{b})); got != want[b] {
			t.Errorf("%#x = %U, want %U", b, got, want[b])
		}
	}
	for b := 0xA0; b <= 0xFF; b++ {
		if got, _ := utf8.DecodeRune(decodeLegacy([]byte{byte(b)})); got != rune(b) {
			t.Errorf("%#x = %U", b, got)
		}
	}
}

func TestParseRefusesEntities(t *testing.T) {
	for name, doc := range map[string]string{
		"billion laughs": `<?xml version="1.0"?><!DOCTYPE config [<!ENTITY lol "lol"><!ENTITY lol2 "&lol;&lol;">]><config><moduleName>&lol2;</moduleName></config>`,
		"external":       `<?xml version="1.0"?><!DOCTYPE config [<!ENTITY x SYSTEM "file:///etc/passwd">]><config><moduleName>&x;</moduleName></config>`,
	} {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
	if cfg, err := Parse([]byte(`<!DOCTYPE config [<!ENTITY unused "x">]><config><moduleName>M</moduleName></config>`)); err != nil || cfg.ModuleName != "M" {
		t.Fatalf("unused DTD: %+v %v", cfg, err)
	}
}

func TestParseBoundsNesting(t *testing.T) {
	nest := func(n int) []byte {
		return []byte("<config><installSteps><installStep name=\"S\"><visible>" + strings.Repeat("<dependencies>", n) +
			strings.Repeat("</dependencies>", n) + "</visible></installStep></installSteps></config>")
	}
	cfg, err := Parse(nest(9000))
	if err != nil {
		t.Fatal(err)
	}
	VisibleSteps(cfg, nil, EvalContext{})
	if _, err := Parse(nest(10010)); err == nil {
		t.Fatal("a 10,010-deep nest parsed")
	}
}

func TestGameDepComparison(t *testing.T) {
	for _, c := range []struct {
		installed, minimum string
		want               bool
	}{
		{"1.6.15", "1.6.0", true},
		{"1.6.15.0", "1.6.15.0", true},
		{"1.6.15", "1.6.15.0", true},
		{"1.5.0.0", "1.6.0", false},
		{"1.6.15-beta", "9.0", false},
		{"1.6.15", "v1.7.0", false},
		{"not a version", "1.0", true},
		{"", "1.0", false},
		{"1.0", "", true},
	} {
		if got := gameDepOK(c.installed, c.minimum); got != c.want {
			t.Errorf("gameDepOK(%q, %q) = %v, want %v", c.installed, c.minimum, got, c.want)
		}
	}
}

func TestKeepIsIdempotentForRepeatedNames(t *testing.T) {
	step := `<installStep name="S"><optionalFileGroups><group name="G" type="SelectAny"><plugins>` +
		`<plugin name="a"/></plugins></group></optionalFileGroups></installStep>`
	cfg, err := Parse([]byte(`<config><installSteps>` + step + step + `</installSteps></config>`))
	if err != nil {
		t.Fatal(err)
	}
	once := Keep(cfg, Choices{"S": {"G": {"a"}}})
	if twice := Keep(cfg, once); !reflect.DeepEqual(once, twice) || len(once["S"]["G"]) != 1 {
		t.Fatalf("Keep = %v then %v", once, twice)
	}
}
