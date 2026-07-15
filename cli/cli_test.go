package cli

import (
	"context"
	"testing"
)

func TestRun_ReturnsNil(t *testing.T) {
	t.Parallel()

	if err := Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}
