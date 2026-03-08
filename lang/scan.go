package lang

import "unicode"

func scanIdentifiers(source string) []string {
	var ids []string
	i := 0

	for i < len(source) {
		ch := rune(source[i])

		if ch == '"' || ch == '\'' || ch == '`' {
			i++
			for i < len(source) && rune(source[i]) != ch {
				if source[i] == '\\' {
					i++
				}
				i++
			}
			if i < len(source) {
				i++
			}

			continue
		}

		if !isIdentStart(ch) {
			i++

			continue
		}

		if i > 0 && source[i-1] == '.' {
			for i < len(source) && isIdentContinue(rune(source[i])) {
				i++
			}

			continue
		}

		start := i
		for i < len(source) && isIdentContinue(rune(source[i])) {
			i++
		}

		token := source[start:i]

		switch token {
		case "true", "false", "nil", "in", "not", "and", "or",
			"matches", "contains", "startsWith", "endsWith",
			"let", "string", "int", "float", "bool",
			"len", "all", "any", "one", "none",
			"map", "filter", "count", "sum", "reduce",
			"first", "last", "take", "trim", "split",
			"upper", "lower", "repeat", "replace",
			"min", "max", "abs", "ceil", "floor",
			"toJSON", "fromJSON", "now", "date", "duration",
			"type", "sprintf", "toPairs", "fromPairs",
			"keys", "values", "sort", "sortBy", "reverse",
			"flatten", "unique", "groupBy", "join":
			continue
		}

		ids = append(ids, token)
	}

	return ids
}

func isIdentStart(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch)
}

func isIdentContinue(ch rune) bool {
	return ch == '_' || ch == '-' || unicode.IsLetter(ch) ||
		unicode.IsDigit(ch)
}
