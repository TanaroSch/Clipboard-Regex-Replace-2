// internal/ui/diffviewer.go
package ui

import (
	"fmt"
	"html"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/TanaroSch/clipboard-regex-replace/internal/diffutil"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// renderUnifiedDiffHtml generates a unified diff view in HTML format,
// including line numbers and static context folding.
func renderUnifiedDiffHtml(diffs []diffmatchpatch.Diff, contextLines int) string {
	var builder strings.Builder
	origLineNum := 1
	modLineNum := 1
	// Minimum number of equal lines required *in the middle* to trigger folding.
	foldThreshold := (contextLines * 2) + 1 // e.g., 3 context + 1 hidden + 3 context = 7

	builder.WriteString(`<pre class="diff-output">`) // Use <pre> for better whitespace handling

	for _, diff := range diffs {
		// Split the segment's text into lines, keeping the newline separators
		segmentLines := strings.SplitAfter(diff.Text, "\n")
		// Remove the potentially empty string after the last newline
		if len(segmentLines) > 0 && segmentLines[len(segmentLines)-1] == "" {
			segmentLines = segmentLines[:len(segmentLines)-1]
		}

		switch diff.Type {
		case diffmatchpatch.DiffEqual:
			if len(segmentLines) >= foldThreshold { // Check if this *entire equal block* is foldable
				// --- Render Folded Block ---
				// 1. Render first 'contextLines'
				for j := 0; j < contextLines; j++ {
					writeDiffLine(&builder, diff.Type, origLineNum, modLineNum, segmentLines[j])
					origLineNum++
					modLineNum++
				}

				// 2. Render fold marker
				skippedLines := len(segmentLines) - (contextLines * 2)
				builder.WriteString(fmt.Sprintf(
					"<div class=\"line foldable\"><span class=\"line-num\">...</span><span class=\"line-num\">...</span><span class=\"line-op\"> </span><span class=\"line-content\">%d lines hidden</span></div>",
					skippedLines))
				origLineNum += skippedLines
				modLineNum += skippedLines

				// 3. Render last 'contextLines'
				for j := len(segmentLines) - contextLines; j < len(segmentLines); j++ {
					writeDiffLine(&builder, diff.Type, origLineNum, modLineNum, segmentLines[j])
					origLineNum++
					modLineNum++
				}
			} else {
				// --- Render Unfolded Equal Block ---
				for _, line := range segmentLines {
					// Only render if the line is not empty (handles potential edge cases)
					if line != "" {
						writeDiffLine(&builder, diff.Type, origLineNum, modLineNum, line)
						origLineNum++
						modLineNum++
					}
				}
			}
		case diffmatchpatch.DiffDelete:
			for _, line := range segmentLines {
				if line != "" {
					writeDiffLine(&builder, diff.Type, origLineNum, 0, line) // 0 for modLineNum
					origLineNum++
				}
			}
		case diffmatchpatch.DiffInsert:
			for _, line := range segmentLines {
				if line != "" {
					writeDiffLine(&builder, diff.Type, 0, modLineNum, line) // 0 for origLineNum
					modLineNum++
				}
			}
		}
	}

	builder.WriteString(`</pre>`)
	return builder.String()
}

// writeDiffLine formats and writes a single line of the diff to the builder.
// It now handles the line number formatting and content escaping.
func writeDiffLine(builder *strings.Builder, op diffmatchpatch.Operation, origNum, modNum int, lineText string) {
	lineClass := ""
	opChar := " " // Default op character for equal lines

	switch op {
	case diffmatchpatch.DiffDelete:
		lineClass = "diff-delete"
		opChar = "-"
	case diffmatchpatch.DiffInsert:
		lineClass = "diff-insert"
		opChar = "+"
	case diffmatchpatch.DiffEqual:
		lineClass = "diff-equal"
	}

	origNumStr := ""
	if origNum > 0 {
		origNumStr = fmt.Sprintf("%d", origNum)
	}
	modNumStr := ""
	if modNum > 0 {
		modNumStr = fmt.Sprintf("%d", modNum)
	}

	// Escape content and handle spaces for <pre> context
	escapedLine := html.EscapeString(lineText)
	// Preserve spaces by replacing them with  , but handle potential trailing newline
	endsWithNewline := strings.HasSuffix(escapedLine, "\n")
	contentToRender := escapedLine
	if endsWithNewline {
		contentToRender = strings.ReplaceAll(escapedLine[:len(escapedLine)-1], " ", " ") + "\n"
	} else {
		contentToRender = strings.ReplaceAll(escapedLine, " ", " ")
	}
	// If the content is just a newline, render it as such to maintain line height
	if contentToRender == "\n" {
		contentToRender = " \n"
	}


	// Render the line as a div
	builder.WriteString(fmt.Sprintf(
		"<div class=\"line %s\"><span class=\"line-num orig-num\">%s</span><span class=\"line-num mod-num\">%s</span><span class=\"line-op\">%s</span><span class=\"line-content\">%s</span></div>",
		lineClass, origNumStr, modNumStr, opChar, contentToRender,
	))
}


// ShowDiffViewer generates an HTML diff view and opens it in the default browser.
// (CSS and overall structure remain the same as the previous corrected version)
func ShowDiffViewer(original, modified string, contextLines int) {
	log.Println("Generating enhanced diff view...")
	diffs, summary := diffutil.GenerateDiffAndSummary(original, modified)

	// Use provided contextLines (or default if <= 0)
	if contextLines <= 0 {
		contextLines = 3 // Fallback to default
	}
	renderedHtmlDiffContent := renderUnifiedDiffHtml(diffs, contextLines)

	// HTML structure and CSS remain the same as the previous successful unified diff attempt
	htmlContent := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Clipboard Change Details</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            margin: 15px;
            background-color: #f8f9fa;
            color: #212529;
            line-height: 1.5;
        }
        h1, h2 {
            border-bottom: 1px solid #dee2e6;
            padding-bottom: 8px;
            color: #0d6efd; /* Bootstrap blue */
            margin-top: 20px;
            margin-bottom: 15px;
        }
        pre.summary {
            background-color: #e9ecef;
            border: 1px solid #ced4da;
            padding: 10px 15px;
            overflow-x: auto;
            white-space: pre-wrap;
            word-wrap: break-word;
            font-family: SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
            font-size: 0.875em;
            line-height: 1.5;
            border-radius: 4px;
            margin-bottom: 20px;
        }
        pre.diff-output {
            font-family: SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
            font-size: 0.9em;
            line-height: 1.4; /* Adjust line height for pre */
            border: 1px solid #dee2e6;
            background-color: #fff;
            padding: 10px;
            border-radius: 4px;
            overflow-x: auto; /* Add horizontal scroll if needed */
            white-space: pre; /* Important for unified diff */
        }
        .line {
            display: flex; /* Arrange spans horizontally */
            min-height: 1.4em; /* Ensure lines have height even if empty */
        }
        .line-num {
            display: inline-block;
            width: 35px; /* Width for line numbers */
            padding-right: 10px;
            text-align: right;
            color: #6c757d; /* Grey */
            user-select: none; /* Prevent selecting line numbers */
            flex-shrink: 0; /* Don't shrink line number columns */
        }
        .line-op {
             display: inline-block;
             width: 15px; /* Width for +/- indicator */
             text-align: center;
             color: #6c757d;
             user-select: none;
             font-weight: bold;
             flex-shrink: 0;
             margin-right: 10px;
        }
        .line-content {
            display: inline-block;
            white-space: pre-wrap; /* Allow content wrapping */
            word-break: break-all; /* Break long words if needed */
            flex-grow: 1; /* Allow content to take remaining space */
        }

        /* Line type specific styling */
        .line.diff-insert { background-color: #e6ffed; }
        .line.diff-insert .line-op { color: #198754; } /* Green op */
        .line.diff-insert .line-content { color: #198754; } /* Green text */

        .line.diff-delete { background-color: #ffeef0; }
        .line.diff-delete .line-op { color: #dc3545; } /* Red op */
        .line.diff-delete .line-content { color: #dc3545; text-decoration: line-through; } /* Red text */

        .line.diff-equal .line-content { color: #495057; } /* Dark grey */

        .line.foldable {
            background-color: #e9ecef;
            justify-content: center; /* Center the "..." */
            color: #6c757d;
            font-style: italic;
            min-height: 1.8em;
            align-items: center;
        }
        .line.foldable .line-num, .line.foldable .line-op {
             display: none; /* Hide numbers/op on folded line */
        }
         .line.foldable .line-content{
            text-align: center;
            flex-grow: 1; /* Make sure content takes full width */
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
		renderedHtmlDiffContent, // Insert the generated diff content
	)

	// --- File creation and opening logic (remains the same) ---
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


// OpenFileInDefaultApp remains the same
func OpenFileInDefaultApp(filePath string) error {
	log.Printf("Executing OpenFileInDefaultApp for path: %s on OS: %s", filePath, runtime.GOOS)
	switch runtime.GOOS {
	case "windows":
		log.Println("Windows: Attempting method: ShellExecuteW API")
		err := windowsOpenFileInDefaultApp(filePath)
		if err == nil {
			log.Println("Windows Method (ShellExecuteW) succeeded.")
		} else {
			log.Printf("Windows Method (ShellExecuteW) failed: %v", err)
		}
		return err
	case "darwin":
		cmd := exec.Command("open", filePath)
		log.Printf("macOS - Executing: %s %v", cmd.Path, cmd.Args)
		err := cmd.Start()
		if err != nil {
			log.Printf("Failed to start command (%s): %v", cmd.String(), err)
			return fmt.Errorf("failed to start command (%s): %w", cmd.String(), err)
		}
		log.Printf("Successfully started command for %s", runtime.GOOS)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("RECOVERED FROM PANIC IN MACOS CMD WAIT: %v", r)
				}
			}()
			_ = cmd.Wait()
		}()
		return nil
	default: // Assume Linux or other Unix-like systems
		cmd := exec.Command("xdg-open", filePath)
		log.Printf("Linux/Other - Executing: %s %v", cmd.Path, cmd.Args)
		err := cmd.Start()
		if err != nil {
			log.Printf("Failed to start command (%s): %v", cmd.String(), err)
			return fmt.Errorf("failed to start command (%s): %w", cmd.String(), err)
		}
		log.Printf("Successfully started command for %s", runtime.GOOS)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("RECOVERED FROM PANIC IN LINUX CMD WAIT: %v", r)
				}
			}()
			_ = cmd.Wait()
		}()
		return nil
	}
}