// ==== internal/clipboard/clipboard.go ====
package clipboard

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/TanaroSch/clipboard-regex-replace/internal/config"
)

// Manager handles clipboard operations and transformations
type Manager struct {
	previousClipboard        string
	lastTransformedClipboard string
	config                   *config.Config // Holds the overall config reference
	onRevertStatusChange     func(bool)
	lastOriginalForDiff      string
	lastModifiedForDiff      string
	resolvedSecrets          map[string]string // Added: Runtime secrets
}

// NewManager creates a new clipboard manager
func NewManager(cfg *config.Config, resolvedSecrets map[string]string, onRevertStatusChange func(bool)) *Manager { // Added resolvedSecrets param
	return &Manager{
		config:               cfg, // Store the main config reference
		resolvedSecrets:      resolvedSecrets, // Store secrets map
		onRevertStatusChange: onRevertStatusChange,
	}
}

// UpdateResolvedSecrets allows updating the secrets map after config reload.
func (m *Manager) UpdateResolvedSecrets(newSecrets map[string]string) { // Added
	m.resolvedSecrets = newSecrets
	log.Println("Clipboard Manager: Updated resolved secrets.")
}

// GetLastDiff returns the last pair of original/modified text for diffing.
func (m *Manager) GetLastDiff() (original string, modified string, ok bool) {
	if m.lastOriginalForDiff != "" || m.lastModifiedForDiff != "" {
		return m.lastOriginalForDiff, m.lastModifiedForDiff, true
	}
	return "", "", false
}

// --- Secret Placeholder Handling ---

var secretPlaceholderRegex = regexp.MustCompile(`\{\{([a-zA-Z0-9_]+)\}\}`)
var ErrSecretNotFound = errors.New("secret placeholder not found in resolved secrets")

// resolvePlaceholders replaces {{placeholder}} with actual secret values.
// Returns the resolved string and an error if any placeholder could not be resolved.
func resolvePlaceholders(text string, secrets map[string]string, escapeForRegex bool) (string, error) {
	var firstError error
	result := secretPlaceholderRegex.ReplaceAllStringFunc(text, func(match string) string {
		// If an error already occurred, stop trying to replace
		if firstError != nil {
			return match
		}

		parts := secretPlaceholderRegex.FindStringSubmatch(match)
		if len(parts) != 2 {
			log.Printf("Internal Error: Failed parsing placeholder match '%s' with regex '%s'", match, secretPlaceholderRegex.String())
			// This indicates a bug in the regex or matching logic, treat as unresolved
			firstError = fmt.Errorf("internal error parsing placeholder match: %s", match)
			return match // Return placeholder unmodified
		}
		name := parts[1]

		secretValue, found := secrets[name]
		if !found {
			// Log the error and set firstError
			log.Printf("Error: Secret placeholder '{{%s}}' found, but secret not loaded/found in map.", name)
			firstError = fmt.Errorf("%w: {{%s}}", ErrSecretNotFound, name) // Use wrapped error
			return match // Return placeholder unmodified
		}

		if escapeForRegex {
			return regexp.QuoteMeta(secretValue)
		}
		return secretValue
	})

	return result, firstError // Return the processed string and the first error encountered (if any)
}

// --- End Secret Placeholder Handling ---

// ProcessClipboard reads, transforms, and pastes clipboard content
func (m *Manager) ProcessClipboard(hotkeyStr string, isReverse bool) (message string, changedForDiff bool) {
	origText, err := clipboard.ReadAll()
	if err != nil {
		log.Printf("Failed to read clipboard: %v", err)
		return "", false
	}

	isNewContent := m.lastTransformedClipboard == "" || origText != m.lastTransformedClipboard
	newText := origText
	totalReplacements := 0
	var activeProfiles []string

	// Apply replacements from all enabled profiles that match this hotkey
	for _, profile := range m.config.Profiles { // Iterate using the config stored in the manager
		if !profile.Enabled {
			continue
		}

		if (profile.Hotkey == hotkeyStr && !isReverse) ||
			(profile.ReverseHotkey == hotkeyStr && isReverse) {
			activeProfiles = append(activeProfiles, profile.Name)
			profileReplacements := 0

			for ruleIndex, rep := range profile.Replacements { // Use index for better logging
				var replaced string
				var replacedCount int
				var errReplace error // Capture errors from replacement functions

				if !isReverse {
					// Pass manager's resolvedSecrets implicitly via method receiver
					replaced, replacedCount, errReplace = m.applyForwardReplacement(newText, rep)
				} else {
					// Pass manager's resolvedSecrets implicitly via method receiver
					replaced, replacedCount, errReplace = m.applyReverseReplacement(newText, rep)
				}

				if errReplace != nil {
					log.Printf("Error applying replacement rule #%d (Profile: %s, Regex: %s): %v. Skipping rule.", ruleIndex+1, profile.Name, rep.Regex, errReplace)
					continue // Skip this rule if secrets couldn't be resolved or regex invalid
				}

				// Only count if the text actually changed
				if replaced != newText {
					// Accumulate counts only if text actually changed for this rule
					if replacedCount > 0 {
						profileReplacements += replacedCount
					} else {
						// If count is 0 but text changed (e.g. empty match replaced), count as 1 change?
						// Let's stick to counting based on regex matches reported by apply* funcs for now.
					}
					totalReplacements += replacedCount // Accumulate total replacements counted
					newText = replaced                 // Update text only if changed
				}
			} // End loop over replacements in profile

			directionText := "forward"
			if isReverse {
				directionText = "reverse"
			}
			if profileReplacements > 0 {
				log.Printf("Applied %d %s replacement(s) from profile '%s'",
					profileReplacements, directionText, profile.Name)
			} else {
                 log.Printf("Profile '%s' (%s) matched hotkey, but no replacements were made.", profile.Name, directionText)
            }
		} // End check for matching hotkey
	} // End loop over profiles

	// --- Temporary clipboard logic ---
	// Use m.config for flags
	if m.config.TemporaryClipboard {
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
			// Enable revert only if a change was made and nothing was stored previously
			if newText != origText && m.previousClipboard == "" {
				m.previousClipboard = origText
				if m.onRevertStatusChange != nil {
					m.onRevertStatusChange(true)
				}
			} else if newText == origText && m.previousClipboard == "" { // No change and nothing stored
				if m.onRevertStatusChange != nil {
					m.onRevertStatusChange(false)
				}
			} // Otherwise, leave revert status as is
		}
	} else if m.previousClipboard != "" {
		// If temporary clipboard got disabled externally (config reload), clear stored original and update UI
		m.previousClipboard = ""
		if m.onRevertStatusChange != nil {
			m.onRevertStatusChange(false)
		}
	}

	// --- Store state for diff *if* changes were actually made ---
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

	// --- Update the clipboard with the replaced text only if it changed ---
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

	// --- Generate notification message if replacements were made and text changed ---
	var baseMessage string // Use a separate var for the core message
	if totalReplacements > 0 && changedForDiff { // Ensure changes happened and were counted
		directionIndicator := ""
		if isReverse {
			directionIndicator = " (reverse)"
		}

		log.Printf("Clipboard updated with %d total replacement(s)%s from profiles: %s",
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

		// Use m.config for flags
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

	} else if changedForDiff && totalReplacements == 0 {
         log.Println("Clipboard text changed, but no specific replacements were counted (e.g., empty match).")
         message = "Clipboard updated. Use Systray Menu to view details." // Generic message if changed but count is 0
    } else {
		log.Println("No regex replacements applied or text did not change.")
		message = "" // No message if no replacements/changes
	}

	// --- Start paste goroutine regardless of replacements ---
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
		// Check config flags *again* inside goroutine using m.config
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
				// Also update diff status in UI? Needs coordination. For now, it updates on next hotkey press.
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
		m.lastTransformedClipboard = originalRestored

		// Also clear the diff state as it's no longer relevant to the restored content
		m.lastOriginalForDiff = ""
		m.lastModifiedForDiff = ""

		// Update UI status for revert option
		if m.onRevertStatusChange != nil {
			m.onRevertStatusChange(false)
		}
		// Update UI status for diff option? Coordinated elsewhere for now.

		return true
	}
	log.Println("No original clipboard content available to restore.")
	return false
}

// applyForwardReplacement handles normal regex-based replacements, now resolving secrets.
// Returns: replaced string, count, error (if secret resolution failed or regex invalid)
func (m *Manager) applyForwardReplacement(text string, rep config.Replacement) (string, int, error) {
	// Resolve secrets first using the manager's map
	resolvedRegex, errRegex := resolvePlaceholders(rep.Regex, m.resolvedSecrets, true)
	resolvedReplaceWith, errReplace := resolvePlaceholders(rep.ReplaceWith, m.resolvedSecrets, false)

	// If either resolution failed, return error immediately
	if errRegex != nil {
		return text, 0, fmt.Errorf("failed to resolve placeholders in regex '%s': %w", rep.Regex, errRegex)
	}
	if errReplace != nil {
		return text, 0, fmt.Errorf("failed to resolve placeholders in replace_with '%s': %w", rep.ReplaceWith, errReplace)
	}

	// Compile the resolved regex pattern
	re, err := regexp.Compile(resolvedRegex)
	if err != nil {
		// Log the specific error
		log.Printf("Invalid resolved regex '%s' (from original: '%s'): %v", resolvedRegex, rep.Regex, err)
		// Return an error indicating compilation failure
		return text, 0, fmt.Errorf("invalid compiled regex from '%s': %w", rep.Regex, err)
	}

	// Find all matches to count accurately *before* replacement
	matchesIndexes := re.FindAllStringIndex(text, -1)
	matchCount := 0
	if matchesIndexes != nil {
		matchCount = len(matchesIndexes)
	}

	// If no matches, return original text immediately
	if matchCount == 0 {
		return text, 0, nil // No matches, no error
	}

	// Apply replacement with or without case preservation using resolvedReplaceWith
	var result string
	if rep.PreserveCase {
		// Use ReplaceAllStringFunc for case preservation
		result = re.ReplaceAllStringFunc(text, func(match string) string {
			// Use the resolved replacement string for case preservation
			return m.preserveCase(match, resolvedReplaceWith)
		})
	} else {
		// Use standard ReplaceAllString
		result = re.ReplaceAllString(text, resolvedReplaceWith)
	}

	// Only return count > 0 if the text actually changed.
	if text == result {
		return text, 0, nil // Text didn't change, no error
	}

	// Return the modified text and the count of matches found
	return result, matchCount, nil
}

// applyReverseReplacement handles reverse replacements, now resolving secrets.
// Returns: replaced string, count, error (if secret resolution failed, source invalid, or regex invalid)
func (m *Manager) applyReverseReplacement(text string, rep config.Replacement) (string, int, error) {
	// --- Resolve Target Word (from replace_with) ---
	resolvedTargetWord, errTarget := resolvePlaceholders(rep.ReplaceWith, m.resolvedSecrets, false)
	if errTarget != nil {
		return text, 0, fmt.Errorf("failed to resolve placeholders in replace_with for reverse target '%s': %w", rep.ReplaceWith, errTarget)
	}
	if resolvedTargetWord == "" {
		log.Printf("Warning: Resolved 'replace_with' is empty for reverse replacement in rule with original regex '%s'. Cannot reverse.", rep.Regex)
		return text, 0, nil // Cannot reverse if target is empty, but not a critical error.
	}

	// --- Resolve Source Word (from reverse_with or derived from regex) ---
	var resolvedSourceWord string
	var errSource error
	if rep.ReverseWith != "" {
		// Resolve placeholders in the specified reverse replacement
		resolvedSourceWord, errSource = resolvePlaceholders(rep.ReverseWith, m.resolvedSecrets, false) // Source word isn't regex usually
	} else {
		// Fall back to extracting from the original forward regex
		rawSourceWord := m.extractFirstAlternative(rep.Regex) // Extract before resolving
		if rawSourceWord == "" {
			log.Printf("Warning: Could not determine raw source word for reverse replacement from regex '%s'. Trying cleaned regex.", rep.Regex)
			rawSourceWord = strings.TrimPrefix(rep.Regex, "(?i)")
			rawSourceWord = strings.Trim(rawSourceWord, "()")
		}
		// Now resolve placeholders in the derived raw source word
		resolvedSourceWord, errSource = resolvePlaceholders(rawSourceWord, m.resolvedSecrets, false)

		// Check if source determination failed or results in empty/same word after resolution
		if resolvedSourceWord == "" {
			log.Printf("Error: Unable to determine a non-empty source word for reverse replacement in rule '%s' after resolving placeholders.", rep.Regex)
			return text, 0, fmt.Errorf("unable to determine non-empty source word for reverse replacement in rule '%s'", rep.Regex)
		}
        // Allow source and target to be the same if preserve_case is involved? Maybe not safe.
        // Let's prevent source == target unless explicitly allowed somehow.
        // if resolvedSourceWord == resolvedTargetWord {
		//	log.Printf("Error: Resolved source word ('%s') is the same as resolved target word ('%s') for reverse replacement in rule '%s'. Cannot reverse.", resolvedSourceWord, resolvedTargetWord, rep.Regex)
		//	return text, 0, fmt.Errorf("resolved source and target are identical for reverse replacement in rule '%s'", rep.Regex)
		//}
	}
	// Check error from resolving source word placeholder itself
	if errSource != nil {
		// Error occurred during placeholder resolution for the source word
		sourceOrigin := rep.ReverseWith
		if sourceOrigin == "" { sourceOrigin = fmt.Sprintf("derived from regex '%s'", rep.Regex)}
		return text, 0, fmt.Errorf("failed to resolve placeholders in source word ('%s') for reverse: %w", sourceOrigin, errSource)
	}


	// --- Compile finder regex for the resolved target word ---
	var findRe *regexp.Regexp
	var err error
	searchPattern := regexp.QuoteMeta(resolvedTargetWord) // Quote meta chars in the resolved target

	if rep.PreserveCase {
		findRe, err = regexp.Compile(`(?i)` + searchPattern)
	} else {
		findRe, err = regexp.Compile(searchPattern)
	}
	if err != nil {
		log.Printf("Error compiling regex for reverse search of resolved target '%s' (from '%s'): %v", resolvedTargetWord, rep.ReplaceWith, err)
		// Return compile error
		return text, 0, fmt.Errorf("failed to compile reverse search regex for target '%s': %w", rep.ReplaceWith, err)
	}

	// Count matches before replacement
	matchesIndexes := findRe.FindAllStringIndex(text, -1)
	matchCount := 0
	if matchesIndexes != nil {
		matchCount = len(matchesIndexes)
	}
	if matchCount == 0 {
		return text, 0, nil // No matches found, no error
	}

	// Perform replacement using ReplaceAllStringFunc to handle case preservation using resolvedSourceWord
	replacedText := findRe.ReplaceAllStringFunc(text, func(match string) string {
		if rep.PreserveCase {
			// Apply the case pattern of the matched text (targetWord instance) to the resolvedSourceWord
			return m.preserveCase(match, resolvedSourceWord)
		}
		// If not preserving case, just return the resolvedSourceWord directly
		return resolvedSourceWord
	})

	// Only return count > 0 if the text actually changed.
	if text == replacedText {
		return text, 0, nil // Text didn't change, no error
	}

	return replacedText, matchCount, nil
}

// --- Helper methods below (no changes needed) ---

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
		// No group found, check if it's a simple alternation without outer parens
		if strings.Contains(regex, "|") && !strings.Contains(regex, "\\|") {
            // Basic split, might be wrong if pipes are escaped later
            parts := strings.SplitN(regex, "|", 2)
            if len(parts) > 0 {
                 return strings.TrimSpace(parts[0])
            }
        }
		// Otherwise return cleaned regex as is
		return strings.TrimSpace(regex)
	}

	// Find the matching closing parenthesis (very basic level matching, ignores nesting/escapes within)
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
		// No matching closing parenthesis found for the first opening one
		return strings.TrimSpace(regex)
	}

	// Extract content within the first top-level parentheses
	groupContent := regex[start+1 : end]

	// Split by the alternation character '|', ignoring escaped pipes \|
	// This requires a more careful split than strings.Split
	var alternatives []string
	current := ""
	escape := false
	parenLevel := 0 // Track nested parentheses within the group
	for _, r := range groupContent {
		if escape {
			current += string(r)
			escape = false
		} else if r == '\\' {
			escape = true
			current += string(r) // Keep the escape char
		} else if r == '(' {
			parenLevel++
			current += string(r)
		} else if r == ')' {
			parenLevel--
			current += string(r)
		} else if r == '|' && parenLevel == 0 { // Split only if not inside nested parens
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
		firstAlt := strings.TrimSpace(alternatives[0])
        // Basic unescaping (e.g., remove \ before | or other metachars if needed)
        // firstAlt = strings.ReplaceAll(firstAlt, "\\|", "|")
        // ... etc.
		return firstAlt
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
			for i := 1; i < len(sourceRunes); i++ { // Check from the second rune
                r := sourceRunes[i]
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
			for _, tr := range targetRunes {
				if unicode.IsLetter(tr) {
					hasTargetLetter = true
					break
				}
			}

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
	// This is often the most useful default.
	if len(targetRunes) > 0 {
		firstSourceRune := sourceRunes[0]
		firstTargetRune := targetRunes[0]

		// Only change case if the first source character is a letter
		if unicode.IsLetter(firstSourceRune) {
			var newFirstTargetRune rune
			if unicode.IsUpper(firstSourceRune) {
				newFirstTargetRune = unicode.ToUpper(firstTargetRune)
			} else { // IsLower
				newFirstTargetRune = unicode.ToLower(firstTargetRune)
			}

			// Construct the result: new first letter + rest of target
			if len(targetRunes) > 1 {
				return string(newFirstTargetRune) + string(targetRunes[1:])
			}
			return string(newFirstTargetRune)
		}
        // If first source char is not a letter, maybe don't change target case?
        // Let's return target as is in this case.
        // return target
	}

	// Fallback: If all else fails (e.g., empty target?), return target unmodified.
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