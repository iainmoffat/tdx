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

// ResolveFormat picks the output format based on explicit flags or env.
// Precedence: --json > --human > TDX_FORMAT > default (human).
//
// Unlike some CLIs, tdx does not auto-select JSON when stdout is a pipe:
// agents that want JSON can pass --json or set TDX_FORMAT=json, and scripts
// piping human output through tools like less keep getting human output.
func ResolveFormat(f Flags) Format {
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
	return FormatHuman
}

// JSON writes v as pretty JSON followed by a newline.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Humanf writes a formatted line to w with a trailing newline.
// The format string should not include a trailing newline.
func Humanf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
	_, _ = fmt.Fprintln(w)
}
