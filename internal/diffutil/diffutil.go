// internal/diffutil/diffutil.go
package diffutil

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// GenerateDiffAndSummary creates a diff result and a basic line-based summary.
// It returns the raw diffs array for manual rendering.
func GenerateDiffAndSummary(original, modified string) (diffs []diffmatchpatch.Diff, summary string) { // <<< Return []Diff now
	dmp := diffmatchpatch.New()
	dmp.DiffTimeout = 5 * time.Second

	// --- Perform Diff Directly ---
	diffs = dmp.DiffMain(original, modified, true) // true = check lines

	// Optional cleanup (can be slow)
	// dmp.DiffCleanupSemantic(diffs)

	// --- Generate Summary (remains the same) ---
	linesOriginal := strings.Split(original, "\n")
	linesModified := strings.Split(modified, "\n")
	var summaryBuilder bytes.Buffer
	opCounts := map[diffmatchpatch.Operation]int{
		diffmatchpatch.DiffEqual:   0,
		diffmatchpatch.DiffInsert: 0,
		diffmatchpatch.DiffDelete: 0,
	}
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffInsert: opCounts[diffmatchpatch.DiffInsert]++
		case diffmatchpatch.DiffDelete: opCounts[diffmatchpatch.DiffDelete]++
		case diffmatchpatch.DiffEqual:  opCounts[diffmatchpatch.DiffEqual]++
		}
	}
	summaryBuilder.WriteString(fmt.Sprintf("Comparison Summary:\n"))
	summaryBuilder.WriteString(fmt.Sprintf("- Original Lines: %d\n", len(linesOriginal)))
	summaryBuilder.WriteString(fmt.Sprintf("- Modified Lines: %d\n", len(linesModified)))
	summaryBuilder.WriteString(fmt.Sprintf("- Change Segments Equal: %d\n", opCounts[diffmatchpatch.DiffEqual]))
	summaryBuilder.WriteString(fmt.Sprintf("- Change Segments Inserted: %d\n", opCounts[diffmatchpatch.DiffInsert]))
	summaryBuilder.WriteString(fmt.Sprintf("- Change Segments Deleted: %d\n", opCounts[diffmatchpatch.DiffDelete]))
	summary = summaryBuilder.String()

	// Return the raw diffs array and the summary.
	return diffs, summary // <<< Return diffs array
}