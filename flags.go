package iscript

const (
	Install int = iota
	Remove
	Update
)

var flags = map[int]string{Install: "install", Remove: "remove", Update: "update"}

// GetFlag takes parse modifier and returns flag
func GetFlag(mode int) (string, bool) {
	result, ok := flags[mode]
	return result, ok
}

// IsFlag checks is string flag or not
func IsFlag(text string) bool {
	for _, flag := range flags {
		if text == flag {
			return true
		}
	}
	return false
}
