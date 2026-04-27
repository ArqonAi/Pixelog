// Fractal-memory data model.
//
// An EraCapsule is a self-similar memory node: it carries the same
// (L0, L1, L2) tier triplet as a session capsule, but its L2 layer holds
// child capsule URIs instead of raw events. This recursion makes the
// memory tree Sierpinski-like — the same retrieval interface works at
// every scale of compression, and "going deeper" means traversing the
// URI graph one hop at a time.
package memory

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// EraLevel names a fractal level. Session is the leaf (raw events live
// inside session capsules); each level above is produced by compacting
// children of the level below.
type EraLevel int

const (
	LevelSession EraLevel = 0
	LevelDay     EraLevel = 1
	LevelWeek    EraLevel = 2
	LevelMonth   EraLevel = 3
	LevelQuarter EraLevel = 4
	LevelYear    EraLevel = 5
	LevelDecade  EraLevel = 6
)

// String returns the canonical level name.
func (l EraLevel) String() string {
	switch l {
	case LevelSession:
		return "session"
	case LevelDay:
		return "day"
	case LevelWeek:
		return "week"
	case LevelMonth:
		return "month"
	case LevelQuarter:
		return "quarter"
	case LevelYear:
		return "year"
	case LevelDecade:
		return "decade"
	default:
		return fmt.Sprintf("level-%d", l)
	}
}

// Parent returns the next coarser level, or itself if already at the top.
func (l EraLevel) Parent() EraLevel {
	if l >= LevelDecade {
		return LevelDecade
	}
	return l + 1
}

// LevelDuration returns the wall-clock window represented by this level.
// Used by the circadian trigger to align compaction boundaries with the
// calendar (Sunday-start ISO weeks; quarters Jan/Apr/Jul/Oct).
func (l EraLevel) LevelDuration() time.Duration {
	switch l {
	case LevelSession:
		return 0 // unbounded; sessions end on user action
	case LevelDay:
		return 24 * time.Hour
	case LevelWeek:
		return 7 * 24 * time.Hour
	case LevelMonth:
		return 30 * 24 * time.Hour
	case LevelQuarter:
		return 91 * 24 * time.Hour
	case LevelYear:
		return 365 * 24 * time.Hour
	case LevelDecade:
		return 10 * 365 * 24 * time.Hour
	default:
		return 0
	}
}

// ParseEraLevel parses a level name back into an enum.
func ParseEraLevel(s string) (EraLevel, error) {
	switch s {
	case "session":
		return LevelSession, nil
	case "day":
		return LevelDay, nil
	case "week":
		return LevelWeek, nil
	case "month":
		return LevelMonth, nil
	case "quarter":
		return LevelQuarter, nil
	case "year":
		return LevelYear, nil
	case "decade":
		return LevelDecade, nil
	default:
		return LevelSession, fmt.Errorf("unknown era level %q", s)
	}
}

// ChildRef points to a child capsule (session or era) inside an EraCapsule.
// Surface tiers are inlined so retrieval can match without resolving the
// child URI; full content is fetched on demand via the resolver.
type ChildRef struct {
	URI       string    `json:"uri"`        // pixe://capsule/<hash>
	Hash      string    `json:"hash"`       // 64-hex content hash (redundant with URI; cached)
	Level     EraLevel  `json:"level"`      // level of the referenced capsule
	L0        string    `json:"l0"`         // surface summary (always inlined)
	L1        string    `json:"l1"`         // detail summary (inlined for "kept" children)
	Salience  float64   `json:"salience"`   // salience at compaction time
	Buried    bool      `json:"buried"`     // true => removed-middle: only L0 inlined
	StartedAt time.Time `json:"started_at"` // earliest message in the subtree
	EndedAt   time.Time `json:"ended_at"`   // latest message in the subtree
	Concepts  []string  `json:"concepts,omitempty"`
}

// EraCapsule is a fractal node: a capsule whose content is a list of
// child capsule references plus its own (L0, L1) summary.
//
// Schema versioning: the JSON serialization carries Version so future
// migrations can be detected without ambiguous parses.
type EraCapsule struct {
	Version   int        `json:"version"`        // schema version; current = 1
	Hash      string     `json:"hash"`           // content hash (sha256-hex of canonical JSON sans this field)
	Namespace string     `json:"namespace"`      // agent / tenant identifier
	Level     EraLevel   `json:"level"`          // semantic level
	StartedAt time.Time  `json:"started_at"`     // earliest message anywhere in subtree
	EndedAt   time.Time  `json:"ended_at"`       // latest message anywhere in subtree
	CreatedAt time.Time  `json:"created_at"`     // wall-clock time of compaction
	L0        string     `json:"l0"`             // single-sentence era summary
	L1        string     `json:"l1"`             // multi-sentence era overview
	Children  []ChildRef `json:"children"`       // ordered by StartedAt asc
	Concepts  []string   `json:"concepts,omitempty"`
}

// EraCapsuleSchemaVersion is the current EraCapsule serialization version.
const EraCapsuleSchemaVersion = 1

// NewEraCapsule constructs an unhashed EraCapsule. Caller must invoke
// Finalize() to compute the canonical content hash before storage.
func NewEraCapsule(namespace string, level EraLevel, children []ChildRef) *EraCapsule {
	ec := &EraCapsule{
		Version:   EraCapsuleSchemaVersion,
		Namespace: namespace,
		Level:     level,
		Children:  append([]ChildRef(nil), children...),
		CreatedAt: time.Now().UTC(),
	}
	sort.Slice(ec.Children, func(i, j int) bool {
		return ec.Children[i].StartedAt.Before(ec.Children[j].StartedAt)
	})
	if len(ec.Children) > 0 {
		ec.StartedAt = ec.Children[0].StartedAt
		ec.EndedAt = ec.Children[len(ec.Children)-1].EndedAt
	}
	return ec
}

// Finalize computes the canonical content hash and populates ec.Hash.
// The hash is sha256 of the JSON encoding with Hash zeroed; this is
// stable across processes and architectures.
func (ec *EraCapsule) Finalize() (string, error) {
	clone := *ec
	clone.Hash = ""
	canonical, err := json.Marshal(&clone)
	if err != nil {
		return "", fmt.Errorf("era: canonical marshal: %w", err)
	}
	sum := sha256.Sum256(canonical)
	ec.Hash = fmt.Sprintf("%x", sum)
	return ec.Hash, nil
}

// URI returns the canonical pixe://capsule/<hash> URI for this era.
// Hash must already be populated via Finalize.
func (ec *EraCapsule) URI() string {
	if ec.Hash == "" {
		return ""
	}
	return BuildCapsuleURI(ec.Hash)
}

// SurfaceChildren returns the kept (non-buried) children — the part of
// the era a surface retrieval would consider.
func (ec *EraCapsule) SurfaceChildren() []ChildRef {
	out := make([]ChildRef, 0, len(ec.Children))
	for _, c := range ec.Children {
		if !c.Buried {
			out = append(out, c)
		}
	}
	return out
}

// BuriedChildren returns the children pushed into the "removed middle".
// They are reachable via deep retrieval but skipped at the surface.
func (ec *EraCapsule) BuriedChildren() []ChildRef {
	out := make([]ChildRef, 0, len(ec.Children))
	for _, c := range ec.Children {
		if c.Buried {
			out = append(out, c)
		}
	}
	return out
}
