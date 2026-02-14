package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

// withSourceFile creates a context with a single source file for testing.
func withSourceFile(t *testing.T, path string) context.Context {
	t.Helper()

	return WithSourceFiles(context.Background(), []string{path})
}

// withStdin creates a context configured to read from stdin for testing.
func withStdin(t *testing.T) context.Context {
	t.Helper()

	return WithSourceFiles(context.Background(), []string{"-"})
}

// TestNativeFmtValidSyntax tests that valid syntax is formatted correctly.
func TestNativeFmtValidSyntax(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		contains string
	}{
		{
			name:     "simple definition",
			input:    "test : 123",
			wantErr:  false,
			contains: "test : 123",
		},
		{
			name:     "definition with tuple",
			input:    "test : {a : 1; b : 2}",
			wantErr:  false,
			contains: "test :",
		},
		{
			name:     "multiple definitions",
			input:    "a : 1; b : 2",
			wantErr:  false,
			contains: "a : 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with input
			tmpfile, err := os.CreateTemp("", "aenv-test-*.aenv")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.WriteString(tt.input); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			// Run the command with context-based source
			native := &Native{Indent: 2}
			ctx := withSourceFile(t, tmpfile.Name())

			err = native.Run(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Native.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestNativeFmtInvalidSyntax tests that invalid syntax produces parse errors.
func TestNativeFmtInvalidSyntax(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "comma in expression",
			input:   "ewjw :123   , 12 {};",
			wantErr: false, // commas are now part of expressions, not separators
		},
		{
			name:    "missing colon",
			input:   "test 123",
			wantErr: true,
		},
		{
			name:    "unclosed tuple",
			input:   "test : {a : 1",
			wantErr: true,
		},
		{
			name:    "invalid token",
			input:   "test : @invalid",
			wantErr: false, // lang2 accepts this as an expression (error at eval time)
		},
		{
			name:    "trailing comma",
			input:   "test : 123,",
			wantErr: false, // lang2 allows trailing separators
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with input
			tmpfile, err := os.CreateTemp("", "aenv-test-*.aenv")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.WriteString(tt.input); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			// Run the command with context-based source
			native := &Native{Indent: 2}
			ctx := withSourceFile(t, tmpfile.Name())

			err = native.Run(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Native.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err == nil {
				t.Error("Native.Run() expected error but got nil")
			}
		})
	}
}

// TestNativeFmtStdin tests reading from stdin.
func TestNativeFmtStdin(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid from stdin",
			input:   "test : 123",
			wantErr: false,
		},
		{
			name:    "invalid from stdin",
			input:   "test 123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore stdin
			oldStdin := os.Stdin
			defer func() { os.Stdin = oldStdin }()

			// Create a pipe to simulate stdin
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stdin = r

			// Write input to pipe in goroutine
			go func() {
				defer w.Close()
				io.WriteString(w, tt.input)
			}()

			// Run the command with stdin source
			native := &Native{Indent: 2}
			ctx := withStdin(t)

			err = native.Run(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Native.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestJSONFmtInvalidSyntax tests that JSON format also catches parse errors.
func TestJSONFmtInvalidSyntax(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "comma in expression",
			input:   "ewjw :123   , 12 {};",
			wantErr: false, // commas are now part of expressions, not separators
		},
		{
			name:    "valid syntax",
			input:   "test : 123",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with input
			tmpfile, err := os.CreateTemp("", "aenv-test-*.aenv")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.WriteString(tt.input); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			// Run the command with context-based source
			json := &JSON{Indent: 2}
			ctx := withSourceFile(t, tmpfile.Name())

			err = json.Run(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("JSON.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestYAMLFmtInvalidSyntax tests that YAML format also catches parse errors.
func TestYAMLFmtInvalidSyntax(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "comma in expression",
			input:   "ewjw :123   , 12 {};",
			wantErr: false, // commas are now part of expressions, not separators
		},
		{
			name:    "valid syntax",
			input:   "test : 123",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with input
			tmpfile, err := os.CreateTemp("", "aenv-test-*.aenv")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.WriteString(tt.input); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			// Run the command with context-based source
			yaml := &YAML{Indent: 2}
			ctx := withSourceFile(t, tmpfile.Name())

			err = yaml.Run(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("YAML.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestASTFmtInvalidSyntax tests that AST format also catches parse errors.
func TestASTFmtInvalidSyntax(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "comma in expression",
			input:   "ewjw :123   , 12 {};",
			wantErr: false, // commas are now part of expressions, not separators
		},
		{
			name:    "valid syntax",
			input:   "test : 123",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with input
			tmpfile, err := os.CreateTemp("", "aenv-test-*.aenv")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.WriteString(tt.input); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			// Run the command with context-based source
			ast := &AST{}
			ctx := withSourceFile(t, tmpfile.Name())

			err = ast.Run(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("AST.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestFormatASTOutput tests the formatAST function output.
func TestFormatASTOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		indent   int
		contains []string
	}{
		{
			name:   "simple definition no indent",
			input:  "test : 123",
			indent: 0,
			contains: []string{
				"test : 123",
			},
		},
		{
			name:   "simple definition with indent",
			input:  "test : 123",
			indent: 2,
			contains: []string{
				"test : 123",
			},
		},
		{
			name:   "tuple with indent",
			input:  "test : {a : 1; b : 2}",
			indent: 2,
			contains: []string{
				"test :",
				"a : 1",
				"b : 2",
			},
		},
		{
			name:   "multiple definitions with indent",
			input:  "a : 1; b : 2",
			indent: 2,
			contains: []string{
				"a : 1",
				"b : 2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with input
			tmpfile, err := os.CreateTemp("", "aenv-test-*.aenv")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.WriteString(tt.input); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Run the command with context-based source
			native := &Native{Indent: tt.indent}
			ctx := withSourceFile(t, tmpfile.Name())

			err = native.Run(ctx)

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			if err != nil {
				t.Fatalf("Native.Run() unexpected error = %v", err)
			}

			// Read captured output
			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			// Check for expected strings
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("Native.Run() output = %q, want to contain %q", output, expected)
				}
			}
		})
	}
}
