package fomod

const (
	SelectExactlyOne = "SelectExactlyOne"
	SelectAtMostOne  = "SelectAtMostOne"
	SelectAtLeastOne = "SelectAtLeastOne"
	SelectAll        = "SelectAll"
	SelectAny        = "SelectAny"
)

const (
	TypeRequired      = "Required"
	TypeOptional      = "Optional"
	TypeRecommended   = "Recommended"
	TypeNotUsable     = "NotUsable"
	TypeCouldBeUsable = "CouldBeUsable"
)

const (
	FileActive   = "Active"
	FileInactive = "Inactive"
	FileMissing  = "Missing"
)

// Order values of the installSteps, optionalFileGroups and plugins order attribute. Parse has already applied it to
// the lists it returns: Explicit keeps document order, the others sort by name.
const (
	OrderAscending  = "Ascending"
	OrderDescending = "Descending"
	OrderExplicit   = "Explicit"
)

// Choices is install step name → group name → selected plugin names.
type Choices map[string]map[string][]string

// Op is one file or folder the installer copies from the archive into the install root.
type Op struct {
	Source string
	// Destination is nil when the config names none, which installs at the source's own path; "" is the install
	// folder itself.
	Destination *string
	Priority    int
	Folder      bool
	// Always and IfUsable install a plugin's file even when the plugin is not chosen, IfUsable only while the plugin's
	// type is not NotUsable.
	Always, IfUsable bool
}

// FileIndex reports the state of a dependency file in the installed game: FileActive, FileInactive or FileMissing.
type FileIndex func(name string) string

// EvalContext holds inputs for FOMOD dependency checks.
type EvalContext struct {
	// Files answers fileDependency checks; nil reports every file FileMissing.
	Files FileIndex
	// GameVersion is the installed game's version, compared with each gameDependency minimum by go-semverish: a missing
	// number counts as 0. "" means unknown, and any gameDependency is then unmet; a version either side that does not
	// parse counts as met.
	GameVersion string
}

// Config is a parsed ModuleConfig.xml.
type Config struct {
	ModuleName   string
	Required     []Op
	Steps        []Step
	StepOrder    string
	Conditionals []Conditional
}

type Step struct {
	Name    string
	Visible Composite
	Groups  []Group
	// GroupOrder is the order attribute of this step's optionalFileGroups.
	GroupOrder string
}

type Group struct {
	Name    string
	Type    string
	Plugins []Plugin
	// PluginOrder is the order attribute of this group's plugins.
	PluginOrder string
}

type Plugin struct {
	Name        string
	Description string
	Image       string
	Files       []Op
	Flags       []FlagSet
	TypeName    string
	DefaultType string
	Patterns    []TypePattern
}

type FlagSet struct {
	Name  string
	Value string
}

type TypePattern struct {
	Type Composite
	Name string
}

type Conditional struct {
	When  Composite
	Files []Op
}

type Composite struct {
	Operator string
	Files    []FileDep
	Flags    []FlagDep
	Games    []GameDep
	Nested   []Composite
}

type FileDep struct {
	File  string
	State string
}

type FlagDep struct {
	Flag  string
	Value string
}

type GameDep struct {
	Version string
}
