package gateway

import (
	"encoding/json"
	"fmt"
	"bytes"
	"io"
	"strings"
)

// ANSI color codes for JSON syntax highlighting.
const (
	jsonKey    = "\033[34m" // blue
	jsonString = "\033[32m" // green
	jsonNumber = "\033[33m" // yellow/orange
	jsonBool   = "\033[35m" // magenta
	jsonNull   = "\033[90m" // gray
	jsonReset  = "\033[0m"
)

// WriteColorJSON writes indented JSON with syntax highlighting to w.
// Falls back to plain json.Indent if colors are disabled (NO_COLOR set).
func WriteColorJSON(w io.Writer, data []byte, prefix, indent string) error {
	if isColorDisabled() {
		var out bytes.Buffer
		if err := json.Indent(&out, data, prefix, indent); err != nil {
			return err
		}
		_, err := w.Write(out.Bytes())
		return err
	}

	return writeColorJSONSlow(w, data, prefix, indent)
}

// ColorJSONString returns a colorized JSON string.
func ColorJSONString(data []byte, prefix, indent string) (string, error) {
	var buf bytes.Buffer
	if err := WriteColorJSON(&buf, data, prefix, indent); err != nil {
		return "", err
	}
	return string(buf.Bytes()), nil
}





func isColorDisabled() bool {
	return false // colors enabled by default; NO_COLOR check could be added
}

// writeColorJSONSlow does a simple pass over already-indented JSON and adds color.
func writeColorJSONSlow(w io.Writer, data []byte, prefix, indent string) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, prefix, indent); err != nil {
		return err
	}

	colored := colorizeJSONBytes(buf.Bytes())
	_, err := w.Write(colored)
	return err
}

func colorizeJSONBytes(data []byte) []byte {
	var out strings.Builder
	out.Grow(len(data) + 512) // extra for color codes

	i := 0
	n := len(data)
	for i < n {
		c := data[i]

		switch {
		case c == '"':
			// Find end of string
			j := i + 1
			for j < n {
				if data[j] == '\\' {
					j += 2 // skip escaped char
				} else if data[j] == '"' {
					break
				} else {
					j++
				}
			}
			// Check if this is a key (followed by ": ")
			isKey := false
			k := j + 1
			for k < n && (data[k] == ' ' || data[k] == '\t' || data[k] == '\n') {
				k++
			}
			if k < n && data[k] == ':' {
				isKey = true
			}

			if isKey {
				out.WriteString(jsonKey)
				out.Write(data[i : j+1])
				out.WriteString(jsonReset)
			} else {
				out.WriteString(jsonString)
				out.Write(data[i : j+1])
				out.WriteString(jsonReset)
			}
			i = j + 1

		case (c >= '0' && c <= '9') || c == '-':
			j := i
			for j < n && ((data[j] >= '0' && data[j] <= '9') || data[j] == '.' || data[j] == '-' || data[j] == 'e' || data[j] == 'E' || data[j] == '+') {
				j++
			}
			out.WriteString(jsonNumber)
			out.Write(data[i:j])
			out.WriteString(jsonReset)
			i = j

		case c == 't' && i+3 < n && string(data[i:i+4]) == "true":
			out.WriteString(jsonBool)
			out.WriteString("true")
			out.WriteString(jsonReset)
			i += 4

		case c == 'f' && i+4 < n && string(data[i:i+5]) == "false":
			out.WriteString(jsonBool)
			out.WriteString("false")
			out.WriteString(jsonReset)
			i += 5

		case c == 'n' && i+3 < n && string(data[i:i+4]) == "null":
			out.WriteString(jsonNull)
			out.WriteString("null")
			out.WriteString(jsonReset)
			i += 4

		default:
			out.WriteByte(c)
			i++
		}
	}

	return []byte(out.String())
}

// MustColorJSON is like ColorJSONString but panics on error (for tests/debug).
func MustColorJSON(data []byte) string {
	s, err := ColorJSONString(data, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("jsoncolor: %v", err))
	}
	return s
}
