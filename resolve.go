package fomod

import (
	"cmp"
	"slices"
	"strings"

	"github.com/Rethunk-Tech/go-semverish"
)

// PluginType is the effective type of a plugin under flags and file dependencies.
func PluginType(p Plugin, flags map[string]string, ctx EvalContext) string {
	for _, pat := range p.Patterns {
		if pat.Type.Eval(flags, ctx) {
			return normType(pat.Name)
		}
	}
	if p.DefaultType != "" {
		return normType(p.DefaultType)
	}
	return normType(p.TypeName)
}

func normType(s string) string {
	switch {
	case strings.EqualFold(s, TypeRequired):
		return TypeRequired
	case strings.EqualFold(s, TypeRecommended):
		return TypeRecommended
	case strings.EqualFold(s, TypeNotUsable):
		return TypeNotUsable
	case strings.EqualFold(s, TypeCouldBeUsable):
		return TypeCouldBeUsable
	default:
		return TypeOptional
	}
}

// Eval reports whether the composite dependency holds.
func (c Composite) Eval(flags map[string]string, ctx EvalContext) bool {
	or := strings.EqualFold(c.Operator, "Or")
	n := len(c.Files) + len(c.Flags) + len(c.Games) + len(c.Nested)
	if n == 0 {
		return !or
	}
	ok := func(v bool) bool {
		if or {
			return v
		}
		return !v
	}
	for _, f := range c.Files {
		if ok(fileOK(f, ctx.Files)) {
			return or
		}
	}
	for _, f := range c.Flags {
		if ok(flags[f.Flag] == f.Value) {
			return or
		}
	}
	for _, g := range c.Games {
		if ok(gameDepOK(ctx.GameVersion, g.Version)) {
			return or
		}
	}
	for _, n := range c.Nested {
		if ok(n.Eval(flags, ctx)) {
			return or
		}
	}
	return !or
}

func gameDepOK(installed, minimum string) bool {
	if minimum == "" {
		return true
	}
	if installed == "" {
		return false
	}
	c, ok := semverish.Compare(installed, minimum)
	return !ok || c >= 0
}

func fileOK(d FileDep, files FileIndex) bool {
	want := cmp.Or(d.State, FileActive)
	got := FileMissing
	if files != nil {
		got = files(d.File)
	}
	return strings.EqualFold(got, want)
}

// VisibleSteps are install steps whose visible condition holds.
func VisibleSteps(cfg Config, flags map[string]string, ctx EvalContext) []Step {
	var out []Step
	for _, s := range cfg.Steps {
		if s.Visible.Eval(flags, ctx) {
			out = append(out, s)
		}
	}
	return out
}

// FlagsFrom returns condition flags set by the selected plugins, in step order.
func FlagsFrom(cfg Config, choices Choices, ctx EvalContext) map[string]string {
	flags := map[string]string{}
	for _, s := range cfg.Steps {
		if !s.Visible.Eval(flags, ctx) {
			continue
		}
		for _, g := range s.Groups {
			raiseFlags(flags, choices, s.Name, g, ctx)
		}
	}
	return flags
}

// raiseFlags sets the flags of g's selected plugins.
func raiseFlags(flags map[string]string, choices Choices, step string, g Group, ctx EvalContext) {
	picked, _ := selected(choices, step, g, flags, ctx)
	for _, i := range picked {
		for _, f := range g.Plugins[i].Flags {
			flags[f.Name] = f.Value
		}
	}
}

// pluginIndex maps each plugin name of g to its position; a repeated name keeps the first.
func pluginIndex(g Group) map[string]int {
	idx := make(map[string]int, len(g.Plugins))
	for i, p := range g.Plugins {
		if _, dup := idx[p.Name]; !dup {
			idx[p.Name] = i
		}
	}
	return idx
}

// selected lists the indexes in g of the chosen plugins plus the plugins that are Required under flags; SelectAll picks
// every plugin, so two that share a name are both installed. unknown counts the chosen names g has no plugin for.
func selected(choices Choices, step string, g Group, flags map[string]string, ctx EvalContext) (picked []int, unknown int) {
	if strings.EqualFold(g.Type, SelectAll) {
		picked = make([]int, len(g.Plugins))
		for i := range picked {
			picked[i] = i
		}
		return picked, 0
	}
	idx := pluginIndex(g)
	have := make(map[int]bool, len(choices[step][g.Name]))
	for _, n := range choices[step][g.Name] {
		i, ok := idx[n]
		switch {
		case !ok:
			unknown++
		case !have[i]:
			have[i] = true
			picked = append(picked, i)
		}
	}
	for i, p := range g.Plugins {
		if !have[i] && PluginType(p, flags, ctx) == TypeRequired {
			picked = append(picked, i)
			have[i] = true
		}
	}
	return picked, unknown
}

// Match reports whether choices still name plugins that exist and satisfy each visible group's type.
func Match(cfg Config, choices Choices, ctx EvalContext) bool {
	flags := map[string]string{}
	for _, s := range cfg.Steps {
		if !s.Visible.Eval(flags, ctx) {
			continue
		}
		for _, g := range s.Groups {
			if !strings.EqualFold(g.Type, SelectAll) && hasRepeat(choices[s.Name][g.Name]) {
				return false
			}
			got, unknown := selected(choices, s.Name, g, flags, ctx)
			if unknown > 0 || !groupOK(g, len(got)) {
				return false
			}
			for _, i := range got {
				if PluginType(g.Plugins[i], flags, ctx) == TypeNotUsable {
					return false
				}
			}
			for _, i := range got {
				for _, f := range g.Plugins[i].Flags {
					flags[f.Name] = f.Value
				}
			}
		}
	}
	return true
}

// hasRepeat reports a name chosen twice, which would install its files twice.
func hasRepeat(names []string) bool {
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		if seen[n] {
			return true
		}
		seen[n] = true
	}
	return false
}

func groupOK(g Group, n int) bool {
	switch {
	case strings.EqualFold(g.Type, SelectExactlyOne):
		return n == 1
	case strings.EqualFold(g.Type, SelectAtMostOne):
		return n <= 1
	case strings.EqualFold(g.Type, SelectAtLeastOne):
		return n >= 1
	case strings.EqualFold(g.Type, SelectAll):
		return true
	default:
		return true
	}
}

// ResolveSafe is Resolve with every operation that Escapes taken out of ops and returned in skipped, so a caller that
// never checks Escapes cannot be sent outside the archive or install root.
func ResolveSafe(cfg Config, choices Choices, ctx EvalContext) (ops, skipped []Op) {
	for _, op := range Resolve(cfg, choices, ctx) {
		if op.Escapes() {
			skipped = append(skipped, op)
		} else {
			ops = append(ops, op)
		}
	}
	return ops, skipped
}

// Resolve lists file operations for required files, selected plugins, and matching conditional installs.
func Resolve(cfg Config, choices Choices, ctx EvalContext) []Op {
	var ops []Op
	ops = append(ops, cfg.Required...)
	flags := map[string]string{}
	for _, s := range cfg.Steps {
		if !s.Visible.Eval(flags, ctx) {
			continue
		}
		for _, g := range s.Groups {
			ops = resolveGroup(ops, flags, choices, s.Name, g, ctx)
		}
	}
	for _, c := range cfg.Conditionals {
		if c.When.Eval(flags, ctx) {
			ops = append(ops, c.Files...)
		}
	}
	slices.SortStableFunc(ops, func(a, b Op) int { return cmp.Compare(a.Priority, b.Priority) })
	return ops
}

// resolveGroup appends the files of g's chosen plugins and of its unchosen plugins that install regardless, and sets the
// flags the chosen plugins raise.
func resolveGroup(ops []Op, flags map[string]string, choices Choices, step string, g Group, ctx EvalContext) []Op {
	chosen, _ := selected(choices, step, g, flags, ctx)
	chosenSet := make(map[int]bool, len(chosen))
	for _, i := range chosen {
		chosenSet[i] = true
		p := g.Plugins[i]
		ops = append(ops, p.Files...)
		for _, f := range p.Flags {
			flags[f.Name] = f.Value
		}
	}
	for i, p := range g.Plugins {
		if chosenSet[i] {
			continue
		}
		usable := PluginType(p, flags, ctx) != TypeNotUsable
		for _, f := range p.Files {
			if f.Always || (f.IfUsable && usable) {
				ops = append(ops, f)
			}
		}
	}
	return ops
}

// Unanswered reports whether cfg shows a group the user picks from that neither old nor choices knew, so replaying
// choices made against old would decide it without asking.
func Unanswered(old, cfg Config, choices Choices, ctx EvalContext) bool {
	seen := map[[2]string]bool{}
	for _, s := range old.Steps {
		for _, g := range s.Groups {
			seen[[2]string{s.Name, g.Name}] = true
		}
	}
	// Visibility is judged where the installer reaches each step, under the flags raised before it.
	flags := map[string]string{}
	for _, s := range cfg.Steps {
		if !s.Visible.Eval(flags, ctx) {
			continue
		}
		for _, g := range s.Groups {
			if _, ok := choices[s.Name][g.Name]; !ok && !strings.EqualFold(g.Type, SelectAll) && !seen[[2]string{s.Name, g.Name}] {
				return true
			}
			raiseFlags(flags, choices, s.Name, g, ctx)
		}
	}
	return false
}

// Keep returns the choices that still name a plugin of cfg. A step and group name that cfg repeats is kept once, so
// Keep is idempotent.
func Keep(cfg Config, choices Choices) Choices {
	out := Choices{}
	seen := map[[2]string]bool{}
	for _, s := range cfg.Steps {
		for _, g := range s.Groups {
			if seen[[2]string{s.Name, g.Name}] {
				continue
			}
			seen[[2]string{s.Name, g.Name}] = true
			idx := pluginIndex(g)
			for _, name := range choices[s.Name][g.Name] {
				if _, ok := idx[name]; !ok {
					continue
				}
				if out[s.Name] == nil {
					out[s.Name] = map[string][]string{}
				}
				out[s.Name][g.Name] = append(out[s.Name][g.Name], name)
			}
		}
	}
	return out
}
