package test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneralCLI(t *testing.T) {
	t.Log("E2E: Testing general CLI")
	e2e.setup(t)
	defer e2e.teardown(t)

	// Test `zarf prepare sha256sum` for a local asset
	expectedShasum := "61b50898f982d015ed87093ba822de0fe011cec6dd67db39f99d8c56391a6109\n"

	// TODO: There has to be a better way to pipe this output to the file.. For some reason exec.Command( ... > file ).Output() couldn't pipe to file
	// output, err = exec.Command("bash", "-c", "\"echo 'random test data 🦄' > shasum-test-file\"").Output()
	shasumTestFilePath := "shasum-test-file"

	e2e.cleanFiles(shasumTestFilePath)

	testfile, _ := os.Create(shasumTestFilePath)
	cmd := exec.Command("echo", "random test data 🦄")
	cmd.Stdout = testfile
	_ = cmd.Run()

	output, err := e2e.execZarfCommand("prepare", "sha256sum", shasumTestFilePath)
	assert.NoError(t, err, output)
	assert.Equal(t, expectedShasum, output, "The expected SHASUM should equal the actual SHASUM")

	// Test `zarf prepare sha256sum` for a remote asset
	expectedShasum = "c3cdea0573ba5a058ec090b5d2683bf398e8b1614c37ec81136ed03b78167617\n"

	output, err = e2e.execZarfCommand("prepare", "sha256sum", "https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/zarf-prepare-shasum-remote-test-file.txt")
	assert.NoError(t, err, output)
	assert.Contains(t, output, expectedShasum, "The expected SHASUM should equal the actual SHASUM")

	// Test `zarf version`
	output, err = e2e.execZarfCommand("version")
	assert.NoError(t, err)
	assert.NotEqual(t, len(output), 0, "Zarf version should not be an empty string")
	assert.NotEqual(t, output, "UnknownVersion", "Zarf version should not be the default value")

	// Test for expected failure when given a bad componenet input
	_, err = e2e.execZarfCommand("init", "--confirm", "--components=k3s,foo,logging")
	assert.Error(t, err)

	// Test that changing the log level actually applies the requested level
	output, _ = e2e.execZarfCommand("version", "--log-level=debug")
	expectedOutString := "Log level set to debug"
	require.Contains(t, output, expectedOutString, "The log level should be changed to 'debug'")

	// Test that `zarf package deploy` gives an error if deploying a remote package without the --insecure or --shasum flags
	output, err = e2e.execZarfCommand("package", "deploy", "https://zarf-examples.s3.amazonaws.com/zarf-package-appliance-demo-doom-20210125.tar.zst", "--confirm")
	assert.Error(t, err, output)

	e2e.cleanFiles(shasumTestFilePath)
}
