package linearizabilitychecker

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// ParseEDNHistory parses Jepsen EDN history format into []Op.
func ParseEDNHistory(r io.Reader, maxBytes int64) ([]Op, error) {
	if maxBytes <= 0 {
		maxBytes = MaxEDNBytes
	}
	lr := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, &CheckerError{Code: "input_too_large", Message: err.Error()}
	}
	if int64(len(data)) > maxBytes {
		return nil, &CheckerError{
			Code:    "input_too_large",
			Message: fmt.Sprintf("input size %d exceeds limit %d", len(data), maxBytes),
		}
	}

	p := &ednParser{text: string(data), pos: 0}
	v, err := p.parse()
	if err != nil {
		return nil, &CheckerError{
			Code:    "edn_parse_error",
			Message: truncateString(err.Error(), 200),
		}
	}

	var rawItems []any
	switch val := v.(type) {
	case []any:
		rawItems = val
	case map[string]any:
		rawItems = []any{val}
	default:
		return nil, &CheckerError{
			Code:    "edn_parse_error",
			Message: "top-level EDN must be a vector or map",
		}
	}

	type pendingCall struct {
		op      Op
		callIdx int
	}
	pending := make(map[int]pendingCall)
	var ops []Op

	for idx, raw := range rawItems {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		typ, _ := m[":type"].(string)
		if typ == "" {
			typ, _ = m["type"].(string)
		}
		typ = strings.TrimPrefix(typ, ":")

		proc := 0
		if pVal, ok := m[":process"]; ok {
			if n, ok := toInt(pVal); ok {
				proc = n
			}
		} else if pVal, ok := m["process"]; ok {
			if n, ok := toInt(pVal); ok {
				proc = n
			}
		}

		var callTime int64
		if tVal, ok := m[":time"]; ok {
			if n, ok := toInt64(tVal); ok {
				callTime = n
			}
		} else if tVal, ok := m["time"]; ok {
			if n, ok := toInt64(tVal); ok {
				callTime = n
			}
		} else {
			callTime = int64(idx * 2)
		}

		val := m[":value"]
		if val == nil {
			val = m["value"]
		}
		f := m[":f"]
		if f == nil {
			f = m["f"]
		}

		var keyStr string
		if kVal, ok := m[":key"].(string); ok {
			keyStr = kVal
		} else if kVal, ok := m["key"].(string); ok {
			keyStr = kVal
		}

		if typ == "invoke" {
			op := Op{
				ID:       len(ops),
				Process:  proc,
				Kind:     KindInvoke,
				Key:      keyStr,
				CallTime: callTime,
				Input:    buildInvokeInput(f, val),
			}
			pending[proc] = pendingCall{op: op, callIdx: len(ops)}
			ops = append(ops, op)
		} else if typ == "ok" || typ == "fail" || typ == "info" {
			if call, exists := pending[proc]; exists {
				ops[call.callIdx].ReturnTime = callTime
				ops[call.callIdx].Kind = Kind(typ)
				ops[call.callIdx].Output = val
				if keyStr != "" {
					ops[call.callIdx].Key = keyStr
				}
				delete(pending, proc)
			} else {
				op := Op{
					ID:         len(ops),
					Process:    proc,
					Kind:       Kind(typ),
					Key:        keyStr,
					CallTime:   callTime,
					ReturnTime: callTime,
					Output:     val,
				}
				ops = append(ops, op)
			}
		}
	}

	for _, pendingCall := range pending {
		ops[pendingCall.callIdx].Kind = KindInfo
		ops[pendingCall.callIdx].ReturnTime = 0
	}

	return ops, nil
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

func buildInvokeInput(f any, val any) any {
	if f == nil {
		return val
	}
	fStr := fmt.Sprintf("%v", f)
	fStr = strings.TrimPrefix(fStr, ":")
	return map[string]any{
		"f":     fStr,
		"value": val,
	}
}

type ednParser struct {
	text string
	pos  int
}

func (p *ednParser) parse() (any, error) {
	p.skipWhitespaceAndComments()
	if p.pos >= len(p.text) {
		return nil, io.EOF
	}
	c := p.text[p.pos]
	switch c {
	case '[':
		return p.parseVector()
	case '{':
		return p.parseMap()
	case '(':
		return p.parseList()
	case '"':
		return p.parseString()
	case ':':
		return p.parseKeyword()
	default:
		if c == '-' || c == '+' || unicode.IsDigit(rune(c)) {
			return p.parseNumber()
		}
		return p.parseSymbolOrLiteral()
	}
}

func (p *ednParser) skipWhitespaceAndComments() {
	for p.pos < len(p.text) {
		c := p.text[p.pos]
		if unicode.IsSpace(rune(c)) || c == ',' {
			p.pos++
			continue
		}
		if c == ';' {
			for p.pos < len(p.text) && p.text[p.pos] != '\n' {
				p.pos++
			}
			continue
		}
		break
	}
}

func (p *ednParser) parseVector() ([]any, error) {
	p.pos++ // skip '['
	var res []any
	for {
		p.skipWhitespaceAndComments()
		if p.pos >= len(p.text) {
			return nil, fmt.Errorf("unclosed vector")
		}
		if p.text[p.pos] == ']' {
			p.pos++
			return res, nil
		}
		elem, err := p.parse()
		if err != nil {
			return nil, err
		}
		res = append(res, elem)
	}
}

func (p *ednParser) parseList() ([]any, error) {
	p.pos++ // skip '('
	var res []any
	for {
		p.skipWhitespaceAndComments()
		if p.pos >= len(p.text) {
			return nil, fmt.Errorf("unclosed list")
		}
		if p.text[p.pos] == ')' {
			p.pos++
			return res, nil
		}
		elem, err := p.parse()
		if err != nil {
			return nil, err
		}
		res = append(res, elem)
	}
}

func (p *ednParser) parseMap() (map[string]any, error) {
	p.pos++ // skip '{'
	res := make(map[string]any)
	for {
		p.skipWhitespaceAndComments()
		if p.pos >= len(p.text) {
			return nil, fmt.Errorf("unclosed map")
		}
		if p.text[p.pos] == '}' {
			p.pos++
			return res, nil
		}
		key, err := p.parse()
		if err != nil {
			return nil, err
		}
		keyStr := fmt.Sprintf("%v", key)
		val, err := p.parse()
		if err != nil {
			return nil, err
		}
		res[keyStr] = val
	}
}

func (p *ednParser) parseString() (string, error) {
	p.pos++ // skip leading '"'
	var sb strings.Builder
	for p.pos < len(p.text) {
		c := p.text[p.pos]
		if c == '\\' {
			p.pos++
			if p.pos >= len(p.text) {
				return "", fmt.Errorf("unexpected end of string escape")
			}
			esc := p.text[p.pos]
			switch esc {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			default:
				sb.WriteByte(esc)
			}
			p.pos++
			continue
		}
		if c == '"' {
			p.pos++
			return sb.String(), nil
		}
		sb.WriteByte(c)
		p.pos++
	}
	return "", fmt.Errorf("unclosed string")
}

func (p *ednParser) parseKeyword() (string, error) {
	start := p.pos
	p.pos++ // skip ':'
	for p.pos < len(p.text) {
		c := p.text[p.pos]
		if unicode.IsSpace(rune(c)) || c == ',' || c == ']' || c == '}' || c == ')' || c == '(' || c == ';' {
			break
		}
		p.pos++
	}
	return p.text[start:p.pos], nil
}

func (p *ednParser) parseNumber() (any, error) {
	start := p.pos
	hasDot := false
	if p.text[p.pos] == '-' || p.text[p.pos] == '+' {
		p.pos++
	}
	for p.pos < len(p.text) {
		c := p.text[p.pos]
		if c == '.' {
			hasDot = true
			p.pos++
		} else if unicode.IsDigit(rune(c)) {
			p.pos++
		} else {
			break
		}
	}
	raw := p.text[start:p.pos]
	if hasDot {
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (p *ednParser) parseSymbolOrLiteral() (any, error) {
	start := p.pos
	for p.pos < len(p.text) {
		c := p.text[p.pos]
		if unicode.IsSpace(rune(c)) || c == ',' || c == ']' || c == '}' || c == ')' || c == '(' || c == ';' || c == '{' || c == '[' {
			break
		}
		p.pos++
	}
	val := p.text[start:p.pos]
	switch val {
	case "nil":
		return nil, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return val, nil
	}
}
