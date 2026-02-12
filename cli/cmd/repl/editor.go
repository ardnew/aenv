package repl

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/lang/lexer"
	"github.com/ardnew/aenv/log"
)

const defaultEditor = "vi"

// editASTCommand implements [tea.ExecCommand] for the full AST
// edit-parse-retry loop. It formats the current AST to a temp file, opens the
// user's editor, and re-parses the result. On parse error the user is prompted
// to re-edit; declining exits the program.
type editASTCommand struct {
	ast     *lang.AST
	ctxFunc func() context.Context
	newAST  *lang.AST
	logger  log.Logger
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
}

// SetStdin sets the stdin reader for the command.
func (c *editASTCommand) SetStdin(r io.Reader) { c.stdin = r }

// SetStdout sets the stdout writer for the command.
func (c *editASTCommand) SetStdout(w io.Writer) { c.stdout = w }

// SetStderr sets the stderr writer for the command.
func (c *editASTCommand) SetStderr(w io.Writer) { c.stderr = w }

// Run executes the edit-parse-retry loop. It formats the AST, opens the
// editor, parses the result, and prompts on error. If the user declines to
// re-edit, it returns [ErrEditDeclined].
func (c *editASTCommand) Run() error {
	ctx := c.ctxFunc()

	// Format AST to native syntax.
	var buf bytes.Buffer
	if err := c.ast.Format(ctx, &buf, 2); err != nil {
		return fmt.Errorf("format AST: %w", err)
	}

	content := buf.String()

	// Create a single temp file for the entire loop.
	f, err := os.CreateTemp(os.TempDir(), "aenv-repl-*.aenv")
	if err != nil {
		return err
	}

	tmpPath := f.Name()

	defer os.Remove(tmpPath)

	if err := f.Chmod(0o600); err != nil {
		f.Close()

		return err
	}

	f.Close()

	for {
		// Write current content to temp file.
		if err := os.WriteFile(tmpPath, []byte(content), 0o600); err != nil {
			return err
		}

		// Launch editor and get a reader over the result.
		r, err := runEditor(ctx, c.stdin, c.stdout, c.stderr, tmpPath)
		if err != nil {
			return err
		}

		// Check for empty file (user cleared content).
		br := bufio.NewReader(r)
		if _, err := br.Peek(1); err != nil {
			// EOF or read error; treat as cancelled edit.
			return nil
		}

		// Read all content from the buffered reader and parse via the lexer.
		data, err := io.ReadAll(br)
		if err != nil {
			return err
		}

		newAST, parseErr := lang.Parse(
			ctx,
			lexer.New([]rune(string(data))),
			lang.WithCompileExprs(true),
			lang.WithLogger(c.logger),
		)
		c.logger.TraceContext(
			ctx,
			"editor parse attempt",
			slog.Int("content_length", len(data)),
			slog.Bool("success", parseErr == nil),
		)

		if parseErr == nil {
			c.newAST = newAST

			return nil
		}

		// Show error and prompt.
		fmt.Fprintf(c.stderr, "\nParse error: %s\n", parseErr)
		fmt.Fprintf(c.stdout, "Re-edit? [Y/n] ")

		scanner := bufio.NewScanner(c.stdin)
		if !scanner.Scan() {
			return ErrEditDeclined
		}

		response := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if response == "n" || response == "no" {
			return ErrEditDeclined
		}

		// Re-read the (failed) content for the next editor iteration.
		data, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			return readErr
		}

		content = string(data)
	}
}

// runEditor launches the user's editor on the given file path and returns a
// reader over the edited file content.
func runEditor(
	ctx context.Context,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	path string,
) (io.Reader, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = defaultEditor
	}

	cmd := exec.CommandContext(ctx, editor, path)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return f, nil
}
