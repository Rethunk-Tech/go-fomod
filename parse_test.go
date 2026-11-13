package fomod

import (
	"slices"
	"testing"
)

func TestParseInstallListDocumentOrder(t *testing.T) {
	const (
		folderThenFile = `<folder source="A" destination="A" priority="0"/><file source="A/x" destination="A/x" priority="0"/>`
		fileThenFolder = `<file source="A/x" destination="A/x" priority="0"/><folder source="A" destination="A" priority="0"/>`
	)
	lists := []struct {
		name string
		wrap func(string) string
		pick func(Config) []Op
	}{
		{
			"required",
			func(inner string) string {
				return `<config><moduleName>m</moduleName><requiredInstallFiles>` + inner + `</requiredInstallFiles></config>`
			},
			func(c Config) []Op { return c.Required },
		},
		{
			"plugin",
			func(inner string) string {
				return `<config><moduleName>m</moduleName><installSteps><installStep name="s"><optionalFileGroups><group name="g" type="SelectAny"><plugins><plugin name="p"><description>d</description><files>` + inner + `</files><typeDescriptor><type name="Optional"/></typeDescriptor></plugin></plugins></group></optionalFileGroups></installStep></installSteps></config>`
			},
			func(c Config) []Op { return c.Steps[0].Groups[0].Plugins[0].Files },
		},
		{
			"conditional",
			func(inner string) string {
				return `<config><moduleName>m</moduleName><conditionalFileInstalls><patterns><pattern><dependencies operator="And"/><files>` + inner + `</files></pattern></patterns></conditionalFileInstalls></config>`
			},
			func(c Config) []Op { return c.Conditionals[0].Files },
		},
	}
	orders := []struct {
		name, inner string
		want        []string
	}{
		{"folder then file", folderThenFile, []string{"A", "A/x"}},
		{"file then folder", fileThenFolder, []string{"A/x", "A"}},
	}
	for _, list := range lists {
		for _, tc := range orders {
			t.Run(list.name+"/"+tc.name, func(t *testing.T) {
				cfg, err := Parse([]byte(list.wrap(tc.inner)))
				if err != nil {
					t.Fatal(err)
				}
				got := opSources(list.pick(cfg))
				if !slices.Equal(got, tc.want) {
					t.Fatalf("ops %q, want %q", got, tc.want)
				}
			})
		}
	}
}

func opSources(ops []Op) []string {
	out := make([]string, len(ops))
	for i, op := range ops {
		out[i] = op.Source
	}
	return out
}
