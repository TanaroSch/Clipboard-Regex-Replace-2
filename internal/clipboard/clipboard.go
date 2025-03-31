package clipboard

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"syscall"
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
}

// NewManager creates a new clipboard manager
func NewManager(cfg *config.Config, onRevertStatusChange func(bool)) *Manager {
	return &Manager{
		config:               cfg,
		onRevertStatusChange: onRevertStatusChange,
	}
}

// ProcessClipboard reads, transforms, and pastes clipboard content
// Returns a notification message if replacements were made
func (m *Manager) ProcessClipboard(hotkeyStr string, isReverse bool) string {
	// Read the current clipboard content
	origText, err := clipboard.ReadAll()
	if err != nil {
		log.Printf("Failed to read clipboard: %v", err)
		return "" // Early return needs to return a string
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

				newText = replaced
				profileReplacements += replacedCount
				totalReplacements += replacedCount
			}

			directionText := "forward"
			if isReverse {
				directionText = "reverse"
			}

			log.Printf("Applied %d %s replacements from profile '%s'",
				profileReplacements, directionText, profile.Name)
		}
	}

	// Handle temporary clipboard storage if needed
	if m.config.TemporaryClipboard && (isNewContent || m.previousClipboard == "") {
		m.previousClipboard = origText
		// Enable revert option
		if m.onRevertStatusChange != nil {
			m.onRevertStatusChange(true)
		}
	}

	// Update the clipboard with the replaced text
	if err := clipboard.WriteAll(newText); err != nil {
		log.Printf("Failed to write to clipboard: %v", err)
		return "" // Early return needs to return a string
	}

	// Track what was just placed in the clipboard
	m.lastTransformedClipboard = newText

	// Generate notification message if replacements were made
	var message string
	if totalReplacements > 0 {
		directionIndicator := ""
		if isReverse {
			directionIndicator = " (reverse)"
		}

		log.Printf("Clipboard updated with %d total replacements%s from profiles: %s",
			totalReplacements, directionIndicator, strings.Join(activeProfiles, ", "))

		if len(activeProfiles) > 1 {
			message = fmt.Sprintf("%d replacements%s applied from profiles: %s",
				totalReplacements, directionIndicator, strings.Join(activeProfiles, ", "))
		} else {
			message = fmt.Sprintf("%d replacements%s applied from profile: %s",
				totalReplacements, directionIndicator, activeProfiles[0])
		}

		if m.config.TemporaryClipboard {
			if m.config.AutomaticReversion {
				message += ". Clipboard will be automatically reverted after paste."
			} else if m.config.RevertHotkey != "" {
				message += fmt.Sprintf(". Press %s to revert or use system tray menu.", m.config.RevertHotkey)
			} else {
				message += ". Original text stored for manual reversion."
			}
		}
	} else {
		log.Println("No regex replacements applied; will paste original text.")
		// Don't return early - we still want to paste the original text
	}

	// Short delay to allow clipboard update
	// time.Sleep(20 * time.Millisecond)
	// m.pasteClipboardContent()

	// Handle automatic reversion after paste if enabled
	// if m.config.TemporaryClipboard && m.config.AutomaticReversion && m.previousClipboard != "" {
	// 	// Give a small delay after paste to ensure the paste operation completes
	// 	time.Sleep(50 * time.Millisecond)

	// 	// Restore original clipboard
	// 	if err := clipboard.WriteAll(m.previousClipboard); err != nil {
	// 		log.Printf("Failed to automatically restore original clipboard: %v", err)
	// 	} else {
	// 		log.Println("Original clipboard content automatically restored after paste.")
	// 		// Update revert status
	// 		m.previousClipboard = ""
	// 		if m.onRevertStatusChange != nil {
	// 			m.onRevertStatusChange(false)
	// 		}
	// 	}
	// }

	// Always start paste goroutine regardless of replacements
	go func() {
		// Important: Recover from any panics so we don't crash
		defer func() {
			if r := recover(); r != nil {
				log.Printf("RECOVERED FROM PANIC IN PASTE GOROUTINE: %v", r)
			}
		}()
		
		log.Println("Starting paste operation in separate goroutine...")
		
		// Important: Add a delay before pasting to give the UI time to update
		time.Sleep(400 * time.Millisecond)
		
		// Try to paste
		m.pasteClipboardContent()
		
		// Handle automatic reversion after paste if enabled
		if m.config.TemporaryClipboard && m.config.AutomaticReversion && m.previousClipboard != "" {
			// Give a small delay after paste to ensure the paste operation completes
			time.Sleep(300 * time.Millisecond)
			
			// Restore original clipboard
			if err := clipboard.WriteAll(m.previousClipboard); err != nil {
				log.Printf("Failed to automatically restore original clipboard: %v", err)
			} else {
				log.Println("Original clipboard content automatically restored after paste.")
				// Update revert status
				m.previousClipboard = ""
				if m.onRevertStatusChange != nil {
					m.onRevertStatusChange(false)
				}
			}
		}
		
		log.Println("Paste goroutine completed successfully.")
	}()
	
	// Return message for notification
	return message
}

// RestoreOriginalClipboard reverts to the previous clipboard content
func (m *Manager) RestoreOriginalClipboard() bool {
	if m.previousClipboard != "" {
		if err := clipboard.WriteAll(m.previousClipboard); err != nil {
			log.Printf("Failed to restore original clipboard: %v", err)
			return false
		}
		
		log.Println("Original clipboard content restored.")

		// Clear the previous clipboard
		m.previousClipboard = ""
		
		// Update UI status
		if m.onRevertStatusChange != nil {
			m.onRevertStatusChange(false)
		}
		
		return true
	}
	return false
}

// applyForwardReplacement handles normal regex-based replacements
func (m *Manager) applyForwardReplacement(text string, rep config.Replacement) (string, int) {
	// Compile the regex pattern
	re, err := regexp.Compile(rep.Regex)
	if err != nil {
		log.Printf("Invalid regex '%s': %v", rep.Regex, err)
		return text, 0
	}

	// Count matches before replacement
	matches := re.FindAllStringIndex(text, -1)
	matchCount := 0
	if matches != nil {
		matchCount = len(matches)
	}

	// Apply replacement with or without case preservation
	var result string
	if rep.PreserveCase {
		result = re.ReplaceAllStringFunc(text, func(match string) string {
			return m.preserveCase(match, rep.ReplaceWith)
		})
	} else {
		result = re.ReplaceAllString(text, rep.ReplaceWith)
	}

	return result, matchCount
}

// applyReverseReplacement handles reverse replacements
func (m *Manager) applyReverseReplacement(text string, rep config.Replacement) (string, int) {
	// For reverse replacement, we'll work with individual words/tokens
	origWord := rep.ReplaceWith // What we're looking for (e.g., "GithubUser")

	// What we'll replace it with - check for override first
	var replaceWord string
	if rep.ReverseWith != "" {
		// Use the specified reverse replacement if provided
		replaceWord = rep.ReverseWith
	} else {
		// Fall back to extracting the first alternative from the regex
		replaceWord = m.extractFirstAlternative(rep.Regex)
	}

	// Split the text into tokens (words, whitespace, punctuation)
	re := regexp.MustCompile(`(\w+|[^\w\s]+|\s+)`)
	tokens := re.FindAllString(text, -1)

	// Track replacements
	replacementCount := 0

	// Go through each token and replace if it matches our target
	for i, token := range tokens {
		if !m.isWord(token) {
			// Skip non-word tokens
			continue
		}

		// Check if this token matches our replacement word
		if (rep.PreserveCase && strings.EqualFold(token, origWord)) ||
			(!rep.PreserveCase && token == origWord) {
			// It's a match - replace it
			if rep.PreserveCase {
				tokens[i] = m.preserveCase(token, replaceWord)
			} else {
				tokens[i] = replaceWord
			}
			replacementCount++
		}
	}

	// Only rebuild the text if we made replacements
	if replacementCount > 0 {
		return strings.Join(tokens, ""), replacementCount
	}

	return text, 0
}

// pasteClipboardContent simulates a paste action
func (m *Manager) pasteClipboardContent() {
	switch runtime.GOOS {
	case "windows":
		keyboard := syscall.NewLazyDLL("user32.dll")
		keybd_event := keyboard.NewProc("keybd_event")
		// VK_CONTROL = 0x11, VK_V = 0x56
		keybd_event.Call(0x11, 0, 0, 0) // Press Ctrl
		keybd_event.Call(0x56, 0, 0, 0) // Press V
		keybd_event.Call(0x56, 0, 2, 0) // Release V
		keybd_event.Call(0x11, 0, 2, 0) // Release Ctrl
	case "linux":
		if err := exec.Command("xdotool", "key", "ctrl+v").Run(); err != nil {
			log.Printf("Failed to simulate paste on Linux: %v", err)
		}
	default:
		log.Println("Automatic paste not supported on this platform.")
	}

	// Handle automatic reversion after paste if enabled
	if m.config.TemporaryClipboard && m.config.AutomaticReversion && m.previousClipboard != "" {
		// Give a small delay after paste to ensure the paste operation completes
		time.Sleep(50 * time.Millisecond)

		// Restore original clipboard
		if err := clipboard.WriteAll(m.previousClipboard); err != nil {
			log.Printf("Failed to automatically restore original clipboard: %v", err)
		} else {
			log.Println("Original clipboard content automatically restored after paste.")
			// Update revert status
			m.previousClipboard = ""
			if m.onRevertStatusChange != nil {
				m.onRevertStatusChange(false)
			}
		}
	}
}

// Helper methods below

// isWord checks if a token is a word (alphanumeric)
func (m *Manager) isWord(token string) bool {
	for _, r := range token {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' {
			return false
		}
	}
	return len(token) > 0
}

// extractFirstAlternative attempts to extract the first pattern from an alternation
func (m *Manager) extractFirstAlternative(regex string) string {
	// Remove case-insensitive flag
	regex = strings.TrimPrefix(regex, "(?i)")

	// Try to extract first alternative from pattern with alternation
	re := regexp.MustCompile(`\(([^|)]+)`)
	matches := re.FindStringSubmatch(regex)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// If no alternation found, try to extract the pattern inside parentheses
	re = regexp.MustCompile(`\(([^)]+)\)`)
	matches = re.FindStringSubmatch(regex)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// As a last resort, just clean up the regex
	regex = strings.TrimPrefix(regex, "(")
	regex = strings.TrimSuffix(regex, ")")
	return strings.TrimSpace(regex)
}

// preserveCase applies the case pattern from source to target
func (m *Manager) preserveCase(source, target string) string {
	// If source is empty or target is empty, return target as is
	if source == "" || target == "" {
		return target
	}

	// If source is all lowercase, return target as all lowercase
	if source == strings.ToLower(source) {
		return strings.ToLower(target)
	}

	// If source is all uppercase, return target as all uppercase
	if source == strings.ToUpper(source) {
		return strings.ToUpper(target)
	}

	// For PascalCase/camelCase and other mixed cases
	sourceRunes := []rune(source)
	targetRunes := []rune(target)

	// If target has internal capitalization (like "GithubUser"), preserve it
	// but adjust the first character to match source
	if m.hasInternalCapitalization(target) {
		if len(sourceRunes) > 0 && len(targetRunes) > 0 {
			if unicode.IsUpper(sourceRunes[0]) {
				targetRunes[0] = unicode.ToUpper(targetRunes[0])
			} else {
				targetRunes[0] = unicode.ToLower(targetRunes[0])
			}
		}
		return string(targetRunes)
	}

	// For Title Case (first letter uppercase, rest lowercase)
	if len(sourceRunes) > 1 &&
		unicode.IsUpper(sourceRunes[0]) &&
		strings.ToLower(string(sourceRunes[1:])) == string(sourceRunes[1:]) {
		if len(targetRunes) > 0 {
			if len(targetRunes) > 1 {
				return string(unicode.ToUpper(targetRunes[0])) + strings.ToLower(string(targetRunes[1:]))
			} else {
				return string(unicode.ToUpper(targetRunes[0]))
			}
		}
	}

	// Default: just make first letter match source
	if len(sourceRunes) > 0 && len(targetRunes) > 0 {
		if unicode.IsUpper(sourceRunes[0]) {
			targetRunes[0] = unicode.ToUpper(targetRunes[0])
		} else {
			targetRunes[0] = unicode.ToLower(targetRunes[0])
		}
	}

	return string(targetRunes)
}

// hasInternalCapitalization checks if a string has uppercase letters after the first position
func (m *Manager) hasInternalCapitalization(s string) bool {
	runes := []rune(s)
	for i := 1; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) {
			return true
		}
	}
	return false
}