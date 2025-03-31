package hotkey

import "golang.design/x/hotkey"

// KeyMap provides mapping between string representations and hotkey.Key values
var KeyMap = map[string]hotkey.Key{
	// Letters
	"a": hotkey.KeyA,
	"b": hotkey.KeyB,
	"c": hotkey.KeyC,
	"d": hotkey.KeyD,
	"e": hotkey.KeyE,
	"f": hotkey.KeyF,
	"g": hotkey.KeyG,
	"h": hotkey.KeyH,
	"i": hotkey.KeyI,
	"j": hotkey.KeyJ,
	"k": hotkey.KeyK,
	"l": hotkey.KeyL,
	"m": hotkey.KeyM,
	"n": hotkey.KeyN,
	"o": hotkey.KeyO,
	"p": hotkey.KeyP,
	"q": hotkey.KeyQ,
	"r": hotkey.KeyR,
	"s": hotkey.KeyS,
	"t": hotkey.KeyT,
	"u": hotkey.KeyU,
	"v": hotkey.KeyV,
	"w": hotkey.KeyW,
	"x": hotkey.KeyX,
	"y": hotkey.KeyY,
	"z": hotkey.KeyZ,

	// Numbers
	"0": hotkey.Key0,
	"1": hotkey.Key1,
	"2": hotkey.Key2,
	"3": hotkey.Key3,
	"4": hotkey.Key4,
	"5": hotkey.Key5,
	"6": hotkey.Key6,
	"7": hotkey.Key7,
	"8": hotkey.Key8,
	"9": hotkey.Key9,

	// Function keys
	"f1":  hotkey.KeyF1,
	"f2":  hotkey.KeyF2,
	"f3":  hotkey.KeyF3,
	"f4":  hotkey.KeyF4,
	"f5":  hotkey.KeyF5,
	"f6":  hotkey.KeyF6,
	"f7":  hotkey.KeyF7,
	"f8":  hotkey.KeyF8,
	"f9":  hotkey.KeyF9,
	"f10": hotkey.KeyF10,
	"f11": hotkey.KeyF11,
	"f12": hotkey.KeyF12,

	// Special keys
	"space":  hotkey.KeySpace,
	"tab":    hotkey.KeyTab,
	"enter":  hotkey.KeyReturn,
	"escape": hotkey.KeyEscape,
}
