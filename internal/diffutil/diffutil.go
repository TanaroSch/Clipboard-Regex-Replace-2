// internal/diffutil/diffutil.go
package diffutil

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// GenerateDiffAndSummary builds a pure line‑based diff and a short summary.
func GenerateDiffAndSummary(original, modified string) (diffs []diffmatchpatch.Diff, summary string) {
	dmp := diffmatchpatch.New()
	dmp.DiffTimeout = 5 * time.Second

	// ------------------------------------------------------------------
	// 1. Collapse every *physical* line into a single rune.
	// ------------------------------------------------------------------
	a, b, lineArray := dmp.DiffLinesToRunes(original, modified)

	// ------------------------------------------------------------------
	// 2. Run the diff on those rune slices – checklines MUST be false
	//    because we already chunked input by line.
	// ------------------------------------------------------------------
	diffs = dmp.DiffMainRunes(a, b, false)

	// ------------------------------------------------------------------
	// 3. Expand the runes back to the original lines.
	// ------------------------------------------------------------------
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	// Optional, but makes the output a bit cleaner (splits huge replace blocks).
	dmp.DiffCleanupSemanticLossless(diffs)

	// ------------------------------------------------------------------
	// Build a simple human‑readable summary
	// ------------------------------------------------------------------
	origLines := lineCount(original)
	modLines  := lineCount(modified)

	inserted, deleted := 0, 0
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			inserted += lineCount(d.Text)
		case diffmatchpatch.DiffDelete:
			deleted  += lineCount(d.Text)
		}
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Comparison Summary:\n")
	fmt.Fprintf(&buf, "- Original Lines : %d\n", origLines)
	fmt.Fprintf(&buf, "- Modified Lines : %d\n", modLines)
	fmt.Fprintf(&buf, "- Lines Inserted : %d\n", inserted)
	fmt.Fprintf(&buf, "- Lines Deleted  : %d\n", deleted)

	return diffs, buf.String()
}

// lineCount returns the number of *physical* lines in the snippet.
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++ // final line has no trailing newline
	}
	return n
}