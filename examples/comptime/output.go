package comptime

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
)

// Stats summarizes a report.
type Stats struct {
	GOOS          string         `json:"goos"`
	GOARCH        string         `json:"goarch"`
	Packages      int            `json:"packages"`
	Failed        int            `json:"failed"`         // the packages that were not analyzed
	TotalSymbols  int            `json:"total_symbols"`  // all exported symbols
	TotalComptime int            `json:"total_comptime"` // all comptime symbols
	Symbols       map[Kind]int   `json:"symbols"`        // the exported symbols of each kind
	Comptime      map[Kind]int   `json:"comptime"`       // the comptime symbols of each kind
	Conditions    map[string]int `json:"conditions"`     // the comptime symbols under each set of conditions
	Reasons       map[string]int `json:"reasons"`        // the other symbols by the kind of their reason
	RootReasons   map[string]int `json:"root_reasons"`   // the other symbols by the construct at the end of the chain
}

// unconditional is the name of the empty set of conditions in the statistics.
const unconditional = "none"

// Stats counts the packages, symbols, verdicts and reasons of a report.
func (r *Report) Stats() *Stats {
	st := &Stats{
		GOOS:        r.GOOS,
		GOARCH:      r.GOARCH,
		Packages:    len(r.Packages),
		Symbols:     map[Kind]int{},
		Comptime:    map[Kind]int{},
		Conditions:  map[string]int{},
		Reasons:     map[string]int{},
		RootReasons: map[string]int{},
	}
	for _, pkg := range r.Packages {
		if pkg.Error != "" {
			st.Failed++
		}
		for _, sym := range pkg.Symbols {
			st.count(sym)
		}
	}
	return st
}

// count adds one symbol to the statistics.
func (st *Stats) count(sym Symbol) {
	st.TotalSymbols++
	st.Symbols[sym.Kind]++
	if !sym.Comptime {
		root := sym.Reason.Root()
		st.Reasons[string(sym.Reason.Kind)]++
		st.RootReasons[string(root.Kind)+": "+root.Detail]++
		return
	}
	st.TotalComptime++
	st.Comptime[sym.Kind]++
	st.Conditions[conditionList(sym.Conditions)]++
}

// conditionList joins conditions with commas, or names the empty set.
func conditionList(conditions []Condition) string {
	if len(conditions) == 0 {
		return unconditional
	}
	names := make([]string, len(conditions))
	for i, c := range conditions {
		names[i] = string(c)
	}
	return strings.Join(names, ", ")
}

// WriteJSON writes any value of this package as indented JSON.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")
	return enc.Encode(v)
}

// WriteText lists, per package, the exported symbols that are comptime and the conditions of each.
func (r *Report) WriteText(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# comptime symbols for GOOS=%s GOARCH=%s\n", r.GOOS, r.GOARCH)
	for _, pkg := range r.Packages {
		writePackage(&b, pkg)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// writePackage writes the path of one package and its comptime symbols, if it has any.
func writePackage(b *strings.Builder, pkg *Package) {
	fmt.Fprintf(b, "\n%s\n", pkg.Path)
	if pkg.Error != "" {
		fmt.Fprintf(b, "\terror: %s\n", pkg.Error)
	}
	for _, sym := range pkg.Symbols {
		if !sym.Comptime {
			continue
		}
		fmt.Fprintf(b, "\t%s %s", sym.Kind, sym.Name)
		if len(sym.Conditions) > 0 {
			fmt.Fprintf(b, "\tif %s", conditionList(sym.Conditions))
		}
		b.WriteByte('\n')
	}
}

// WriteText writes the statistics as aligned plain text.
func (st *Stats) WriteText(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "target    GOOS=%s GOARCH=%s\n", st.GOOS, st.GOARCH)
	fmt.Fprintf(&b, "packages  %d, of which %d not analyzed\n", st.Packages, st.Failed)
	fmt.Fprintf(&b, "symbols   %d, of which %d comptime\n", st.TotalSymbols, st.TotalComptime)
	b.WriteString("\nby kind (comptime / all)\n")
	for _, kind := range []Kind{KindConst, KindVar, KindFunc, KindMethod} {
		fmt.Fprintf(&b, "%8d /%8d  %s\n", st.Comptime[kind], st.Symbols[kind], kind)
	}
	writeHistogram(&b, "comptime symbols by conditions", st.Conditions)
	writeHistogram(&b, "other symbols by reason", st.Reasons)
	writeHistogram(&b, "other symbols by the construct at the end of the chain", st.RootReasons)
	_, err := io.WriteString(w, b.String())
	return err
}

// writeHistogram writes counts from the highest to the lowest, and equal counts by name.
func writeHistogram(b *strings.Builder, title string, counts map[string]int) {
	fmt.Fprintf(b, "\n%s\n", title)
	names := slices.SortedFunc(maps.Keys(counts), func(x, y string) int {
		if counts[x] != counts[y] {
			return counts[y] - counts[x]
		}
		return strings.Compare(x, y)
	})
	for _, name := range names {
		fmt.Fprintf(b, "%8d  %s\n", counts[name], name)
	}
}
