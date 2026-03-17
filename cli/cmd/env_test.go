package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/ardnew/aenv/lang"
)

// TestEnv_Run_SimpleScalar tests a non-parametric scalar namespace.
func TestEnv_Run_SimpleScalar(t *testing.T) {
	output := runEnvCommand(t, "port : 8080", "port", nil, quoteNone)

	want := "port=8080\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// TestEnv_Run_BlockDefinitionOrder tests that block members appear in
// definition order.
func TestEnv_Run_BlockDefinitionOrder(t *testing.T) {
	source := `db : { port : 5432; host : "localhost" }`
	output := runEnvCommand(t, source, "db", nil, quoteNone)

	want := "port=5432\nhost=localhost\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// TestEnv_Run_ParametricNamespace tests a namespace with parameters.
func TestEnv_Run_ParametricNamespace(t *testing.T) {
	source := `greet name : { msg : "Hello, " + name }`
	output := runEnvCommand(t, source, "greet", []string{"world"}, quoteNone)

	want := "msg=Hello, world\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// TestEnv_Run_NonScalarMembersOmitted tests that nested block members are
// omitted from output.
func TestEnv_Run_NonScalarMembersOmitted(t *testing.T) {
	source := `ns : { nested : { inner : 1 }; flat : 42 }`
	output := runEnvCommand(t, source, "ns", nil, quoteNone)

	want := "flat=42\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// TestEnv_Run_CallableMemberOmitted tests that parametric entries and FuncRef
// aliases are absent from output.
func TestEnv_Run_CallableMemberOmitted(t *testing.T) {
	source := `ns : { add a b : a + b; ref : add; val : add(1, 2) }`
	output := runEnvCommand(t, source, "ns", nil, quoteNone)

	want := "val=3\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// TestEnv_Run_CallableUsedByScalar tests the comprehensive example from the
// plan: callable members used by scalar members that invoke them.
func TestEnv_Run_CallableUsedByScalar(t *testing.T) {
	source := `foo a1 a2 : {
  bar : { baz : a1 * a2 }
  bah b1 b2 : b1 + b2;
  raa : bar;
  rbb : bah;
  vaa : raa.baz;
  vz1 : bah(1, 2);
  vz2 : rbb(6, 7) - a1;
}`
	output := runEnvCommand(t, source, "foo", []string{"10", "20"}, quoteNone)

	want := "vaa=200\nvz1=3\nvz2=3\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// TestEnv_Run_QuoteNone tests the default (no quoting) mode.
func TestEnv_Run_QuoteNone(t *testing.T) {
	source := `ns : { key : "value with spaces" }`
	output := runEnvCommand(t, source, "ns", nil, quoteNone)

	want := "key=value with spaces\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// TestEnv_Run_QuoteSingle tests single quoting.
func TestEnv_Run_QuoteSingle(t *testing.T) {
	source := `ns : { key : "value with spaces" }`
	output := runEnvCommand(t, source, "ns", nil, quoteSingle)

	want := "key='value with spaces'\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// TestEnv_Run_QuoteDouble tests double quoting.
func TestEnv_Run_QuoteDouble(t *testing.T) {
	source := `ns : { key : "value with spaces" }`
	output := runEnvCommand(t, source, "ns", nil, quoteDouble)

	want := `key="value with spaces"` + "\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// TestEnv_Run_QuotePosix tests POSIX $'...' quoting.
func TestEnv_Run_QuotePosix(t *testing.T) {
	source := `ns : { key : "value with spaces" }`
	output := runEnvCommand(t, source, "ns", nil, quotePosix)

	want := "key=$'value with spaces'\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// TestEnv_Run_QuoteSingleEmbeddedQuote tests single quoting with an embedded
// single quote in the value.
func TestEnv_Run_QuoteSingleEmbeddedQuote(t *testing.T) {
	source := `ns : { key : "it's here" }`
	output := runEnvCommand(t, source, "ns", nil, quoteSingle)

	want := "key='it'\"'\"'s here'\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// TestEnv_Run_VariadicParams tests a variadic namespace with multiple args.
func TestEnv_Run_VariadicParams(t *testing.T) {
	source := `sum ...nums : { result : nums[0] + nums[1] + nums[2] }`
	output := runEnvCommand(t, source, "sum", []string{"10", "20", "30"}, quoteNone)

	want := "result=60\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// TestEnv_Run_MissingNamespace tests that an unknown namespace returns an
// error wrapping [lang.ErrNotDefined].
func TestEnv_Run_MissingNamespace(t *testing.T) {
	source := `existing : 1`

	err := runEnvCommandErr(t, source, "nonexistent", nil, quoteNone)

	if err == nil {
		t.Fatal("expected error for missing namespace, got nil")
	}

	if !errors.Is(err, lang.ErrNotDefined) {
		t.Errorf("expected error wrapping ErrNotDefined, got: %v", err)
	}
}

// TestEnv_Run_WrongParamCount tests that too few arguments returns an error
// wrapping [lang.ErrParameterCount].
func TestEnv_Run_WrongParamCount(t *testing.T) {
	source := `greet a b : { msg : a + b }`

	err := runEnvCommandErr(t, source, "greet", []string{"one"}, quoteNone)

	if err == nil {
		t.Fatal("expected error for wrong param count, got nil")
	}

	if !errors.Is(err, lang.ErrParameterCount) {
		t.Errorf("expected error wrapping ErrParameterCount, got: %v", err)
	}
}

// TestEnv_Run_EmptyBlock tests that a namespace with an empty block produces
// no output.
func TestEnv_Run_EmptyBlock(t *testing.T) {
	source := `x : {}`
	output := runEnvCommand(t, source, "x", nil, quoteNone)

	if output != "" {
		t.Errorf("expected no output for empty block, got %q", output)
	}
}

// TestEnv_Run_BooleanValue tests that a boolean value is formatted correctly.
func TestEnv_Run_BooleanValue(t *testing.T) {
	source := `ns : { flag : true }`
	output := runEnvCommand(t, source, "ns", nil, quoteNone)

	want := "flag=true\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// TestEnv_Run_ArrayValue tests that an array value is formatted as
// comma-separated elements.
func TestEnv_Run_ArrayValue(t *testing.T) {
	source := `ns : { items : ["a", "b", "c"] }`
	output := runEnvCommand(t, source, "ns", nil, quoteNone)

	want := "items=a,b,c\n"
	if output != want {
		t.Errorf("got %q, want %q", output, want)
	}
}

// --- helpers ---------------------------------------------------------------

// runEnvCommand writes source to a temp file, runs Env.Run, captures stdout,
// and returns the output. It fails the test on any error.
func runEnvCommand(
	t *testing.T,
	source, name string,
	args []string,
	q quote,
) string {
	t.Helper()

	output, err := execEnv(t, source, name, args, q)
	if err != nil {
		t.Fatalf("Env.Run() unexpected error: %v", err)
	}

	return output
}

// runEnvCommandErr writes source to a temp file, runs Env.Run, and returns
// only the error (stdout is discarded).
func runEnvCommandErr(
	t *testing.T,
	source, name string,
	args []string,
	q quote,
) error {
	t.Helper()

	_, err := execEnv(t, source, name, args, q)

	return err
}

// execEnv is the shared implementation for running the env command in tests.
// It creates a temp file with source, sets up context, captures stdout, and
// returns both the captured output and any error from Run.
func execEnv(
	t *testing.T,
	source, name string,
	args []string,
	q quote,
) (string, error) {
	t.Helper()

	tmpfile, err := os.CreateTemp("", "aenv-env-test-*.aenv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(source); err != nil {
		t.Fatal(err)
	}

	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	ctx := withSourceFile(t, tmpfile.Name())

	envCmd := &Env{
		Name:  name,
		Args:  args,
		Quote: q,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	runErr := envCmd.Run(ctx)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)

	return buf.String(), runErr
}

// --- unit tests for helpers ------------------------------------------------

// TestPosixEscape tests the posixEscape function directly.
func TestPosixEscape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain", input: "hello", want: "hello"},
		{name: "backslash", input: `a\b`, want: `a\\b`},
		{name: "single_quote", input: "it's", want: `it\'s`},
		{name: "newline", input: "a\nb", want: `a\nb`},
		{name: "carriage_return", input: "a\rb", want: `a\rb`},
		{name: "tab", input: "a\tb", want: `a\tb`},
		{name: "mixed", input: "it's\na\\b", want: `it\'s\na\\b`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := posixEscape(tt.input)
			if got != tt.want {
				t.Errorf("posixEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestQuote_wrap tests the quote.wrap method for each quoting style.
func TestQuote_wrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		q     quote
		input string
		want  string
	}{
		{name: "none_plain", q: quoteNone, input: "hello", want: "hello"},
		{name: "single_plain", q: quoteSingle, input: "hello", want: "'hello'"},
		{name: "single_embedded", q: quoteSingle, input: "it's", want: "'it'\"'\"'s'"},
		{name: "double_plain", q: quoteDouble, input: "hello", want: `"hello"`},
		{name: "double_special", q: quoteDouble, input: "a\nb", want: `"a\nb"`},
		{name: "posix_plain", q: quotePosix, input: "hello", want: "$'hello'"},
		{name: "posix_special", q: quotePosix, input: "a\nb", want: `$'a\nb'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.q.wrap(tt.input)
			if got != tt.want {
				t.Errorf("quote(%q).wrap(%q) = %q, want %q", tt.q, tt.input, got, tt.want)
			}
		})
	}
}

// TestIsEnvScalar tests the isEnvScalar function.
func TestIsEnvScalar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		val  any
		want bool
	}{
		{name: "nil", val: nil, want: true},
		{name: "string", val: "hello", want: true},
		{name: "int", val: 42, want: true},
		{name: "float", val: 3.14, want: true},
		{name: "bool", val: true, want: true},
		{name: "slice", val: []any{"a"}, want: true},
		{name: "map", val: map[string]any{"k": "v"}, want: false},
		{name: "funcref", val: lang.NewFuncRef("f", "f()"), want: false},
		{name: "func", val: func() {}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isEnvScalar(tt.val)
			if got != tt.want {
				t.Errorf("isEnvScalar(%v) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

// TestFormatEnvValue tests the formatEnvValue function.
func TestFormatEnvValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		val  any
		want string
	}{
		{name: "nil", val: nil, want: ""},
		{name: "bool_true", val: true, want: "true"},
		{name: "bool_false", val: false, want: "false"},
		{name: "int", val: 42, want: "42"},
		{name: "int64", val: int64(100), want: "100"},
		{name: "float", val: 3.14, want: "3.14"},
		{name: "string", val: "hello", want: "hello"},
		{name: "slice", val: []any{"a", "b"}, want: "a,b"},
		{name: "empty_slice", val: []any{}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatEnvValue(tt.val)
			if got != tt.want {
				t.Errorf("formatEnvValue(%v) = %q, want %q", tt.val, got, tt.want)
			}
		})
	}
}

// TestCollectEnv tests the collectEnv function directly.
func TestCollectEnv(t *testing.T) {
	t.Parallel()

	t.Run("scalar_result", func(t *testing.T) {
		t.Parallel()

		lines := collectEnv("port", 8080, nil, quoteNone)
		if len(lines) != 1 || lines[0] != "port=8080" {
			t.Errorf("got %v, want [port=8080]", lines)
		}
	})

	t.Run("non_scalar_result", func(t *testing.T) {
		t.Parallel()

		lines := collectEnv("f", func() {}, nil, quoteNone)
		if lines != nil {
			t.Errorf("got %v, want nil", lines)
		}
	})

	t.Run("map_definition_order", func(t *testing.T) {
		t.Parallel()

		entries := []*lang.Namespace{
			{Name: "b"},
			{Name: "a"},
		}
		m := map[string]any{"a": "1", "b": "2"}
		lines := collectEnv("ns", m, entries, quoteNone)

		want := []string{"b=2", "a=1"}
		if len(lines) != len(want) {
			t.Fatalf("got %d lines, want %d", len(lines), len(want))
		}

		for i, line := range lines {
			if line != want[i] {
				t.Errorf("line %d: got %q, want %q", i, line, want[i])
			}
		}
	})

	t.Run("deduplicates_entries", func(t *testing.T) {
		t.Parallel()

		entries := []*lang.Namespace{
			{Name: "a"},
			{Name: "a"},
		}
		m := map[string]any{"a": "1"}
		lines := collectEnv("ns", m, entries, quoteNone)

		if len(lines) != 1 {
			t.Errorf("got %d lines, want 1 (deduplication)", len(lines))
		}
	})

	t.Run("skips_non_scalar_map_values", func(t *testing.T) {
		t.Parallel()

		entries := []*lang.Namespace{
			{Name: "scalar"},
			{Name: "block"},
		}
		m := map[string]any{
			"scalar": "yes",
			"block":  map[string]any{"inner": 1},
		}
		lines := collectEnv("ns", m, entries, quoteNone)

		if len(lines) != 1 || lines[0] != "scalar=yes" {
			t.Errorf("got %v, want [scalar=yes]", lines)
		}
	})

	t.Run("with_quoting", func(t *testing.T) {
		t.Parallel()

		lines := collectEnv("k", "hello world", nil, quoteSingle)
		if len(lines) != 1 || lines[0] != "k='hello world'" {
			t.Errorf("got %v, want [k='hello world']", lines)
		}
	})
}
