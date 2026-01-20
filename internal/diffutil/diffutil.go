// internal/diffutil/diffutil.go
package diffutil

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// DiffLine represents a single line in the diff output with inline character-level changes.
type DiffLine struct {
	Type         diffmatchpatch.Operation // DiffEqual, DiffInsert, or DiffDelete
	OrigLineNum  int                      // Original line number (0 if inserted)
	ModLineNum   int                      // Modified line number (0 if deleted)
	InlineDiffs  []diffmatchpatch.Diff    // Character-level diffs within this line
}

// GenerateDiffAndSummary builds a character-level diff organized by lines with a short summary.
func GenerateDiffAndSummary(original, modified string) (lines []DiffLine, summary string) {
	dmp := diffmatchpatch.New()
	dmp.DiffTimeout = 5 * time.Second

	// Perform character-level diff
	diffs := dmp.DiffMain(original, modified, true)
	dmp.DiffCleanupSemantic(diffs)

	// Convert character-level diffs to line-based structure
	lines = convertToLineDiffs(diffs)

	// Build summary
	origLines := lineCount(original)
	modLines := lineCount(modified)

	inserted, deleted, changed := 0, 0, 0
	for _, line := range lines {
		switch line.Type {
		case diffmatchpatch.DiffInsert:
			inserted++
		case diffmatchpatch.DiffDelete:
			deleted++
		case diffmatchpatch.DiffEqual:
			// Check if line has inline changes
			hasChanges := false
			for _, d := range line.InlineDiffs {
				if d.Type != diffmatchpatch.DiffEqual {
					hasChanges = true
					break
				}
			}
			if hasChanges {
				changed++
			}
		}
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Comparison Summary:\n")
	fmt.Fprintf(&buf, "- Original Lines : %d\n", origLines)
	fmt.Fprintf(&buf, "- Modified Lines : %d\n", modLines)
	fmt.Fprintf(&buf, "- Lines Inserted : %d\n", inserted)
	fmt.Fprintf(&buf, "- Lines Deleted  : %d\n", deleted)
	fmt.Fprintf(&buf, "- Lines Changed  : %d\n", changed)

	return lines, buf.String()
}

// convertToLineDiffs converts character-level diffs to a line-based structure
// with inline character-level highlighting.
func convertToLineDiffs(diffs []diffmatchpatch.Diff) []DiffLine {
	var lines []DiffLine
	origLineNum := 1
	modLineNum := 1

	var currentLineDiffs []diffmatchpatch.Diff

	for _, diff := range diffs {
		text := diff.Text
		for len(text) > 0 {
			// Find next newline
			newlineIdx := strings.Index(text, "\n")

			if newlineIdx == -1 {
				// No newline, add to current line buffer
				currentLineDiffs = append(currentLineDiffs, diffmatchpatch.Diff{Type: diff.Type, Text: text})
				break
			}

			// Include the newline in the segment
			segment := text[:newlineIdx+1]
			text = text[newlineIdx+1:]

			// Add this segment to current line
			currentLineDiffs = append(currentLineDiffs, diffmatchpatch.Diff{Type: diff.Type, Text: segment})

			// Line is complete, determine its type and emit it
			lineType, hasDeletes, hasInserts := analyzeLineDiffs(currentLineDiffs)

			var emittedLine DiffLine
			switch {
			case lineType == diffmatchpatch.DiffEqual && !hasDeletes && !hasInserts:
				// Pure unchanged line
				emittedLine = DiffLine{
					Type:        diffmatchpatch.DiffEqual,
					OrigLineNum: origLineNum,
					ModLineNum:  modLineNum,
					InlineDiffs: currentLineDiffs,
				}
				origLineNum++
				modLineNum++

			case hasDeletes && !hasInserts:
				// Pure deletion
				emittedLine = DiffLine{
					Type:        diffmatchpatch.DiffDelete,
					OrigLineNum: origLineNum,
					ModLineNum:  0,
					InlineDiffs: currentLineDiffs,
				}
				origLineNum++

			case hasInserts && !hasDeletes:
				// Pure insertion
				emittedLine = DiffLine{
					Type:        diffmatchpatch.DiffInsert,
					OrigLineNum: 0,
					ModLineNum:  modLineNum,
					InlineDiffs: currentLineDiffs,
				}
				modLineNum++

			default:
				// Mixed: line has both deletes and inserts (inline changes)
				emittedLine = DiffLine{
					Type:        diffmatchpatch.DiffEqual, // Mark as "equal" but with inline changes
					OrigLineNum: origLineNum,
					ModLineNum:  modLineNum,
					InlineDiffs: currentLineDiffs,
				}
				origLineNum++
				modLineNum++
			}

			lines = append(lines, emittedLine)
			currentLineDiffs = nil
		}
	}

	// Handle any remaining content without trailing newline
	if len(currentLineDiffs) > 0 {
		lineType, hasDeletes, hasInserts := analyzeLineDiffs(currentLineDiffs)

		var emittedLine DiffLine
		switch {
		case lineType == diffmatchpatch.DiffEqual && !hasDeletes && !hasInserts:
			emittedLine = DiffLine{
				Type:        diffmatchpatch.DiffEqual,
				OrigLineNum: origLineNum,
				ModLineNum:  modLineNum,
				InlineDiffs: currentLineDiffs,
			}

		case hasDeletes && !hasInserts:
			emittedLine = DiffLine{
				Type:        diffmatchpatch.DiffDelete,
				OrigLineNum: origLineNum,
				ModLineNum:  0,
				InlineDiffs: currentLineDiffs,
			}

		case hasInserts && !hasDeletes:
			emittedLine = DiffLine{
				Type:        diffmatchpatch.DiffInsert,
				OrigLineNum: 0,
				ModLineNum:  modLineNum,
				InlineDiffs: currentLineDiffs,
			}

		default:
			emittedLine = DiffLine{
				Type:        diffmatchpatch.DiffEqual,
				OrigLineNum: origLineNum,
				ModLineNum:  modLineNum,
				InlineDiffs: currentLineDiffs,
			}
		}

		lines = append(lines, emittedLine)
	}

	return lines
}

// analyzeLineDiffs determines the overall type of a line based on its character-level diffs.
func analyzeLineDiffs(diffs []diffmatchpatch.Diff) (mainType diffmatchpatch.Operation, hasDeletes, hasInserts bool) {
	mainType = diffmatchpatch.DiffEqual

	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffDelete:
			hasDeletes = true
			mainType = diffmatchpatch.DiffDelete
		case diffmatchpatch.DiffInsert:
			hasInserts = true
			if mainType == diffmatchpatch.DiffEqual {
				mainType = diffmatchpatch.DiffInsert
			}
		}
	}

	return mainType, hasDeletes, hasInserts
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