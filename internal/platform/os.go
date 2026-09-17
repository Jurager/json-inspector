package platform

import "runtime"

// OSName is the platform as a person reads it in a list of their devices: "macOS", "Windows",
// "Linux". It is the app's own word rather than the kernel's — "darwin" is nobody's laptop — and it
// is the only thing about the machine that leaves it: the server cannot tell a laptop from a phone,
// and the app does not pretend to know more than that.
func OSName() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return runtime.GOOS
	}
}
