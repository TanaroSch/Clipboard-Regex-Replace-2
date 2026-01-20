// internal/ui/diffviewer.go
package ui

import (
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TanaroSch/clipboard-regex-replace/internal/diffutil"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// renderInlineDiffHtml generates a unified diff view with character-level inline highlighting.
func renderInlineDiffHtml(lines []diffutil.DiffLine, contextLines int) string {
	var builder strings.Builder
	foldThreshold := (contextLines * 2) + 1 // Minimum unchanged lines to trigger folding

	builder.WriteString(`<div class="diff-container">`)

	// Group consecutive unchanged lines for folding
	var unchangedGroup []diffutil.DiffLine

	for i, line := range lines {
		isUnchanged := line.Type == diffmatchpatch.DiffEqual && !hasInlineChanges(line)

		if isUnchanged {
			unchangedGroup = append(unchangedGroup, line)

			// Check if this is the last line or next line is changed
			isLast := i == len(lines)-1
			nextIsChanged := !isLast && (lines[i+1].Type != diffmatchpatch.DiffEqual || hasInlineChanges(lines[i+1]))

			if isLast || nextIsChanged {
				// Process the accumulated unchanged group
				if len(unchangedGroup) >= foldThreshold {
					renderFoldableGroup(&builder, unchangedGroup, contextLines)
				} else {
					// Render all lines without folding
					for _, uLine := range unchangedGroup {
						renderDiffLine(&builder, uLine)
					}
				}
				unchangedGroup = nil
			}
		} else {
			// Changed line - flush any accumulated unchanged lines first
			if len(unchangedGroup) > 0 {
				if len(unchangedGroup) >= foldThreshold {
					renderFoldableGroup(&builder, unchangedGroup, contextLines)
				} else {
					for _, uLine := range unchangedGroup {
						renderDiffLine(&builder, uLine)
					}
				}
				unchangedGroup = nil
			}

			// Render the changed line
			renderDiffLine(&builder, line)
		}
	}

	builder.WriteString(`</div>`)
	return builder.String()
}

// hasInlineChanges checks if a line has character-level changes.
func hasInlineChanges(line diffutil.DiffLine) bool {
	for _, d := range line.InlineDiffs {
		if d.Type != diffmatchpatch.DiffEqual {
			return true
		}
	}
	return false
}

// renderFoldableGroup renders a group of unchanged lines with folding capability.
func renderFoldableGroup(builder *strings.Builder, group []diffutil.DiffLine, contextLines int) {
	// Render first contextLines
	for i := 0; i < contextLines && i < len(group); i++ {
		renderDiffLine(builder, group[i])
	}

	// Render fold marker
	skippedCount := len(group) - (contextLines * 2)
	if skippedCount > 0 {
		firstHidden := group[contextLines].OrigLineNum
		lastHidden := group[len(group)-contextLines-1].OrigLineNum

		builder.WriteString(fmt.Sprintf(
			`<div class="line foldable collapsed" onclick="this.classList.toggle('collapsed')">
				<span class="line-num">%d-%d</span>
				<span class="line-num">%d-%d</span>
				<span class="line-op">⋮</span>
				<span class="line-content fold-indicator">
					<span class="fold-text">%d unchanged lines (click to expand)</span>
				</span>
			</div>`,
			firstHidden, lastHidden,
			firstHidden, lastHidden,
			skippedCount,
		))

		// Hidden lines (wrapped in collapsible container)
		builder.WriteString(`<div class="fold-content">`)
		for i := contextLines; i < len(group)-contextLines; i++ {
			renderDiffLine(builder, group[i])
		}
		builder.WriteString(`</div>`)
	}

	// Render last contextLines
	startIdx := len(group) - contextLines
	if startIdx < 0 {
		startIdx = 0
	}
	for i := startIdx; i < len(group); i++ {
		renderDiffLine(builder, group[i])
	}
}

// renderDiffLine renders a single line with inline character-level highlighting.
func renderDiffLine(builder *strings.Builder, line diffutil.DiffLine) {
	lineClass := ""
	opChar := " "

	switch line.Type {
	case diffmatchpatch.DiffDelete:
		lineClass = "diff-delete"
		opChar = "-"
	case diffmatchpatch.DiffInsert:
		lineClass = "diff-insert"
		opChar = "+"
	case diffmatchpatch.DiffEqual:
		if hasInlineChanges(line) {
			lineClass = "diff-modified"
			opChar = "~"
		} else {
			lineClass = "diff-equal"
			opChar = " "
		}
	}

	origNumStr := ""
	if line.OrigLineNum > 0 {
		origNumStr = fmt.Sprintf("%d", line.OrigLineNum)
	}
	modNumStr := ""
	if line.ModLineNum > 0 {
		modNumStr = fmt.Sprintf("%d", line.ModLineNum)
	}

	// Render inline diffs with character-level highlighting
	contentHtml := renderInlineContent(line.InlineDiffs)

	builder.WriteString(fmt.Sprintf(
		`<div class="line %s">
			<span class="line-num orig-num">%s</span>
			<span class="line-num mod-num">%s</span>
			<span class="line-op">%s</span>
			<span class="line-content">%s</span>
		</div>`,
		lineClass, origNumStr, modNumStr, opChar, contentHtml,
	))
}

// renderInlineContent renders character-level diffs within a line.
func renderInlineContent(diffs []diffmatchpatch.Diff) string {
	var builder strings.Builder

	for _, diff := range diffs {
		escapedText := html.EscapeString(diff.Text)

		// Replace spaces with non-breaking spaces for better rendering
		escapedText = strings.ReplaceAll(escapedText, " ", "&nbsp;")

		// Preserve newlines
		escapedText = strings.ReplaceAll(escapedText, "\n", "")

		switch diff.Type {
		case diffmatchpatch.DiffEqual:
			builder.WriteString(escapedText)
		case diffmatchpatch.DiffDelete:
			builder.WriteString(fmt.Sprintf(`<span class="char-delete">%s</span>`, escapedText))
		case diffmatchpatch.DiffInsert:
			builder.WriteString(fmt.Sprintf(`<span class="char-insert">%s</span>`, escapedText))
		}
	}

	return builder.String()
}

// ShowDiffViewer generates an HTML diff view and opens it in the default browser.
func ShowDiffViewer(original, modified string, contextLines int) {
	log.Println("Generating enhanced diff view...")
	lines, summary := diffutil.GenerateDiffAndSummary(original, modified)

	// Use provided contextLines (or default if <= 0)
	if contextLines <= 0 {
		contextLines = 3 // Fallback to default
	}
	renderedHtmlDiffContent := renderInlineDiffHtml(lines, contextLines)

	htmlContent := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Clipboard Change Details</title>
    <style>
        * {
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            margin: 20px;
            background-color: #f5f5f5;
            color: #24292e;
            line-height: 1.5;
        }
        h1, h2 {
            border-bottom: 2px solid #e1e4e8;
            padding-bottom: 10px;
            color: #0366d6;
            margin-top: 24px;
            margin-bottom: 16px;
        }
        pre.summary {
            background-color: #f6f8fa;
            border: 1px solid #d1d5da;
            padding: 16px;
            overflow-x: auto;
            white-space: pre-wrap;
            word-wrap: break-word;
            font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Monaco, "Courier New", monospace;
            font-size: 13px;
            line-height: 1.45;
            border-radius: 6px;
            margin-bottom: 24px;
        }
        .diff-container {
            font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Monaco, "Courier New", monospace;
            font-size: 12px;
            line-height: 1.6;
            border: 1px solid #d1d5da;
            background-color: #ffffff;
            border-radius: 6px;
            overflow: hidden;
        }
        .line {
            display: flex;
            align-items: stretch;
            min-height: 20px;
            border-bottom: 1px solid #f0f0f0;
        }
        .line:last-child {
            border-bottom: none;
        }
        .line-num {
            display: inline-block;
            width: 50px;
            padding: 2px 10px;
            text-align: right;
            color: #57606a;
            background-color: #f6f8fa;
            border-right: 1px solid #d1d5da;
            user-select: none;
            flex-shrink: 0;
            font-weight: 400;
        }
        .line-num.orig-num {
            border-right: none;
        }
        .line-num.mod-num {
            border-right: 1px solid #d1d5da;
        }
        .line-op {
            display: inline-block;
            width: 20px;
            text-align: center;
            color: #57606a;
            user-select: none;
            font-weight: bold;
            flex-shrink: 0;
            padding: 2px 5px;
            background-color: #f6f8fa;
            border-right: 1px solid #d1d5da;
        }
        .line-content {
            display: inline-block;
            padding: 2px 10px;
            white-space: pre;
            flex-grow: 1;
            overflow-x: auto;
        }

        /* Line type styling */
        .line.diff-insert {
            background-color: #e6ffec;
        }
        .line.diff-insert .line-op {
            color: #22863a;
            background-color: #cdffd8;
        }
        .line.diff-insert .line-num {
            background-color: #cdffd8;
        }

        .line.diff-delete {
            background-color: #ffebe9;
        }
        .line.diff-delete .line-op {
            color: #cb2431;
            background-color: #ffdce0;
        }
        .line.diff-delete .line-num {
            background-color: #ffdce0;
        }

        .line.diff-modified {
            background-color: #fff8c5;
        }
        .line.diff-modified .line-op {
            color: #735c0f;
            background-color: #fffbdd;
        }
        .line.diff-modified .line-num {
            background-color: #fffbdd;
        }

        .line.diff-equal {
            background-color: #ffffff;
        }

        /* Character-level highlighting */
        .char-delete {
            background-color: #ffdce0;
            color: #cb2431;
            font-weight: 600;
            text-decoration: line-through;
        }
        .char-insert {
            background-color: #acf2bd;
            color: #22863a;
            font-weight: 600;
        }

        /* Foldable sections */
        .line.foldable {
            background-color: #f6f8fa;
            border: 1px solid #d1d5da;
            cursor: pointer;
            font-style: italic;
            color: #57606a;
            transition: background-color 0.2s;
        }
        .line.foldable:hover {
            background-color: #e1e4e8;
        }
        .line.foldable .line-num {
            background-color: transparent;
            border-right: 1px solid #d1d5da;
        }
        .line.foldable .line-op {
            background-color: transparent;
            color: #57606a;
            border-right: 1px solid #d1d5da;
        }
        .fold-indicator {
            text-align: center;
            flex-grow: 1;
        }
        .fold-text {
            font-size: 11px;
            color: #0366d6;
        }
        .fold-content {
            display: none;
        }
        .line.foldable.collapsed + .fold-content {
            display: none;
        }
        .line.foldable:not(.collapsed) + .fold-content {
            display: block;
        }
        .line.foldable.collapsed .fold-text::after {
            content: " ▶";
        }
        .line.foldable:not(.collapsed) .fold-text::after {
            content: " ▼";
        }
    </style>
</head>
<body>
    <h1>Clipboard Change Details</h1>
    <h2>Summary</h2>
    <pre class="summary">%s</pre>
    <h2>Detailed Diff</h2>
    %s
</body>
</html>
`
	fullHtml := fmt.Sprintf(htmlContent,
		html.EscapeString(summary),
		renderedHtmlDiffContent,
	)

	// Create temporary file and open in browser
	tmpFile, err := os.CreateTemp("", "clipdiff-*.html")
	if err != nil {
		errMsg := fmt.Sprintf("Could not create temporary file. Error: %v", err)
		log.Printf("Error creating temp file for diff view: %v", err)
		ShowAdminNotification(LevelWarn, "Diff View Error", errMsg)
		return
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(fullHtml); err != nil {
		errMsg := fmt.Sprintf("Could not write changes to temporary file. Error: %v", err)
		log.Printf("Error writing to temp file: %v", err)
		ShowAdminNotification(LevelWarn, "Diff View Error", errMsg)
		if errRem := os.Remove(tmpFile.Name()); errRem != nil && !os.IsNotExist(errRem) {
			log.Printf("Error removing temporary file after write error: %s, %v", tmpFile.Name(), errRem)
		}
		return
	}
	if err := tmpFile.Close(); err != nil {
		log.Printf("Error closing temp file after write: %v", err)
	}

	absPath, err := filepath.Abs(tmpFile.Name())
	if err != nil {
		log.Printf("Warning: Could not get absolute path for temp file '%s': %v. Using original.", tmpFile.Name(), err)
		absPath = tmpFile.Name()
	}
	log.Printf("Diff view saved to: %s", absPath)
	if err := OpenFileInDefaultApp(absPath); err != nil {
		errMsg := fmt.Sprintf("Could not open changes in browser. File saved at: %s. Error: %v", absPath, err)
		log.Printf("Error opening diff view in browser: %v", err)
		ShowAdminNotification(LevelWarn, "Diff View Error", errMsg)
	}

	// Clean up temporary file after 1 minute
	go func(pathToDelete string) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("RECOVERED FROM PANIC IN DIFF FILE CLEANUP: %v", r)
			}
		}()
		time.Sleep(1 * time.Minute)
		err := os.Remove(pathToDelete)
		if err != nil && !os.IsNotExist(err) {
			log.Printf("Error deleting temporary diff file %s: %v", pathToDelete, err)
		} else {
			log.Printf("Attempted deletion of temporary diff file: %s", pathToDelete)
		}
	}(absPath)
}
