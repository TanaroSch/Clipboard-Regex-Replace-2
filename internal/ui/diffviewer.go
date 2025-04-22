// internal/ui/diffviewer.go
package ui

import (
	"fmt"
	"html" // Still needed for escaping
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings" // Still needed for SplitAfter, Builder
	"time"

	"github.com/TanaroSch/clipboard-regex-replace/internal/diffutil"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// renderDiffHtml manually builds HTML line-by-line from diff chunks.
func renderDiffHtml(diffs []diffmatchpatch.Diff) string {
	// Rename the variable to avoid conflict with the 'html' package
	var builder strings.Builder // <<< RENAMED variable here
	for _, diff := range diffs {
		text := diff.Text
		lines := strings.SplitAfter(text, "\n")

		for _, line := range lines {
			if line == "" {
				continue
			}
			// Use the html package function correctly
			escapedLine := html.EscapeString(line) // <<< Now calls the package function

			switch diff.Type {
			case diffmatchpatch.DiffInsert:
				// Use the renamed variable 'builder'
				builder.WriteString(fmt.Sprintf("<span class=\"diff-insert\">%s</span>", escapedLine)) // <<< Use builder
			case diffmatchpatch.DiffDelete:
				builder.WriteString(fmt.Sprintf("<span class=\"diff-delete\">%s</span>", escapedLine)) // <<< Use builder
			case diffmatchpatch.DiffEqual:
				builder.WriteString(fmt.Sprintf("<span class=\"diff-equal\">%s</span>", escapedLine)) // <<< Use builder
			}
		}
	}
	return builder.String() // <<< Return from builder
}


// ShowDiffViewer generates an HTML diff view and opens it in the default browser.
// (Rest of the function remains the same as the previous version)
func ShowDiffViewer(original, modified string) {
	log.Println("Generating diff view...")
	diffs, summary := diffutil.GenerateDiffAndSummary(original, modified)
	renderedHtmlDiff := renderDiffHtml(diffs)

	htmlContent := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Clipboard Change Details</title>
    <style>
        body { font-family: sans-serif; margin: 20px; background-color: #f8f8f8; color: #333; }
        h1, h2 { border-bottom: 1px solid #ccc; padding-bottom: 5px; color: #0056b3; }
        pre.summary {
            background-color: #eee; border: 1px solid #ccc; padding: 10px; overflow-x: auto;
            white-space: pre-wrap; word-wrap: break-word; font-family: monospace;
            font-size: 13px; line-height: 1.4;
        }
        div.diff-output {
             margin-top: 15px; border: 1px solid #ddd; padding: 10px; background-color: #fff;
             font-family: monospace; line-height: 1.4; font-size: 14px;
             white-space: pre-wrap; word-wrap: break-word;
         }
        .diff-insert { background-color: #e6ffed; color: #006400; }
        .diff-delete { background-color: #ffeef0; color: #8B0000; text-decoration: line-through; }
        .diff-equal { color: #555; }
    </style>
</head>
<body>
    <h1>Clipboard Change Details</h1>
    <h2>Summary</h2>
    <pre class="summary">%s</pre>
    <h2>Detailed Diff</h2>
    <div class="diff-output">
%s
    </div>
</body>
</html>
`
	fullHtml := fmt.Sprintf(htmlContent,
		html.EscapeString(summary), // Escaping summary is still correct
		renderedHtmlDiff,
	)

	// --- File creation and opening logic remains the same ---
	tmpFile, err := ioutil.TempFile("", "clipdiff-*.html")
	if err != nil {
		log.Printf("Error creating temp file for diff view: %v", err)
		ShowNotification("Diff View Error", "Could not create temporary file.")
		return
	}
	if _, err := tmpFile.WriteString(fullHtml); err != nil {
		log.Printf("Error writing to temp file: %v", err)
		ShowNotification("Diff View Error", "Could not write changes to temporary file.")
		tmpFile.Close()
		if errRem := os.Remove(tmpFile.Name()); errRem != nil {
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
		log.Printf("Error opening diff view in browser: %v", err)
		ShowNotification("Diff View Error", "Could not open changes in browser. File saved at: "+absPath)
		return
	}
	go func(pathToDelete string) {
		time.Sleep(1 * time.Minute)
		err := os.Remove(pathToDelete)
		if err != nil { log.Printf("Error deleting temporary diff file %s: %v", pathToDelete, err)
		} else { log.Printf("Successfully deleted temporary diff file: %s", pathToDelete) }
	}(absPath)
}


// OpenFileInDefaultApp remains the same
func OpenFileInDefaultApp(filePath string) error {
	log.Printf("Executing OpenFileInDefaultApp for path: %s on OS: %s", filePath, runtime.GOOS)
	switch runtime.GOOS {
	case "windows":
		log.Println("Windows: Attempting method: ShellExecuteW API")
		err := windowsOpenFileInDefaultApp(filePath)
		if err == nil {	log.Println("Windows Method (ShellExecuteW) succeeded.") } else { log.Printf("Windows Method (ShellExecuteW) failed: %v", err) }
		return err
	case "darwin":
		cmd := exec.Command("open", filePath)
		log.Printf("macOS - Executing: %s %v", cmd.Path, cmd.Args)
		err := cmd.Start()
		if err != nil {	log.Printf("Failed to start command (%s): %v", cmd.String(), err); return fmt.Errorf("failed to start command (%s): %w", cmd.String(), err) }
		log.Printf("Successfully started command for %s", runtime.GOOS)
		return nil
	default: // Assume Linux or other Unix-like systems
		cmd := exec.Command("xdg-open", filePath)
		log.Printf("Linux/Other - Executing: %s %v", cmd.Path, cmd.Args)
		err := cmd.Start()
        if err != nil {	log.Printf("Failed to start command (%s): %v", cmd.String(), err);	return fmt.Errorf("failed to start command (%s): %w", cmd.String(), err) }
		log.Printf("Successfully started command for %s", runtime.GOOS)
		return nil
	}
}