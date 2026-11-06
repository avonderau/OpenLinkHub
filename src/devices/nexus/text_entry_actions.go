package nexus

import (
	"fmt"
	"strings"
	"time"

	"OpenLinkHub/src/inputmanager"
)

// Text-entry actions synthesize printable US-layout ASCII key presses.
// The limit bounds execution time and excludes control characters, shell
// syntax interpretation, multiline input and unsupported Unicode.
const maxNexusTextEntryBytes = 80

type typedKey struct {
	code  uint16
	shift bool
}

var nexusTextPunctuation = map[rune]typedKey{
	' ':  {inputmanager.KeySpace, false},
	'-':  {inputmanager.KeyMinus, false},
	'_':  {inputmanager.KeyMinus, true},
	'=':  {inputmanager.KeyEqual, false},
	'+':  {inputmanager.KeyEqual, true},
	'[':  {inputmanager.KeyLeftSquare, false},
	'{':  {inputmanager.KeyLeftSquare, true},
	']':  {inputmanager.KeyRightSquare, false},
	'}':  {inputmanager.KeyRightSquare, true},
	'\\': {inputmanager.KeyBackslash, false},
	'|':  {inputmanager.KeyBackslash, true},
	';':  {inputmanager.KeySemicolon, false},
	':':  {inputmanager.KeySemicolon, true},
	'\'': {inputmanager.KeySingleQuote, false},
	'"':  {inputmanager.KeySingleQuote, true},
	',':  {inputmanager.KeyComma, false},
	'<':  {inputmanager.KeyComma, true},
	'.':  {inputmanager.KeyDot, false},
	'>':  {inputmanager.KeyDot, true},
	'/':  {inputmanager.KeySlash, false},
	'?':  {inputmanager.KeySlash, true},
	'`':  {inputmanager.KeyTilde, false},
	'~':  {inputmanager.KeyTilde, true},
	'!':  {inputmanager.Number1, true},
	'@':  {inputmanager.Number2, true},
	'#':  {inputmanager.Number3, true},
	'$':  {inputmanager.Number4, true},
	'%':  {inputmanager.Number5, true},
	'^':  {inputmanager.Number6, true},
	'&':  {inputmanager.Number7, true},
	'*':  {inputmanager.Number8, true},
	'(':  {inputmanager.Number9, true},
	')':  {inputmanager.Number0, true},
}

// parseNexusTextEntry rejects characters that cannot be emitted reliably
// by the known US virtual keyboard layout. This makes behavior explicit,
// instead of silently dropping unsupported runes.
func parseNexusTextEntry(value string) ([]keyChord, error) {
	if len(value) == 0 || len(value) > maxNexusTextEntryBytes {
		return nil, fmt.Errorf("text entry must contain 1-80 printable ASCII characters")
	}
	keys := make([]keyChord, 0, len(value))
	for _, r := range value {
		if r < 32 || r > 126 {
			return nil, fmt.Errorf("text entry contains unsupported character")
		}
		item, ok := nexusTextPunctuation[r]
		if !ok {
			switch {
			case r >= 'a' && r <= 'z':
				item.code = shortcutKeyCodes[strings.ToUpper(string(r))]
			case r >= 'A' && r <= 'Z':
				item.code = shortcutKeyCodes[string(r)]
				item.shift = true
			case r >= '0' && r <= '9':
				item.code = shortcutKeyCodes[string(r)]
			default:
				return nil, fmt.Errorf("unsupported printable character")
			}
		}
		if item.code == 0 {
			return nil, fmt.Errorf("unsupported virtual keyboard key")
		}
		chord := keyChord{key: item.code}
		if item.shift {
			chord.modifiers = []uint16{inputmanager.KeyLeftShift}
		}
		keys = append(keys, chord)
	}
	return keys, nil
}

func executeNexusTextEntry(value string) {
	chords, err := parseNexusTextEntry(value)
	if err != nil {
		return
	}
	nexusShortcutMutex.Lock()
	defer nexusShortcutMutex.Unlock()
	for _, chord := range chords {
		sendKeyChord(chord, inputmanager.InputControlKeyboardHold)
		time.Sleep(8 * time.Millisecond)
	}
}
