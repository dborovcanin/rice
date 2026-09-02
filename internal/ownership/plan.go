package ownership

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/config"
)

// Kind is what Rice would do with one configuration path.
type Kind int

const (
	// KindNone means the path already points where it should.
	KindNone Kind = iota
	// KindLink means nothing is there and a symlink can simply be created.
	KindLink
	// KindAdopt means a real file is there: back it up, then link.
	KindAdopt
	// KindRelink means a Rice symlink points at the wrong place.
	KindRelink
	// KindConflict means Rice refuses to touch the path.
	KindConflict
)

func (k Kind) String() string {
	switch k {
	case KindLink:
		return "link"
	case KindAdopt:
		return "adopt"
	case KindRelink:
		return "relink"
	case KindConflict:
		return "conflict"
	default:
		return "ok"
	}
}

// Action is one planned change to one configuration path.
type Action struct {
	Component string
	// Target is the absolute application configuration path.
	Target string
	// Source is the path inside a generation, e.g. "foot/foot.ini".
	Source string
	// LinkTo is the absolute path the symlink should point at.
	LinkTo string
	// Status is what sits at Target now.
	Status Status
	Kind   Kind
	// Reason explains a conflict, and is empty otherwise.
	Reason string
}

// Forceable reports whether --force may override this conflict. A directory
// standing where a file belongs never is: removing it could destroy a tree.
func (a Action) Forceable() bool {
	return a.Kind == KindConflict && a.Status.State == ExternalSymlink
}

// Plan is what setup would do, before anything is done.
type Plan struct {
	Actions   []Action
	Root      string
	ConfigDir string
}

// Changes returns the actions that would modify something.
func (p Plan) Changes() []Action {
	var out []Action
	for _, a := range p.Actions {
		if a.Kind != KindNone && a.Kind != KindConflict {
			out = append(out, a)
		}
	}
	return out
}

// Conflicts returns the actions Rice refuses to perform.
func (p Plan) Conflicts() []Action {
	var out []Action
	for _, a := range p.Actions {
		if a.Kind == KindConflict {
			out = append(out, a)
		}
	}
	return out
}

// Adoptions returns the actions that would displace an existing file.
func (p Plan) Adoptions() []Action {
	var out []Action
	for _, a := range p.Actions {
		if a.Kind == KindAdopt {
			out = append(out, a)
		}
	}
	return out
}

// Empty reports whether the plan would change nothing.
func (p Plan) Empty() bool { return len(p.Changes()) == 0 }

// BuildPlan works out what each enabled component's configuration paths need,
// without modifying anything. Callers are expected to show it before acting.
func BuildPlan(adapters []adapter.Adapter, cfg config.Config, paths config.Paths, configDir string) (Plan, error) {
	plan := Plan{Root: paths.Root, ConfigDir: configDir}

	for _, a := range adapters {
		for _, managed := range adapter.ConfigPathsOf(a, cfg) {
			target := filepath.Join(configDir, filepath.FromSlash(managed.Target))
			linkTo := filepath.Join(paths.Current, filepath.FromSlash(managed.Source))

			status, err := Detect(target, paths.Root)
			if err != nil {
				return Plan{}, err
			}

			action := Action{
				Component: a.Name(),
				Target:    target,
				Source:    managed.Source,
				LinkTo:    linkTo,
				Status:    status,
			}

			switch status.State {
			case Missing:
				action.Kind = KindLink
			case RegularFile:
				action.Kind = KindAdopt
			case RiceManaged:
				if status.LinkTarget == filepath.Clean(linkTo) {
					action.Kind = KindNone
				} else {
					action.Kind = KindRelink
				}
			case BrokenSymlink:
				action.Kind = KindRelink
			case ExternalSymlink:
				action.Kind = KindConflict
				action.Reason = fmt.Sprintf("points at %s, which Rice does not own", status.LinkTarget)
			case Directory:
				action.Kind = KindConflict
				action.Reason = "a directory is where this file belongs"
			}

			plan.Actions = append(plan.Actions, action)
		}
	}

	sort.SliceStable(plan.Actions, func(i, j int) bool {
		if plan.Actions[i].Component != plan.Actions[j].Component {
			return plan.Actions[i].Component < plan.Actions[j].Component
		}
		return plan.Actions[i].Source < plan.Actions[j].Source
	})
	return plan, nil
}
