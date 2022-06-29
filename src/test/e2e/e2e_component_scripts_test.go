package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestE2eComponentScripts(t *testing.T) {
	t.Log("E2E: Testing component scripts")
	e2e.setup(t)
	defer e2e.teardown(t)

	path := fmt.Sprintf("build/zarf-package-component-scripts-%s.tar.zst", e2e.arch)

	// Deploy the simple script that should pass
	output, err := e2e.execZarfCommand("package", "deploy", path, "--confirm", "--components=passes")
	require.NoError(t, err, output)

	// Deploy the simple script that should fail the timeout
	output, err = e2e.execZarfCommand("package", "deploy", path, "--confirm", "--components=does-not-pass")
	require.Error(t, err, output)
}
