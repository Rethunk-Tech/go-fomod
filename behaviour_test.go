package fomod

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
)

// The behaviour table pins what every evaluator function returns for a set of configs, choices and contexts, so a
// refactor shows up as a named row. Regenerate testdata/behaviour.golden with
// FOMOD_RECORD=1 go test -run TestBehaviourTable only when a difference is intended.

const (
	plugin = `<plugin name="%s"><files><file source="%s.txt" destination="%s.txt"/></files>` +
		`<typeDescriptor><type name="%s"/></typeDescriptor></plugin>`
	flagStep = `<installStep name="A"><optionalFileGroups><group name="G" type="SelectExactlyOne"><plugins>` +
		`<plugin name="On"><conditionFlags><flag name="f">1</flag></conditionFlags>` +
		`<typeDescriptor><type name="Optional"/></typeDescriptor></plugin></plugins></group></optionalFileGroups></installStep>`
)

func gameDepConfig(minimum string) string {
	return `<config><installSteps><installStep name="S"><visible><gameDependency version="` + minimum + `"/></visible>` +
		`<optionalFileGroups><group name="G" type="SelectAny"><plugins>` + fmt.Sprintf(plugin, "P", "p", "p", "Optional") +
		`</plugins></group></optionalFileGroups></installStep></installSteps></config>`
}

func behaviourConfigs(t *testing.T) []struct{ name, xml string } {
	t.Helper()
	out := []struct{ name, xml string }{
		{"required-plugin", `<config><installSteps><installStep name="S"><optionalFileGroups><group name="G" type="SelectExactlyOne"><plugins>` +
			fmt.Sprintf(plugin, "Required", "required", "required", "Required") + fmt.Sprintf(plugin, "Other", "other", "other", "Optional") +
			`</plugins></group></optionalFileGroups></installStep></installSteps></config>`},
		{"unchosen-always-usable", `<config><installSteps><installStep name="S"><optionalFileGroups><group name="G" type="SelectAny"><plugins order="Explicit">` +
			`<plugin name="Usable"><files><file source="always.txt" alwaysInstall="true"/><file source="usable.txt" installIfUsable="1"/><file source="chosen.txt"/></files>` +
			`<typeDescriptor><type name="Optional"/></typeDescriptor></plugin>` +
			`<plugin name="Unusable"><files><file source="always2.txt" alwaysInstall="true"/><file source="usable2.txt" installIfUsable="true"/></files>` +
			`<typeDescriptor><type name="NotUsable"/></typeDescriptor></plugin></plugins></group></optionalFileGroups></installStep></installSteps></config>`},
		{"file-dependency", `<config><installSteps><installStep name="S"><visible><fileDependency file="other.dll" state="Active"/></visible>` +
			`<optionalFileGroups><group name="G" type="SelectAny"><plugins>` + fmt.Sprintf(plugin, "P", "p", "p", "Optional") +
			`</plugins></group></optionalFileGroups></installStep></installSteps></config>`},
		{"game-dependency", gameDepConfig("1.6.0")},
		{"game-four-part-minimum", gameDepConfig("1.6.15.0")},
		{"required-notusable-under-flag", `<config><installSteps>` + flagStep +
			`<installStep name="B"><optionalFileGroups><group name="H" type="SelectAny"><plugins>` +
			`<plugin name="R"><files><file source="r.txt"/></files><typeDescriptor><dependencyType><defaultType name="Required"/>` +
			`<patterns><pattern><dependencies><flagDependency flag="f" value="1"/></dependencies><type name="NotUsable"/></pattern></patterns>` +
			`</dependencyType></typeDescriptor></plugin></plugins></group></optionalFileGroups></installStep></installSteps></config>`},
		{"order-tie", `<config><installSteps><installStep name="S"><optionalFileGroups><group name="G" type="SelectAll"><plugins>` +
			`<plugin name="b"><files><file source="b" destination="x" priority="1"/></files></plugin>` +
			`<plugin name="a"><files><file source="a" destination="x" priority="1"/></files></plugin>` +
			`</plugins></group></optionalFileGroups></installStep></installSteps></config>`},
		{"descending-order", `<config><installSteps order="Descending"><installStep name="2"><optionalFileGroups order="Descending">` +
			`<group name="y" type="SelectAny"><plugins order="Descending">` + fmt.Sprintf(plugin, "b", "b", "b", "Optional") +
			fmt.Sprintf(plugin, "C", "c", "c", "Optional") + fmt.Sprintf(plugin, "a", "a", "a", "Optional") +
			`</plugins></group><group name="z" type="SelectAtLeastOne"><plugins>` + fmt.Sprintf(plugin, "k", "k", "k", "Recommended") +
			`</plugins></group></optionalFileGroups></installStep><installStep name="1"><optionalFileGroups>` +
			`<group name="x" type="SelectAtMostOne"><plugins>` + fmt.Sprintf(plugin, "m", "m", "m", "CouldBeUsable") +
			`</plugins></group></optionalFileGroups></installStep></installSteps></config>`},
	}
	for _, name := range []string{"choose-one.xml", "flags.xml", "required.xml", "folder.xml"} {
		b, err := fs.ReadFile(os.DirFS("testdata"), name)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, struct{ name, xml string }{name, string(b)})
	}
	return out
}

// choiceSets are the answers replayed against a config: none, each group's first plugin, all of them, a name that
// does not exist, and the answers the fixtures' own tests use.
func choiceSets(cfg Config) map[string]Choices {
	sets := map[string]Choices{"none": nil, "bogus": {"S": {"G": {"Nope"}}}}
	first, all := Choices{}, Choices{}
	for _, s := range cfg.Steps {
		for _, g := range s.Groups {
			if first[s.Name] == nil {
				first[s.Name], all[s.Name] = map[string][]string{}, map[string][]string{}
			}
			for i, p := range g.Plugins {
				if i == 0 {
					first[s.Name][g.Name] = []string{p.Name}
				}
				all[s.Name][g.Name] = append(all[s.Name][g.Name], p.Name)
			}
		}
	}
	sets["first"], sets["all"] = first, all
	sets["choose-one"] = Choices{"Options": {"Pack": {"Beta"}}}
	sets["flags-on"] = Choices{"Pick": {"Mode": {"On"}}}
	sets["flags-off"] = Choices{"Pick": {"Mode": {"Off"}}}
	sets["more-addon"] = Choices{"More": {"Extra": {"Addon"}}}
	sets["flag-then-b"] = Choices{"A": {"G": {"On"}}}
	return sets
}

func contexts() map[string]EvalContext {
	all := func(state string) FileIndex { return func(string) string { return state } }
	out := map[string]EvalContext{}
	for _, v := range []string{"", "1.6.15"} {
		for name, files := range map[string]FileIndex{"active": all(FileActive), "inactive": all(FileInactive), "nil": nil} {
			out["v="+v+"|files="+name] = EvalContext{Files: files, GameVersion: v}
		}
	}
	return out
}

func withoutLastGroup(cfg Config) Config {
	out := cfg
	out.Steps = slices.Clone(cfg.Steps)
	for i := len(out.Steps) - 1; i >= 0; i-- {
		if n := len(out.Steps[i].Groups); n > 0 {
			out.Steps[i].Groups = slices.Clone(out.Steps[i].Groups[:n-1])
			break
		}
	}
	return out
}

func showOps(ops []Op) string {
	var b strings.Builder
	for _, o := range ops {
		dest := "<nil>"
		if o.Destination != nil {
			dest = fmt.Sprintf("%q", *o.Destination)
		}
		fmt.Fprintf(&b, "[%s dest=%s p=%d folder=%t always=%t usable=%t]", o.Source, dest, o.Priority, o.Folder, o.Always, o.IfUsable)
	}
	return b.String()
}

func snapshot(name string, cfg Config) map[string]string {
	out := map[string]string{}
	put := func(parts []string, val any) { out[name+"|"+strings.Join(parts, "|")] = fmt.Sprint(val) }
	less := withoutLastGroup(cfg)
	for cname, ch := range choiceSets(cfg) {
		for xname, ctx := range contexts() {
			key := []string{"choices=" + cname, xname}
			put(append([]string{"Match"}, key...), Match(cfg, ch, ctx))
			put(append([]string{"Resolve"}, key...), showOps(Resolve(cfg, ch, ctx)))
			put(append([]string{"Unanswered-self"}, key...), Unanswered(cfg, cfg, ch, ctx))
			put(append([]string{"Unanswered-less"}, key...), Unanswered(less, cfg, ch, ctx))
			put(append([]string{"Keep"}, key...), Keep(cfg, ch))
			flags := FlagsFrom(cfg, ch, ctx)
			put(append([]string{"FlagsFrom"}, key...), flags)
			var visible []string
			for _, s := range VisibleSteps(cfg, flags, ctx) {
				visible = append(visible, s.Name)
			}
			put(append([]string{"VisibleSteps"}, key...), visible)
			for _, s := range cfg.Steps {
				for _, g := range s.Groups {
					for _, p := range g.Plugins {
						put(append([]string{"PluginType", s.Name, g.Name, p.Name}, key...), PluginType(p, flags, ctx))
					}
				}
			}
		}
	}
	return out
}

func TestBehaviourTable(t *testing.T) {
	got := map[string]string{}
	for _, c := range behaviourConfigs(t) {
		cfg, err := Parse([]byte(c.xml))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		maps.Copy(got, snapshot(c.name, cfg))
	}
	const golden = "testdata/behaviour.golden"
	if os.Getenv("FOMOD_RECORD") != "" {
		var lines []string
		for _, k := range slices.Sorted(maps.Keys(got)) {
			lines = append(lines, k+"\t"+got[k])
		}
		if err := os.WriteFile(golden, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSuffix(string(raw), "\n"), "\n") {
		k, v, _ := strings.Cut(line, "\t")
		want[k] = v
	}
	if len(want) != len(got) {
		t.Fatalf("table has %d rows, golden %d", len(got), len(want))
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s\n  got  %s\n  want %s", k, got[k], w)
		}
	}
}
