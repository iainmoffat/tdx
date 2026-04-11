package render

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Format selects human vs machine output.
type Format int

const (
	FormatHuman Format = iota
	FormatJSON
)

// Flags carries the global format flags parsed from CLI.
type Flags struct {
	JSON  bool
	Human bool
}

// ResolveFormat picks the output format based on explicit flags, env, and TTY.
// Precedence: --json > --human > TDX_FORMAT > TTY detection (TTY->Human, pipe->JSON).
func ResolveFormat(f Flags, out *os.File) Format {
	if f.JSON {
		return FormatJSON
	}
	if f.Human {
		return FormatHuman
	}
	if env := os.Getenv("TDX_FORMAT"); env != "" {
		if env == "json" {
			return FormatJSON
		}
		return FormatHuman
	}
	if out == nil || !isTerminal(out) {
		return FormatJSON
	}
	return FormatHuman
}

// JSON writes v as pretty JSON followed by a newline.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Humanf writes a formatted line to w with a trailing newline.
func Humanf(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, format, args...)
	fmt.Fprintln(w)
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
