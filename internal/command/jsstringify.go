package command

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// The writing half: JSON.stringify with JavaScript's own escaping and its own way of
// spelling a number, which is not Go's — `1e+21` and `0.0000001` are what a request body is
// rendered back as.

// jsJSONStringify renders a value the way JSON.stringify does. Go's encoder is not a substitute: it
// escapes <, > and & as < and friends, which would change an exported command.
func jsJSONStringify(v jsValue) string {
	var b strings.Builder
	writeJSJSON(&b, v)
	return b.String()
}

func writeJSJSON(b *strings.Builder, v jsValue) {
	switch v.kind {
	case jsNull:
		b.WriteString("null")
	case jsBool:
		if v.bl {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case jsNum:
		b.WriteString(jsNumberString(v.num))
	case jsStr:
		writeJSJSONString(b, v.str)
	case jsArr:
		b.WriteByte('[')
		for i, e := range v.arr {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSJSON(b, e)
		}
		b.WriteByte(']')
	case jsObj:
		b.WriteByte('{')
		for i, m := range v.members() {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSJSONString(b, m.key)
			b.WriteByte(':')
			writeJSJSON(b, m.val)
		}
		b.WriteByte('}')
	}
}

// writeJSJSONString escapes what JSON.stringify escapes and nothing else: the quote, the backslash,
// the short forms of the usual controls, and \u00xx for the rest.
func writeJSJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
}

// jsToString is String(value) for a header value that is not a string. A null never reaches here —
// the TS writes the empty string for those first — but an object or an array does, and String
// renders those as `[object Object]` and a comma-joined list, not as JSON.
func jsToString(v jsValue) string {
	switch v.kind {
	case jsStr:
		return v.str
	case jsNum:
		return jsNumberString(v.num)
	case jsBool:
		if v.bl {
			return "true"
		}
		return "false"
	case jsArr:
		parts := make([]string, 0, len(v.arr))
		for _, e := range v.arr {
			// Array.prototype.join writes null and undefined as nothing at all.
			if e.kind == jsNull {
				parts = append(parts, "")
				continue
			}
			parts = append(parts, jsToString(e))
		}
		return strings.Join(parts, ",")
	default:
		return "[object Object]"
	}
}

// jsNumberString is ECMAScript's Number::toString: the shortest digits that round-trip, written
// positionally while the exponent of the first digit is in (-7, 21) and exponentially outside it.
// Go's %g has its own thresholds, so JSON.stringify output would drift on a body like 1e21.
func jsNumberString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0" // covers -0 as well, which String(-0) also writes as "0"
	case f < 0:
		return "-" + jsNumberString(-f)
	}

	e := strconv.FormatFloat(f, 'e', -1, 64)
	mantissa, exponent := e, ""
	if idx := strings.IndexByte(e, 'e'); idx >= 0 {
		mantissa, exponent = e[:idx], e[idx+1:]
	}
	digits := strings.Replace(mantissa, ".", "", 1)
	exp, _ := strconv.Atoi(exponent)
	k := len(digits)
	n := exp + 1

	switch {
	case k <= n && n <= 21:
		return digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return "0." + strings.Repeat("0", -n) + digits
	case k == 1:
		return digits + "e" + jsExponentSign(n-1)
	default:
		return digits[:1] + "." + digits[1:] + "e" + jsExponentSign(n-1)
	}
}

// jsExponentSign writes the exponent the way JS does, always with a sign.
func jsExponentSign(e int) string {
	if e < 0 {
		return "-" + strconv.Itoa(-e)
	}
	return "+" + strconv.Itoa(e)
}
