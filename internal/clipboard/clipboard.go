// ==== internal/clipboard/clipboard.go ====
package clipboard

import (
	"fmt"
	"log"
	// "os/exec" // Removed unused import
	"regexp"
	"strings"
	// "syscall" // Removed unused import
	"time"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/TanaroSch/clipboard-regex-replace/internal/config"
)

// Manager handles clipboard operations and transformations
type Manager struct {
	previousClipboard       string
	lastTransformedClipboard string
	config                  *config.Config
	onRevertStatusChange    func(bool)
	// Store last diff state *within* the manager
	lastOriginalForDiff string
	lastModifiedForDiff string
}

// NewManager creates a new clipboard manager
func NewManager(cfg *config.Config, onRevertStatusChange func(bool)) *Manager {
	return &Manager{
		config:               cfg,
		onRevertStatusChange: onRevertStatusChange,
	}
}

// GetLastDiff returns the last pair of original/modified text for diffing.
func (m *Manager) GetLastDiff() (original string, modified string, ok bool) {
	// Check if *an operation* resulted in a state to compare.
	// This is true if either original or modified has been set (meaning changedForDiff was true).
	if m.lastOriginalForDiff != "" || m.lastModifiedForDiff != "" {
		// We rely on ProcessClipboard logic: these are only set if origText != newText.
		return m.lastOriginalForDiff, m.lastModifiedForDiff, true
	}
	return "", "", false
}


// ProcessClipboard reads, transforms, and pastes clipboard content
// Returns:
// - notification message string
// - boolean indicating if changes were made for diff purposes
func (m *Manager) ProcessClipboard(hotkeyStr string, isReverse bool) (message string, changedForDiff bool) {
	// Read the current clipboard content
	origText, err := clipboard.ReadAll()
	if err != nil {
		log.Printf("Failed to read clipboard: %v", err)
		return "", false // Return false for changedForDiff
	}

	// Determine if this is new content or our previously transformed content
	isNewContent := m.lastTransformedClipboard == "" || origText != m.lastTransformedClipboard

	// Start with original text for transformation
	newText := origText
	totalReplacements := 0

	// Track which profiles are being used
	var activeProfiles []string

	// Apply replacements from all enabled profiles that match this hotkey
	for _, profile := range m.config.Profiles {
		if !profile.Enabled {
			continue
		}

		// Check if this profile matches the pressed hotkey
		if (profile.Hotkey == hotkeyStr && !isReverse) ||
			(profile.ReverseHotkey == hotkeyStr && isReverse) {
			activeProfiles = append(activeProfiles, profile.Name)
			profileReplacements := 0

			// Apply each regex replacement rule from this profile
			for _, rep := range profile.Replacements {
				var replaced string
				var replacedCount int

				if !isReverse {
					// Forward replacement
					replaced, replacedCount = m.applyForwardReplacement(newText, rep)
				} else {
					// Reverse replacement
					replaced, replacedCount = m.applyReverseReplacement(newText, rep)
				}

				// Only count if the text actually changed
				if replaced != newText {
					// Accumulate counts only if text actually changed for this rule
					if replacedCount > 0 {
                         profileReplacements += replacedCount // Or just += 1 if we count rules that changed text? Let's count replacements made.
                    } else {
                         // If replacedCount was 0 but text changed (e.g. empty match replaced), count as 1 change?
                         // For simplicity, let's stick to counting based on regex matches reported by apply* funcs.
                         // We need apply* funcs to be accurate about count > 0 only if change is possible.
                    }
					totalReplacements += replacedCount // Accumulate total replacements counted
					newText = replaced                 // Update text only if changed
				}
			}

			directionText := "forward"
			if isReverse {
				directionText = "reverse"
			}
			if profileReplacements > 0 {
				log.Printf("Applied %d %s replacements from profile '%s'",
					profileReplacements, directionText, profile.Name)
			}
		}
	}

	// Handle temporary clipboard storage if needed
	if m.config.TemporaryClipboard {
        // Store original if 1) it's new content and changed, OR 2) it's old content but we still have a previousClipboard stored
		if isNewContent && newText != origText {
			m.previousClipboard = origText
			if m.onRevertStatusChange != nil {
				m.onRevertStatusChange(true) // Enable revert option
			}
        } else if !isNewContent && m.previousClipboard != "" {
            // If processing already transformed text, keep the existing previousClipboard and revert status active
			if m.onRevertStatusChange != nil {
				m.onRevertStatusChange(true)
			}
        } else {
            // No change, or new content is same as old, disable revert if it was enabled previously without a stored value
            // However, if previousClipboard *is* set, don't disable revert just because *this* run made no changes.
            // Revert should only be disabled when explicitly reverted or temp clipboard turned off.
            // Let's simplify: Enable revert if temp is on and a change was made *unless* already enabled.
             if newText != origText && m.previousClipboard == "" { // Store only if not already stored
                 m.previousClipboard = origText
                 if m.onRevertStatusChange != nil { m.onRevertStatusChange(true) }
             } else if newText == origText && m.previousClipboard == "" { // No change and nothing stored
                 if m.onRevertStatusChange != nil { m.onRevertStatusChange(false) }
             } // Otherwise, leave revert status as is
        }
	} else if m.previousClipboard != "" {
        // If temporary clipboard got disabled externally (config reload), clear stored original and update UI
        m.previousClipboard = ""
         if m.onRevertStatusChange != nil {
            m.onRevertStatusChange(false)
        }
    }


	// *** Store state for diff *if* changes were actually made ***
	changedForDiff = (origText != newText) // The most reliable check
	if changedForDiff {
		m.lastOriginalForDiff = origText
		m.lastModifiedForDiff = newText
		log.Printf("Stored original and modified text for diff view.")
	} else {
		// If no changes, clear the diff state
		m.lastOriginalForDiff = ""
		m.lastModifiedForDiff = ""
		log.Printf("No changes made, cleared diff state.")
	}


	// Update the clipboard with the replaced text only if it changed
	if changedForDiff {
		if err := clipboard.WriteAll(newText); err != nil {
			log.Printf("Failed to write to clipboard: %v", err)
			m.lastOriginalForDiff = "" // Clear diff state on error
			m.lastModifiedForDiff = ""
			return "", false // Return false for changedForDiff
		}
		// Track what was just placed in the clipboard
		m.lastTransformedClipboard = newText
	} else {
		// If no change, ensure lastTransformed is same as original read
		m.lastTransformedClipboard = origText
	}


	// Generate notification message if replacements were made and text changed
	var baseMessage string // Use a separate var for the core message
	if totalReplacements > 0 && changedForDiff { // Ensure changes happened and were counted
		directionIndicator := ""
		if isReverse {
			directionIndicator = " (reverse)"
		}

		log.Printf("Clipboard updated with %d total replacements%s from profiles: %s",
			totalReplacements, directionIndicator, strings.Join(activeProfiles, ", "))

		profileNames := strings.Join(activeProfiles, ", ")
		profilePart := ""
		if len(activeProfiles) > 1 {
            profilePart = fmt.Sprintf(" from profiles: %s", profileNames)
        } else if len(activeProfiles) == 1 {
            profilePart = fmt.Sprintf(" from profile: %s", profileNames)
        }

		baseMessage = fmt.Sprintf("%d replacement(s)%s applied%s.",
            totalReplacements, directionIndicator, profilePart)


		if m.config.TemporaryClipboard && m.previousClipboard != "" { // Check if something is stored
			if m.config.AutomaticReversion {
				baseMessage += " Clipboard will be automatically reverted after paste."
			} else if m.config.RevertHotkey != "" {
				baseMessage += fmt.Sprintf(" Press %s or use Systray Menu to revert.", m.config.RevertHotkey)
			} else {
				baseMessage += " Use Systray Menu to revert."
			}
		}
		// Append note about viewing changes
		message = baseMessage + " Use Systray Menu to view details."

	} else {
		log.Println("No regex replacements applied or text did not change.")
		message = "" // No message if no replacements/changes
	}

	// Start paste goroutine regardless of replacements (pastes the current clipboard content)
	go func() {
		// Important: Recover from any panics so we don't crash
		defer func() {
			if r := recover(); r != nil {
				log.Printf("RECOVERED FROM PANIC IN PASTE GOROUTINE: %v", r)
			}
		}()

		log.Println("Starting paste operation in separate goroutine...")

		// Delay before pasting to allow clipboard system and target app to be ready
		time.Sleep(400 * time.Millisecond)

		// Try to paste the content *currently* in the clipboard (which is newText)
		simulatePlatformPaste() // Call the platform-specific paste function

		// Handle automatic reversion *after* paste attempt if enabled
		// Check config flags *again* inside goroutine
		if m.config.TemporaryClipboard && m.config.AutomaticReversion && m.previousClipboard != "" {
			// Delay *after* paste simulation
			time.Sleep(300 * time.Millisecond)

			// Restore original clipboard
			if err := clipboard.WriteAll(m.previousClipboard); err != nil {
				log.Printf("Failed to automatically restore original clipboard: %v", err)
			} else {
				log.Println("Original clipboard content automatically restored after paste.")
				currentStored := m.previousClipboard // Capture before clearing
				// Clear the stored original and update UI status
				m.previousClipboard = ""
				m.lastTransformedClipboard = currentStored // Set last transformed to what was restored
				// Clear diff state too
				m.lastOriginalForDiff = ""
				m.lastModifiedForDiff = ""

				if m.onRevertStatusChange != nil {
					// Run callback in a separate goroutine to avoid blocking paste thread if UI is slow
					go m.onRevertStatusChange(false)
				}
				// Also update diff status in UI (needs another callback or direct access)
                // For now, diff status only updates on next hotkey press.
			}
		}

		log.Println("Paste goroutine potentially completed.")
	}()

	// Return message and diff status
	return message, changedForDiff
}

// RestoreOriginalClipboard reverts to the previous clipboard content
func (m *Manager) RestoreOriginalClipboard() bool {
	if m.previousClipboard != "" {
		// Read current clipboard content (optional, for logging comparison)
		// currentClipboard, errRead := clipboard.ReadAll() // Removed unused variable
        _, errRead := clipboard.ReadAll()
		if errRead != nil {
			log.Printf("Warning: Failed to read current clipboard before reverting: %v", errRead)
			// Decide whether to proceed anyway or return false. Let's proceed.
		}

		// Write the stored original content back to the clipboard
		if err := clipboard.WriteAll(m.previousClipboard); err != nil {
			log.Printf("Failed to restore original clipboard: %v", err)
			return false
		}

		log.Println("Original clipboard content restored.")

		// Clear the stored original clipboard content
		originalRestored := m.previousClipboard
		m.previousClipboard = ""

		// Update the 'last transformed' state to reflect the restored content
		// This prevents immediately re-storing the just-restored content as 'original' on next trigger
		m.lastTransformedClipboard = originalRestored

		// Also clear the diff state as it's no longer relevant to the restored content
		m.lastOriginalForDiff = ""
		m.lastModifiedForDiff = ""

		// Update UI status for revert option
		if m.onRevertStatusChange != nil {
			m.onRevertStatusChange(false)
		}
		// Update UI status for diff option (since state is cleared)
		// This ideally needs another callback passed to the manager, or direct UI access.
		// For now, diff button state will only update on the *next* hotkey press.

		return true
	}
	log.Println("No original clipboard content available to restore.")
	return false
}


// applyForwardReplacement handles normal regex-based replacements
func (m *Manager) applyForwardReplacement(text string, rep config.Replacement) (string, int) {
	// Compile the regex pattern
	re, err := regexp.Compile(rep.Regex)
	if err != nil {
		log.Printf("Invalid regex '%s': %v", rep.Regex, err)
		return text, 0 // Return original text on error
	}

	// Find all matches to count accurately *before* replacement
	matchesIndexes := re.FindAllStringIndex(text, -1)
	matchCount := 0
	if matchesIndexes != nil {
		matchCount = len(matchesIndexes)
	}

	// If no matches, return original text immediately
	if matchCount == 0 {
		return text, 0
	}

	// Apply replacement with or without case preservation
	var result string
	if rep.PreserveCase {
		// Use ReplaceAllStringFunc for case preservation
		result = re.ReplaceAllStringFunc(text, func(match string) string {
			return m.preserveCase(match, rep.ReplaceWith)
		})
	} else {
		// Use standard ReplaceAllString
		result = re.ReplaceAllString(text, rep.ReplaceWith)
	}

    // Only return count > 0 if the text actually changed.
    if text == result {
        return text, 0
    }

	// Return the modified text and the count of matches found
	return result, matchCount
}

// applyReverseReplacement handles reverse replacements
func (m *Manager) applyReverseReplacement(text string, rep config.Replacement) (string, int) {
    // Target word to find (the result of the forward replacement)
    targetWord := rep.ReplaceWith
    if targetWord == "" {
        log.Printf("Warning: 'replace_with' is empty for reverse replacement in regex '%s'. Cannot reverse.", rep.Regex)
        return text, 0 // Cannot reverse if target is empty
    }

    // Word to replace with (original text fragment)
    var sourceWord string
    if rep.ReverseWith != "" {
        // Use the specified reverse replacement if provided
        sourceWord = rep.ReverseWith
    } else {
        // Fall back to extracting the first alternative from the forward regex
        sourceWord = m.extractFirstAlternative(rep.Regex)
        if sourceWord == "" {
             log.Printf("Warning: Could not determine source word for reverse replacement from regex '%s'. Trying regex itself.", rep.Regex)
             // As a fallback, use the cleaned regex itself. This might be wrong.
             cleanedRegex := strings.TrimPrefix(rep.Regex, "(?i)")
             cleanedRegex = strings.Trim(cleanedRegex, "()") // Basic cleaning
             sourceWord = cleanedRegex
             if sourceWord == "" || sourceWord == targetWord { // Avoid replacing with empty or same word
                 log.Printf("Error: Cannot perform reverse replacement for rule with regex '%s' - unable to determine valid source.", rep.Regex)
                 return text, 0
             }
        }
    }

    // Compile a regex to find the targetWord, considering case preservation flag
    var findRe *regexp.Regexp
    var err error
    searchPattern := regexp.QuoteMeta(targetWord) // Quote meta characters in the target word

    if rep.PreserveCase {
        // Case-insensitive search
         findRe, err = regexp.Compile(`(?i)` + searchPattern)
    } else {
        // Case-sensitive search
        findRe, err = regexp.Compile(searchPattern)
    }

    if err != nil {
        log.Printf("Error compiling regex for reverse search of '%s': %v", targetWord, err)
        return text, 0
    }

    // Count matches before replacement
	matchesIndexes := findRe.FindAllStringIndex(text, -1)
	matchCount := 0
	if matchesIndexes != nil {
		matchCount = len(matchesIndexes)
	}
    if matchCount == 0 {
        return text, 0 // No matches found
    }

    // Perform replacement using ReplaceAllStringFunc to handle case preservation
    replacedText := findRe.ReplaceAllStringFunc(text, func(match string) string {
        if rep.PreserveCase {
            // Apply the case pattern of the matched text (targetWord instance) to the sourceWord
            return m.preserveCase(match, sourceWord)
        }
        // If not preserving case, just return the sourceWord directly
        return sourceWord
    })

    // Only return count > 0 if the text actually changed.
    if text == replacedText {
        return text, 0
    }

    return replacedText, matchCount
}


// simulatePaste is now just a placeholder comment. The actual call is simulatePlatformPaste().
// func (m *Manager) simulatePaste() { ... }

// pasteClipboardContent is a wrapper for the paste simulation.
// Renaming helps clarify that it triggers the OS-specific paste simulation.
// We call the platform specific function directly now.
// func (m *Manager) pasteClipboardContent() {
// 	simulatePlatformPaste() // Call platform specific implementation
// }


// Helper methods below

// isWord checks if a token is primarily a word (alphanumeric + underscore)
func (m *Manager) isWord(token string) bool {
	if len(token) == 0 {
		return false
	}
	for _, r := range token {
		// Allow letters, numbers, and underscore for typical variable/word definition
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' {
			return false
		}
	}
	return true
}

// extractFirstAlternative attempts to extract the first pattern from an alternation `(a|b|c)` in a regex.
func (m *Manager) extractFirstAlternative(regex string) string {
	// Remove common flags like (?i) at the start
	if i := strings.Index(regex, ")"); i > 0 && strings.HasPrefix(regex, "(?") {
		regex = regex[i+1:]
	}

	// Find the first opening parenthesis that isn't escaped
	start := -1
	for i := 0; i < len(regex); i++ {
		if regex[i] == '(' && (i == 0 || regex[i-1] != '\\') {
			start = i
			break
		}
	}

	if start == -1 {
		// No group found, return the cleaned regex as is? Or empty?
		return strings.TrimSpace(regex)
	}

	// Find the matching closing parenthesis (very basic level matching, ignores nesting/escapes)
	end := -1
	level := 0
	for i := start; i < len(regex); i++ {
		if regex[i] == '(' && (i == 0 || regex[i-1] != '\\') {
			level++
		} else if regex[i] == ')' && (i == 0 || regex[i-1] != '\\') {
			level--
			if level == 0 {
				end = i
				break
			}
		}
	}

	if end == -1 {
		// No matching closing parenthesis found
		return strings.TrimSpace(regex)
	}

	// Extract content within the first top-level parentheses
	groupContent := regex[start+1 : end]

	// Split by the alternation character '|', ignoring escaped pipes \|
	// This requires a more careful split than strings.Split
    var alternatives []string
    current := ""
    escape := false
    for _, r := range groupContent {
        if escape {
            current += string(r)
            escape = false
        } else if r == '\\' {
            escape = true
            current += string(r) // Keep the escape char for now? Or handle later? Let's keep it.
        } else if r == '|' {
            alternatives = append(alternatives, current)
            current = ""
        } else {
            current += string(r)
        }
    }
    alternatives = append(alternatives, current) // Add the last part


	if len(alternatives) > 0 {
		// Return the first part, trimmed of whitespace
		// Also potentially unescape characters if needed, but likely not required for simple cases
		return strings.TrimSpace(alternatives[0])
	}

	// If no alternatives found within the group, return the group content itself
	return strings.TrimSpace(groupContent)
}


// preserveCase applies the case pattern from source to target string.
func (m *Manager) preserveCase(source, target string) string {
	// If source or target is empty, nothing to base case on, return target.
	if len(source) == 0 || len(target) == 0 {
		return target
	}

	sourceRunes := []rune(source)
	targetRunes := []rune(target)

	// 1. All Lowercase: If source is all lowercase (considering only letters).
	isSourceLower := true
	hasSourceLetter := false
	for _, r := range sourceRunes {
		if unicode.IsLetter(r) {
			hasSourceLetter = true
			if !unicode.IsLower(r) {
				isSourceLower = false
				break
			}
		}
	}
	if hasSourceLetter && isSourceLower {
		return strings.ToLower(target)
	}

	// 2. All Uppercase: If source is all uppercase (considering only letters).
	isSourceUpper := true
	hasSourceLetter = false // Reset for upper check
	for _, r := range sourceRunes {
		if unicode.IsLetter(r) {
			hasSourceLetter = true
			if !unicode.IsUpper(r) {
				isSourceUpper = false
				break
			}
		}
	}
	if hasSourceLetter && isSourceUpper {
		return strings.ToUpper(target)
	}

	// 3. Title Case / First Letter Upper: Source starts upper, rest lower (letters only).
	if len(sourceRunes) > 0 && unicode.IsUpper(sourceRunes[0]) {
		isSourceTitle := true
		if len(sourceRunes) > 1 {
			subsequentAreLower := true
			for _, r := range sourceRunes[1:] {
				if unicode.IsLetter(r) && !unicode.IsLower(r) {
					subsequentAreLower = false
					break
				}
			}
			if !subsequentAreLower {
				isSourceTitle = false
			}
		}
		// Apply Title Case to target if source matches pattern and target has letters
		if isSourceTitle {
            hasTargetLetter := false
            for _, tr := range targetRunes { if unicode.IsLetter(tr) { hasTargetLetter = true; break } }

			if hasTargetLetter {
				res := string(unicode.ToUpper(targetRunes[0]))
				if len(targetRunes) > 1 {
					res += strings.ToLower(string(targetRunes[1:]))
				}
				return res
			}
		}
	}

	// 4. PascalCase/camelCase heuristic or Default: Match first letter case, keep rest of target's case.
	if len(targetRunes) > 0 {
		firstSourceRune := sourceRunes[0]
		firstTargetRune := targetRunes[0]

		// Only change case if the first source character is a letter
		if unicode.IsLetter(firstSourceRune) {
			var newFirstTargetRune rune
			if unicode.IsUpper(firstSourceRune) {
				newFirstTargetRune = unicode.ToUpper(firstTargetRune)
			} else { // IsLower or not a letter (e.g. number, symbol) -> make target lower
				newFirstTargetRune = unicode.ToLower(firstTargetRune)
			}

            // Construct the result: new first letter + rest of target
            if len(targetRunes) > 1 {
                return string(newFirstTargetRune) + string(targetRunes[1:])
            }
            return string(newFirstTargetRune)
        }
	}

	// Fallback: If all else fails (e.g., source starts with non-letter), return target unmodified.
	return target
}


// hasInternalCapitalization checks if a string has uppercase letters after the first position
func (m *Manager) hasInternalCapitalization(s string) bool {
	runes := []rune(s)
	if len(runes) <= 1 {
		return false // Cannot have internal capitalization with 0 or 1 chars
	}
	for i := 1; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) {
			return true
		}
	}
	return false
}