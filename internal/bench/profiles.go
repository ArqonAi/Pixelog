package bench

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// EntityProfile is a structured biography for one entity, split into
// the two dimensions that conversational memory fundamentally has:
//
//   - Traits: enduring preferences, identity, relationships —
//     "likes classical music", "identifies-as counselor",
//     "lives-in Toronto". Rendered UNDATED because these are
//     stable assertions that don't belong on a timeline. Surfacing
//     them with a timestamp misleads the answerer into treating
//     enduring preferences as transient events, which collapses
//     performance on counterfactual questions ("would X likely Y?").
//
//   - Events: dated actions / occurrences — "started HRT on 2023-08",
//     "went hiking last weekend", "moved to Portland". Rendered as
//     a date-sorted timeline so the answerer can do time-arithmetic
//     across entries.
//
// This is the Lever-2 complement to factIndex: factIndex applies
// supersession ("what's Caroline's CURRENT status"), EntityProfile
// preserves the full record ("what do we KNOW about Caroline").
//
// Design choices:
//   - Traits are deduped by (predicate, object) across all turns —
//     repeated mentions collapse to a single entry.
//   - Events are deduped by (predicate, object, yyyy-mm-dd) so the
//     same assertion echoed across consecutive turns of one day
//     surfaces once, but multi-day recurrences are preserved.
//   - Predicate classification is driven by predicateIsTrait(); any
//     predicate not explicitly a trait falls through to events, so
//     LLM-extracted predicates default to the safe timestamped
//     surface.
type EntityProfile struct {
	// Name is the normalised (title-case) subject of every entry.
	Name string
	// Traits are enduring (subject, predicate, object) assertions
	// rendered without dates. Order is stable: predicate ascending,
	// then object ascending.
	Traits []ProfileEntry
	// Events are dated occurrences rendered as a chronological
	// timeline. Order is ascending by timestamp.
	Events []ProfileEntry
}

// Entries is kept for compatibility with callers that iterate over
// the whole record (tests, older renderers). Returns Traits ++ Events
// as a single slice in that order.
func (p *EntityProfile) Entries() []ProfileEntry {
	if p == nil {
		return nil
	}
	out := make([]ProfileEntry, 0, len(p.Traits)+len(p.Events))
	out = append(out, p.Traits...)
	out = append(out, p.Events...)
	return out
}

// ProfileEntry is a single dated assertion in an entity's timeline.
// Fields mirror the upstream Fact but drop supersession metadata
// (Source, SourceSessionID) that the renderer doesn't use.
type ProfileEntry struct {
	Date         time.Time
	Predicate    string
	Object       string
	SourceTurnID string
	Confidence   float64
}

// profileIndex holds every entity's timeline for one conversation.
// Keyed by the normalised (title-case) subject, consistent with
// factIndex.bySubject so callers can use the same extractEntities
// output for lookup without case/whitespace gymnastics.
type profileIndex struct {
	byName map[string]*EntityProfile
}

// buildProfileIndex constructs the profile timelines from the same
// per-turn fact extraction that feeds factIndex. Passing the
// already-built factIndex.all slice avoids re-running the extractor
// and keeps the two structures consistent — every fact that made it
// past supersession also appears in the timeline (plus older facts
// that supersession pruned, which are preserved here).
//
// For Lever 2 specifically we want the UNION, not the superseded
// set — so this function accepts the raw per-turn facts and does its
// own (predicate, object, day) dedup pass. Callers that only have
// the post-supersession set can still pass factIndex.all; they'll
// get a lower-recall but internally consistent timeline.
func buildProfileIndex(allFacts []Fact) *profileIndex {
	if len(allFacts) == 0 {
		return &profileIndex{byName: map[string]*EntityProfile{}}
	}

	// Traits dedup key: (subject, predicate, object) — no date. A
	// repeated "Caroline likes classical music" across multiple
	// turns collapses to one entry. Keep highest confidence.
	type traitKey struct {
		subject, pred, obj string
	}
	traits := map[traitKey]ProfileEntry{}

	// Events dedup key: (subject, predicate, object, day). Same-day
	// paraphrased mentions collapse; multi-day recurrences preserved.
	type eventKey struct {
		subject, pred, obj, day string
	}
	events := map[eventKey]ProfileEntry{}

	for _, f := range allFacts {
		if f.Subject == "" || f.Predicate == "" || f.Object == "" {
			continue
		}
		name := titleCaseName(f.Subject)
		entry := ProfileEntry{
			Date:         f.Timestamp,
			Predicate:    f.Predicate,
			Object:       f.Object,
			SourceTurnID: f.SourceTurnID,
			Confidence:   f.Confidence,
		}
		if predicateIsTrait(f.Predicate) {
			k := traitKey{subject: name, pred: f.Predicate, obj: f.Object}
			if existing, ok := traits[k]; !ok || entry.Confidence > existing.Confidence {
				// Traits are undated at render time; clear the
				// Date field so downstream renderers never leak a
				// misleading timestamp for an enduring preference.
				entry.Date = time.Time{}
				traits[k] = entry
			}
			continue
		}
		day := "unknown"
		if !f.Timestamp.IsZero() {
			day = f.Timestamp.Format("2006-01-02")
		}
		k := eventKey{subject: name, pred: f.Predicate, obj: f.Object, day: day}
		if existing, ok := events[k]; !ok || entry.Confidence > existing.Confidence {
			events[k] = entry
		}
	}

	idx := &profileIndex{byName: map[string]*EntityProfile{}}
	getOrCreate := func(name string) *EntityProfile {
		if p, ok := idx.byName[name]; ok {
			return p
		}
		p := &EntityProfile{Name: name}
		idx.byName[name] = p
		return p
	}
	for k, e := range traits {
		p := getOrCreate(k.subject)
		p.Traits = append(p.Traits, e)
	}
	for k, e := range events {
		p := getOrCreate(k.subject)
		p.Events = append(p.Events, e)
	}

	for _, prof := range idx.byName {
		// Traits: stable by (predicate, object) ascending.
		sort.SliceStable(prof.Traits, func(i, j int) bool {
			if prof.Traits[i].Predicate != prof.Traits[j].Predicate {
				return prof.Traits[i].Predicate < prof.Traits[j].Predicate
			}
			return prof.Traits[i].Object < prof.Traits[j].Object
		})
		// Events: ascending by timestamp, undated sink to the tail.
		sort.SliceStable(prof.Events, func(i, j int) bool {
			ai, aj := prof.Events[i].Date, prof.Events[j].Date
			if ai.IsZero() && aj.IsZero() {
				if prof.Events[i].Predicate != prof.Events[j].Predicate {
					return prof.Events[i].Predicate < prof.Events[j].Predicate
				}
				return prof.Events[i].Object < prof.Events[j].Object
			}
			if ai.IsZero() {
				return false
			}
			if aj.IsZero() {
				return true
			}
			if !ai.Equal(aj) {
				return ai.Before(aj)
			}
			if prof.Events[i].Predicate != prof.Events[j].Predicate {
				return prof.Events[i].Predicate < prof.Events[j].Predicate
			}
			return prof.Events[i].Object < prof.Events[j].Object
		})
	}
	return idx
}

// Lookup returns the profiles for a set of query subjects. The
// caller passes the set of capitalised entities from the question
// (via extractEntities). Returns an empty slice when nothing matches
// or when the index is nil / empty. Results are sorted by subject
// name so the rendered preamble has stable ordering across runs.
func (idx *profileIndex) Lookup(subjects []string) []*EntityProfile {
	if idx == nil || len(subjects) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]*EntityProfile, 0, len(subjects))
	for _, s := range subjects {
		key := titleCaseName(s)
		if seen[key] {
			continue
		}
		seen[key] = true
		if p, ok := idx.byName[key]; ok && (len(p.Traits) > 0 || len(p.Events) > 0) {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// profilePreambleConfig caps how much of the profile surface is
// rendered in the preamble. Defaults are tuned for LoCoMo's k=30
// answerer budget: 3 entities × (10 traits + 8 events) × ~50
// chars/entry ≈ 2.5 KB, which leaves plenty of headroom for the
// k=30 turns that follow.
type profilePreambleConfig struct {
	MaxEntities    int
	MaxTraits      int
	MaxEvents      int
}

func defaultProfilePreambleConfig() profilePreambleConfig {
	return profilePreambleConfig{MaxEntities: 3, MaxTraits: 10, MaxEvents: 8}
}

// formatProfilePreamble renders a deterministic, structured profile
// block suitable for prepending to the answerer context. Traits and
// events render in separate sections so the answerer picks the right
// one for the question type — enduring preferences for counterfactual
// inference, dated events for time arithmetic.
//
// Shape:
//
//	KNOWN PROFILES:
//	Caroline:
//	  traits:
//	    - likes classical music
//	    - identifies-as counselor
//	    - raised-in a supportive family
//	    - relationship-status: single
//	  events:
//	    2023-05-14: did-event went hiking
//	    2023-08-22: did-event started HRT
//
// Entities with only traits render just the traits section; entities
// with only events render just the events section; entities with
// neither are skipped entirely. Returns the empty string when the
// profile set is empty so the caller can skip the preamble cleanly.
func formatProfilePreamble(profiles []*EntityProfile, cfg profilePreambleConfig) string {
	if len(profiles) == 0 {
		return ""
	}
	if cfg.MaxEntities <= 0 {
		return ""
	}
	if cfg.MaxTraits <= 0 && cfg.MaxEvents <= 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("KNOWN PROFILES:\n")
	emitted := 0
	for _, p := range profiles {
		if emitted >= cfg.MaxEntities {
			break
		}
		hasTraits := cfg.MaxTraits > 0 && len(p.Traits) > 0
		hasEvents := cfg.MaxEvents > 0 && len(p.Events) > 0
		if !hasTraits && !hasEvents {
			continue
		}
		sb.WriteString(p.Name)
		sb.WriteString(":\n")

		if hasTraits {
			sb.WriteString("  traits:\n")
			limit := cfg.MaxTraits
			if limit > len(p.Traits) {
				limit = len(p.Traits)
			}
			for i := 0; i < limit; i++ {
				e := p.Traits[i]
				// "    - <predicate> <object>" — no date. Predicates
				// stay lowercase for consistency with facts.go.
				fmt.Fprintf(&sb, "    - %s %s\n", e.Predicate, e.Object)
			}
		}

		if hasEvents {
			sb.WriteString("  events:\n")
			limit := cfg.MaxEvents
			if limit > len(p.Events) {
				limit = len(p.Events)
			}
			for i := 0; i < limit; i++ {
				e := p.Events[i]
				dateStr := "(undated)"
				if !e.Date.IsZero() {
					dateStr = e.Date.Format("2006-01-02")
				}
				fmt.Fprintf(&sb, "    %s: %s %s\n", dateStr, e.Predicate, e.Object)
			}
		}
		emitted++
	}
	if emitted == 0 {
		return ""
	}
	return sb.String()
}
