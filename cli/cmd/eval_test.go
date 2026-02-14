package cmd

// import (
// 	"context"
// 	"os"
// 	"path/filepath"
// 	"strings"
// 	"testing"

// 	"github.com/alecthomas/kong"
// )

// // TestEvalRun tests the Eval.Run command.
// func TestEvalRun(t *testing.T) {
// 	t.Parallel()

// 	// Check if we have a TTY - REPL requires it
// 	if _, err := os.Open("/dev/tty"); err != nil {
// 		t.Skip("Skipping REPL test: no TTY available")
// 	}

// 	tests := []struct {
// 		name    string
// 		source  string
// 		wantErr bool
// 	}{
// 		{
// 			name:    "simple_namespace",
// 			source:  "test : 123",
// 			wantErr: false,
// 		},
// 		{
// 			name:    "namespace_with_tuple",
// 			source:  "test : {a : 1; b : 2}",
// 			wantErr: false,
// 		},
// 		{
// 			name:    "invalid_syntax",
// 			source:  "invalid : : :",
// 			wantErr: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			t.Parallel()

// 			// Create temp directory for cache
// 			tmpCache, err := os.MkdirTemp("", "aenv-eval-cache-*")
// 			if err != nil {
// 				t.Fatal(err)
// 			}
// 			defer os.RemoveAll(tmpCache)

// 			// Create temp source file
// 			tmpfile, err := os.CreateTemp("", "aenv-eval-*.aenv")
// 			if err != nil {
// 				t.Fatal(err)
// 			}
// 			defer os.Remove(tmpfile.Name())

// 			if _, err := tmpfile.WriteString(tt.source); err != nil {
// 				t.Fatal(err)
// 			}
// 			if err := tmpfile.Close(); err != nil {
// 				t.Fatal(err)
// 			}

// 			// Create Kong context with cache identifier
// 			var cli struct{}
// 			parser, err := kong.New(&cli, kong.Vars{
// 				CacheIdentifier: tmpCache,
// 			})
// 			if err != nil {
// 				t.Fatal(err)
// 			}

// 			kctx, err := parser.Parse(nil)
// 			if err != nil {
// 				t.Fatal(err)
// 			}

// 			// Create context with Kong and source files
// 			ctx := WithContext(context.Background(), kctx)
// 			ctx = WithSourceFiles(ctx, []string{tmpfile.Name()})

// 			// Run eval command
// 			evalCmd := &Eval{
// 				Name: "test",
// 				Args: []string{},
// 			}

// 			err = evalCmd.Run(ctx)

// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("Eval.Run() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// // TestEvalWithArgs tests eval with namespace arguments.
// func TestEvalWithArgs(t *testing.T) {
// 	t.Parallel()

// 	// Check if we have a TTY - REPL requires it
// 	if _, err := os.Open("/dev/tty"); err != nil {
// 		t.Skip("Skipping REPL test: no TTY available")
// 	}

// 	// Create temp cache directory
// 	tmpCache, err := os.MkdirTemp("", "aenv-eval-cache-*")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer os.RemoveAll(tmpCache)

// 	// Create source file with parameterized namespace
// 	tmpfile, err := os.CreateTemp("", "aenv-eval-*.aenv")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer os.Remove(tmpfile.Name())

// 	// Use valid aenv syntax - namespace with simple definition
// 	source := "greet : {greeting : \"Hello\"}"
// 	if _, err := tmpfile.WriteString(source); err != nil {
// 		t.Fatal(err)
// 	}
// 	if err := tmpfile.Close(); err != nil {
// 		t.Fatal(err)
// 	}

// 	// Create Kong context
// 	var cli struct{}
// 	parser, err := kong.New(&cli, kong.Vars{
// 		CacheIdentifier: tmpCache,
// 	})
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	kctx, err := parser.Parse(nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	ctx := WithContext(context.Background(), kctx)
// 	ctx = WithSourceFiles(ctx, []string{tmpfile.Name()})

// 	// Run eval with arguments
// 	evalCmd := &Eval{
// 		Name: "greet",
// 		Args: []string{},
// 	}

// 	err = evalCmd.Run(ctx)
// 	if err != nil {
// 		t.Errorf("Eval.Run() with args unexpected error = %v", err)
// 	}
// }

// // TestEvalWithMissingCache tests eval without cache directory.
// func TestEvalWithMissingCache(t *testing.T) {
// 	t.Parallel()

// 	// Create Kong context without CacheIdentifier
// 	var cli struct{}
// 	parser, err := kong.New(&cli)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	kctx, err := parser.Parse(nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	ctx := WithContext(context.Background(), kctx)

// 	// Run eval - should panic
// 	evalCmd := &Eval{Name: "test"}

// 	// Capture panic
// 	defer func() {
// 		if r := recover(); r == nil {
// 			t.Error("Eval.Run() expected panic for missing cache, got none")
// 		}
// 	}()

// 	_ = evalCmd.Run(ctx)
// }

// // TestEvalWithMultipleSourceFiles tests eval with multiple source files.
// func TestEvalWithMultipleSourceFiles(t *testing.T) {
// 	t.Parallel()

// 	// Check if we have a TTY - REPL requires it
// 	if _, err := os.Open("/dev/tty"); err != nil {
// 		t.Skip("Skipping REPL test: no TTY available")
// 	}

// 	// Create temp cache directory
// 	tmpCache, err := os.MkdirTemp("", "aenv-eval-cache-*")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer os.RemoveAll(tmpCache)

// 	// Create temp directory for source files
// 	tmpDir, err := os.MkdirTemp("", "aenv-eval-sources-*")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer os.RemoveAll(tmpDir)

// 	// Create first source file
// 	file1 := filepath.Join(tmpDir, "file1.aenv")
// 	if err := os.WriteFile(file1, []byte("first : 1"), 0644); err != nil {
// 		t.Fatal(err)
// 	}

// 	// Create second source file
// 	file2 := filepath.Join(tmpDir, "file2.aenv")
// 	if err := os.WriteFile(file2, []byte("second : 2"), 0644); err != nil {
// 		t.Fatal(err)
// 	}

// 	// Create Kong context
// 	var cli struct{}
// 	parser, err := kong.New(&cli, kong.Vars{
// 		CacheIdentifier: tmpCache,
// 	})
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	kctx, err := parser.Parse(nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	ctx := WithContext(context.Background(), kctx)
// 	ctx = WithSourceFiles(ctx, []string{file1, file2})

// 	// Run eval
// 	evalCmd := &Eval{Name: "first"}
// 	err = evalCmd.Run(ctx)
// 	if err != nil {
// 		t.Errorf("Eval.Run() with multiple sources unexpected error = %v", err)
// 	}
// }

// // TestEvalStructFields tests the Eval struct field assignments.
// func TestEvalStructFields(t *testing.T) {
// 	t.Parallel()

// 	tests := []struct {
// 		name     string
// 		evalName string
// 		args     []string
// 	}{
// 		{
// 			name:     "empty_name_no_args",
// 			evalName: "",
// 			args:     []string{},
// 		},
// 		{
// 			name:     "with_name_no_args",
// 			evalName: "test",
// 			args:     []string{},
// 		},
// 		{
// 			name:     "with_name_and_args",
// 			evalName: "test",
// 			args:     []string{"arg1", "arg2", "arg3"},
// 		},
// 		{
// 			name:     "empty_name_with_args",
// 			evalName: "",
// 			args:     []string{"arg1"},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			t.Parallel()

// 			eval := &Eval{
// 				Name: tt.evalName,
// 				Args: tt.args,
// 			}

// 			if eval.Name != tt.evalName {
// 				t.Errorf("Eval.Name = %v, want %v", eval.Name, tt.evalName)
// 			}

// 			if len(eval.Args) != len(tt.args) {
// 				t.Errorf("len(Eval.Args) = %v, want %v", len(eval.Args), len(tt.args))
// 			}

// 			for i, arg := range tt.args {
// 				if eval.Args[i] != arg {
// 					t.Errorf("Eval.Args[%d] = %v, want %v", i, eval.Args[i], arg)
// 				}
// 			}
// 		})
// 	}
// }

// // TestEvalValidatesContext tests that Eval.Run validates Kong context properly.
// func TestEvalValidatesContext(t *testing.T) {
// 	t.Parallel()

// 	tests := []struct {
// 		name        string
// 		setupCtx    func() context.Context
// 		expectPanic bool
// 		panicMsg    string
// 	}{
// 		{
// 			name: "missing_cache_identifier",
// 			setupCtx: func() context.Context {
// 				var cli struct{}
// 				parser, _ := kong.New(&cli)
// 				kctx, _ := parser.Parse(nil)
// 				return WithContext(context.Background(), kctx)
// 			},
// 			expectPanic: true,
// 			panicMsg:    "cache directory",
// 		},
// 		{
// 			name: "with_cache_identifier_no_source",
// 			setupCtx: func() context.Context {
// 				var cli struct{}
// 				parser, _ := kong.New(&cli, kong.Vars{
// 					CacheIdentifier: "/tmp/cache",
// 				})
// 				kctx, _ := parser.Parse(nil)
// 				return WithContext(context.Background(), kctx)
// 			},
// 			expectPanic: false, // Will error but not panic
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			t.Parallel()

// 			ctx := tt.setupCtx()
// 			evalCmd := &Eval{Name: "test"}

// 			if tt.expectPanic {
// 				defer func() {
// 					r := recover()
// 					if r == nil {
// 						t.Error("Expected panic but got none")
// 					} else if tt.panicMsg != "" {
// 						panicStr := r.(string)
// 						if !strings.Contains(panicStr, tt.panicMsg) {
// 							t.Errorf("Panic message = %v, want to contain %v", panicStr, tt.panicMsg)
// 						}
// 					}
// 				}()
// 			}

// 			_ = evalCmd.Run(ctx)
// 		})
// 	}
// }
