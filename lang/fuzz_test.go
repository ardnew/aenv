package lang

import (
	"testing"
	"unicode/utf8"

	"github.com/ardnew/aenv/lang/lexer"
	"github.com/ardnew/aenv/lang/parser"
)

// FuzzLexer tests the lexer with random inputs to find edge cases.
func FuzzLexer(f *testing.F) {
	// Seed corpus with known valid inputs
	f.Add("foo")
	f.Add("123")
	f.Add(`"string"`)
	f.Add("<{code}>")
	f.Add("// comment\n")
	f.Add("/* block */")
	f.Add("foo-bar_baz.qux")
	f.Add("0x1234")
	f.Add("-123.456e-10")
	f.Add(`"escaped\"quote"`)
	f.Add("name:{key=val}")

	f.Fuzz(func(t *testing.T, input string) {
		// Skip invalid UTF-8
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		// Lexer should not panic on any input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("lexer panicked on input %q: %v", input, r)
			}
		}()

		l := lexer.New([]rune(input))

		// Basic sanity checks
		if l == nil {
			return
		}

		// Verify all tokens have valid positions
		for i, tok := range l.Tokens {
			if tok == nil {
				t.Errorf("token %d is nil", i)
				continue
			}

			// Token type should be valid
			if tok.Type() < 0 {
				t.Errorf("token %d has invalid type: %d", i, tok.Type())
			}
		}
	})
}

// FuzzParser tests the parser with random inputs to find edge cases.
func FuzzParser(f *testing.F) {
	// Seed corpus with known valid syntax
	f.Add("ns:{}")
	f.Add("ns:{key=val}")
	f.Add("ns:{a=1,b=2}")
	f.Add("ns:{key=[1,2,3]}")
	f.Add("ns:{key={nested=val}}")
	f.Add(`ns:{str="hello"}`)
	f.Add("ns:{code=<{foo}>}")
	f.Add("ns:{num=123}")
	f.Add("ns:{neg=-456}")
	f.Add("ns:{float=12.34}")
	f.Add("ns:{bool=true}")
	f.Add(`ns:{tmpl="fmt %s"%["val"]}`)
	f.Add(`ns:{tmpl="%s %d"%["str",42]}`)
	f.Add("a:{} b:{}")

	f.Fuzz(func(t *testing.T, input string) {
		// Skip invalid UTF-8
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		// Parser should not panic on any input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parser panicked on input %q: %v", input, r)
			}
		}()

		l := lexer.New([]rune(input))
		if l == nil {
			return
		}

		bsr, err := parser.Parse(l)

		// It's OK for parsing to fail, but it shouldn't panic
		// and errors should be well-formed
		if err != nil {
			// Verify errors are not nil
			for i, e := range err {
				if e == nil {
					t.Errorf("error %d is nil", i)
				}
			}
			return
		}

		// If parsing succeeded, verify the BSR is valid
		if bsr != nil {
			roots := bsr.GetRoots()
			// Valid parse should have at least one root (even if empty input)
			if roots == nil {
				t.Error("GetRoots() returned nil for successful parse")
			}
		}
	})
}

// FuzzIdentifier tests identifier lexing specifically.
func FuzzIdentifier(f *testing.F) {
	f.Add("foo")
	f.Add("_foo")
	f.Add("foo.bar")
	f.Add("foo/bar")
	f.Add("foo-bar")
	f.Add(`foo\ bar`)
	f.Add("/path/to/file")

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("identifier lexing panicked on %q: %v", input, r)
			}
		}()

		l := lexer.New([]rune(input))
		if l == nil {
			return
		}

		// Should not crash
		_ = l.Tokens
	})
}

// FuzzNumber tests number literal lexing specifically.
func FuzzNumber(f *testing.F) {
	f.Add("0")
	f.Add("123")
	f.Add("-456")
	f.Add("0755")
	f.Add("0xff")
	f.Add("0b1010")
	f.Add("12.34")
	f.Add("-12.34")
	f.Add("1.23e10")
	f.Add("1.23e-10")

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("number lexing panicked on %q: %v", input, r)
			}
		}()

		l := lexer.New([]rune(input))
		if l == nil {
			return
		}

		// Should not crash
		_ = l.Tokens
	})
}

// FuzzString tests string literal lexing specifically.
func FuzzString(f *testing.F) {
	f.Add(`""`)
	f.Add(`"hello"`)
	f.Add(`"hello world"`)
	f.Add(`"hello\nworld"`)
	f.Add(`"say \"hello\""`)
	f.Add(`"\u0048"`)
	f.Add(`"\x48"`)
	f.Add(`"\101"`)

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("string lexing panicked on %q: %v", input, r)
			}
		}()

		l := lexer.New([]rune(input))
		if l == nil {
			return
		}

		// Should not crash
		_ = l.Tokens
	})
}

// FuzzCodeLiteral tests code literal lexing specifically.
func FuzzCodeLiteral(f *testing.F) {
	f.Add("<{}>")
	f.Add("<{foo}>")
	f.Add("<{foo; bar}>")
	f.Add("<{foo, bar}>")
	f.Add("<{foo} bar}>")

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("code literal lexing panicked on %q: %v", input, r)
			}
		}()

		l := lexer.New([]rune(input))
		if l == nil {
			return
		}

		// Should not crash
		_ = l.Tokens
	})
}

// FuzzNestedStructures tests deeply nested structures.
func FuzzNestedStructures(f *testing.F) {
	f.Add("ns:{a={b={c=val}}}")
	f.Add("ns:{a=[[[]]]}")
	f.Add("ns:{a={b=[{c=val}]}}")

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip("invalid UTF-8")
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("nested structure parsing panicked on %q: %v", input, r)
			}
		}()

		l := lexer.New([]rune(input))
		if l == nil {
			return
		}

		// Should not panic even with deeply nested structures
		_, _ = parser.Parse(l)
	})
}
