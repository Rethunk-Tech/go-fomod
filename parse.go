package fomod

import (
	"bytes"
	"cmp"
	"encoding/xml"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Parse reads the bytes of a ModuleConfig.xml document. The encoding comes from the bytes (decodeXML, then
// Windows-1252 for anything still not UTF-8); the declaration is ignored because real ones are often wrong.
func Parse(data []byte) (Config, error) {
	var raw xmlConfig
	data = decodeLegacy(decodeXML(data))
	if bytes.Count(data, []byte("<")) > maxMarkup {
		return Config{}, ErrTooComplex
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charsetReader
	if err := dec.Decode(&raw); err != nil {
		return Config{}, fmt.Errorf("fomod: %w", err)
	}
	cfg := Config{ModuleName: strings.TrimSpace(raw.ModuleName), StepOrder: normOrder(raw.Steps.Order)}
	cfg.Required = appendOps(nil, raw.Required.items)
	for _, s := range raw.Steps.Steps {
		st := Step{Name: s.Name, Visible: s.Visible.Composite, GroupOrder: normOrder(s.Groups.Order)}
		for _, g := range s.Groups.Groups {
			gr := Group{Name: g.Name, Type: g.Type, PluginOrder: normOrder(g.Plugins.Order)}
			for _, p := range g.Plugins.Plugins {
				pl := Plugin{
					Name:        p.Name,
					Description: strings.TrimSpace(p.Description),
					Image:       p.Image.Path,
					Flags:       flagSets(p.Flags.Flags),
					TypeName:    p.Type.Type.Name,
					DefaultType: p.Type.Dependency.Default.Name,
				}
				pl.Files = appendOps(pl.Files, p.Files.items)
				for _, pat := range p.Type.Dependency.Patterns.Patterns {
					pl.Patterns = append(pl.Patterns, TypePattern{Type: pat.Dependencies.Composite, Name: pat.Type.Name})
				}
				gr.Plugins = append(gr.Plugins, pl)
			}
			sortByOrder(gr.Plugins, gr.PluginOrder, func(p Plugin) string { return p.Name })
			st.Groups = append(st.Groups, gr)
		}
		sortByOrder(st.Groups, st.GroupOrder, func(g Group) string { return g.Name })
		cfg.Steps = append(cfg.Steps, st)
	}
	sortByOrder(cfg.Steps, cfg.StepOrder, func(s Step) string { return s.Name })
	for _, p := range raw.Conditionals.Patterns.Patterns {
		c := Conditional{When: p.Dependencies.Composite}
		c.Files = appendOps(c.Files, p.Files.items)
		cfg.Conditionals = append(cfg.Conditionals, c)
	}
	return cfg, nil
}

// normOrder maps an order attribute to its constant; an absent or unknown value is the schema default, Ascending.
func normOrder(s string) string {
	switch {
	case strings.EqualFold(s, OrderDescending):
		return OrderDescending
	case strings.EqualFold(s, OrderExplicit):
		return OrderExplicit
	default:
		return OrderAscending
	}
}

// sortByOrder sorts by name unless the order is Explicit. Names compare by UTF-16 code unit, case-sensitively, as
// Qt's QString operator< does in MO2's installer_fomod (fomodinstallerdialog.cpp ControlsAscending, PagesAscending).
func sortByOrder[T any](items []T, order string, name func(T) string) {
	if order == OrderExplicit {
		return
	}
	type keyed struct {
		item T
		key  []uint16
	}
	ks := make([]keyed, len(items))
	for i, t := range items {
		ks[i] = keyed{t, utf16.Encode([]rune(name(t)))}
	}
	slices.SortStableFunc(ks, func(a, b keyed) int {
		c := slices.Compare(a.key, b.key)
		if order == OrderDescending {
			return -c
		}
		return c
	})
	for i, k := range ks {
		items[i] = k.item
	}
}

// charsetReader accepts any declared label: Parse has already made the bytes UTF-8 (decodeXML, then the
// Windows-1252 fallback), and real declarations are often wrong.
func charsetReader(_ string, input io.Reader) (io.Reader, error) {
	return input, nil
}

func flagSets(flags []xmlFlag) []FlagSet {
	out := make([]FlagSet, len(flags))
	for i, f := range flags {
		out[i] = FlagSet{Name: f.Name, Value: strings.TrimSpace(f.Value)}
	}
	return out
}

func appendOps(dst []Op, items []xmlFile) []Op {
	n := len(dst)
	for _, it := range items {
		dst = append(dst, Op{
			Source:      it.Source,
			Destination: it.Destination,
			Priority:    atoi(it.Priority),
			Folder:      it.folder,
			Always:      isTrue(it.AlwaysInstall),
			IfUsable:    isTrue(it.InstallIfUsable),
		})
	}
	// Higher priority overwrites, so it sorts later; equals keep document order.
	slices.SortStableFunc(dst[n:], func(a, b Op) int {
		return cmp.Compare(a.Priority, b.Priority)
	})
	return dst
}

func isTrue(s string) bool {
	b, _ := strconv.ParseBool(strings.TrimSpace(s))
	return b
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

type xmlConfig struct {
	ModuleName   string          `xml:"moduleName"`
	Required     xmlFiles        `xml:"requiredInstallFiles"`
	Steps        xmlSteps        `xml:"installSteps"`
	Conditionals xmlConditionals `xml:"conditionalFileInstalls"`
}

type xmlSteps struct {
	Order string    `xml:"order,attr"`
	Steps []xmlStep `xml:"installStep"`
}

type xmlStep struct {
	Name    string     `xml:"name,attr"`
	Visible xmlVisible `xml:"visible"`
	Groups  xmlGroups  `xml:"optionalFileGroups"`
}

type xmlVisible struct {
	Composite
}

type xmlGroups struct {
	Order  string     `xml:"order,attr"`
	Groups []xmlGroup `xml:"group"`
}

type xmlGroup struct {
	Name    string     `xml:"name,attr"`
	Type    string     `xml:"type,attr"`
	Plugins xmlPlugins `xml:"plugins"`
}

type xmlPlugins struct {
	Order   string      `xml:"order,attr"`
	Plugins []xmlPlugin `xml:"plugin"`
}

type xmlPlugin struct {
	Name        string      `xml:"name,attr"`
	Description string      `xml:"description"`
	Image       xmlImage    `xml:"image"`
	Files       xmlFiles    `xml:"files"`
	Flags       xmlFlags    `xml:"conditionFlags"`
	Type        xmlTypeDesc `xml:"typeDescriptor"`
}

type xmlImage struct {
	Path string `xml:"path,attr"`
}

type xmlFiles struct {
	items []xmlFile
}

type xmlFile struct {
	Source          string  `xml:"source,attr"`
	Destination     *string `xml:"destination,attr"`
	Priority        string  `xml:"priority,attr"`
	AlwaysInstall   string  `xml:"alwaysInstall,attr"`
	InstallIfUsable string  `xml:"installIfUsable,attr"`
	folder          bool
}

// UnmarshalXML keeps file and folder entries in one list so equal-priority ops stay in document order.
func (f *xmlFiles) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "file", "folder":
				var it xmlFile
				if err := d.DecodeElement(&it, &t); err != nil {
					return err
				}
				it.folder = t.Name.Local == "folder"
				f.items = append(f.items, it)
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

type xmlFlags struct {
	Flags []xmlFlag `xml:"flag"`
}

type xmlFlag struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

type xmlTypeDesc struct {
	Type       xmlTypeName `xml:"type"`
	Dependency xmlDepType  `xml:"dependencyType"`
}

type xmlTypeName struct {
	Name string `xml:"name,attr"`
}

type xmlDepType struct {
	Default  xmlTypeName `xml:"defaultType"`
	Patterns xmlTypePats `xml:"patterns"`
}

type xmlTypePats struct {
	Patterns []xmlTypePat `xml:"pattern"`
}

type xmlTypePat struct {
	Dependencies xmlDeps     `xml:"dependencies"`
	Type         xmlTypeName `xml:"type"`
}

type xmlConditionals struct {
	Patterns xmlCondPats `xml:"patterns"`
}

type xmlCondPats struct {
	Patterns []xmlCondPat `xml:"pattern"`
}

type xmlCondPat struct {
	Dependencies xmlDeps  `xml:"dependencies"`
	Files        xmlFiles `xml:"files"`
}

type xmlDeps struct {
	Composite
}

func (c *Composite) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		if strings.EqualFold(a.Name.Local, "operator") {
			c.Operator = a.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := decodeChild(c, d, t); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

// decodeChild decodes one child of a composite into the list its tag names, skipping any other tag.
func decodeChild(c *Composite, d *xml.Decoder, t xml.StartElement) error {
	switch t.Name.Local {
	case "fileDependency":
		return decodeInto(d, t, &c.Files)
	case "flagDependency":
		return decodeInto(d, t, &c.Flags)
	case "gameDependency":
		return decodeInto(d, t, &c.Games)
	case "dependencies", "visible", "compositeDependency":
		return decodeInto(d, t, &c.Nested)
	default:
		return d.Skip()
	}
}

func decodeInto[T any](d *xml.Decoder, t xml.StartElement, dst *[]T) error {
	var v T
	if err := d.DecodeElement(&v, &t); err != nil {
		return err
	}
	*dst = append(*dst, v)
	return nil
}

func (f *FileDep) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		switch strings.ToLower(a.Name.Local) {
		case "file":
			f.File = a.Value
		case "state":
			f.State = a.Value
		}
	}
	return d.Skip()
}

func (f *FlagDep) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		switch strings.ToLower(a.Name.Local) {
		case "flag":
			f.Flag = a.Value
		case "value":
			f.Value = a.Value
		}
	}
	return d.Skip()
}

func (g *GameDep) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		if strings.EqualFold(a.Name.Local, "version") {
			g.Version = a.Value
		}
	}
	return d.Skip()
}
