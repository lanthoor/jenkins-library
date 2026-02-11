//go:build unit
// +build unit

package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

type executedBrunoExecutables struct {
	executable string
	params     []string
}

type brunoExecuteMockUtils struct {
	errorOnBrunoInstall   bool
	errorOnRunShell       bool
	errorOnBrunoExecution bool
	errorOnLoggingNode    bool
	errorOnLoggingNpm     bool
	executedExecutables   []executedBrunoExecutables
	commandIndex          int
}

func newBrunoExecuteMockUtils() brunoExecuteMockUtils {
	return brunoExecuteMockUtils{}
}

func TestRunBrunoExecute(t *testing.T) {
	t.Parallel()

	defaultConfig := brunoExecuteOptions{
		BrunoCollection:     "api-tests",
		BrunoInstallCommand: "npm install @usebruno/cli --global --quiet",
		RunOptions: []string{
			"run",
			"{{.BrunoCollection}}",
			"--reporter-junit",
			"target/bruno/TEST-{{.CollectionDisplayName}}.xml",
			"--reporter-html",
			"target/bruno/TEST-{{.CollectionDisplayName}}.html",
		},
		SandboxMode: "safe",
		FailOnError: true,
	}

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err)
		assert.Contains(t, utils.executedExecutables, executedBrunoExecutables{executable: "node", params: []string{"--version"}})
		assert.Contains(t, utils.executedExecutables, executedBrunoExecutables{executable: "npm", params: []string{"--version"}})
		assert.Contains(t, utils.executedExecutables, executedBrunoExecutables{executable: "npm", params: []string{"install", "@usebruno/cli", "--global", "--quiet", "--prefix=~/.npm-global"}})
		assert.Contains(t, utils.executedExecutables, executedBrunoExecutables{
			executable: filepath.FromSlash("/home/node/.npm-global/bin/bru"),
			params: []string{
				"run", "api-tests",
				"--reporter-junit", "target/bruno/TEST-api-tests.xml",
				"--reporter-html", "target/bruno/TEST-api-tests.html",
				"--sandbox", "safe",
			},
		})
	})

	t.Run("happy path with fail on error false", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		utils.errorOnBrunoExecution = true
		config := defaultConfig
		config.FailOnError = false

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err) // Should not fail because failOnError is false
	})

	t.Run("with environment", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig
		config.BrunoEnvironment = "ci"

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err)
		// Check that --env ci is in the params
		found := false
		for _, exec := range utils.executedExecutables {
			if strings.Contains(exec.executable, "bru") {
				for i, param := range exec.params {
					if param == "--env" && i+1 < len(exec.params) && exec.params[i+1] == "ci" {
						found = true
						break
					}
				}
			}
		}
		assert.True(t, found, "Expected --env ci in Bruno command")
	})

	t.Run("with global environment", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig
		config.BrunoGlobalEnv = "global-ci"

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err)
		found := false
		for _, exec := range utils.executedExecutables {
			if strings.Contains(exec.executable, "bru") {
				for i, param := range exec.params {
					if param == "--global-env" && i+1 < len(exec.params) && exec.params[i+1] == "global-ci" {
						found = true
						break
					}
				}
			}
		}
		assert.True(t, found, "Expected --global-env global-ci in Bruno command")
	})

	t.Run("with env vars", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig
		config.EnvVars = []string{"API_KEY=secret123", "BASE_URL=https://api.test.com"}

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err)
		envVarCount := 0
		for _, exec := range utils.executedExecutables {
			if strings.Contains(exec.executable, "bru") {
				for i, param := range exec.params {
					if param == "--env-var" && i+1 < len(exec.params) {
						envVarCount++
					}
				}
			}
		}
		assert.Equal(t, 2, envVarCount, "Expected 2 --env-var options")
	})

	t.Run("with parallel execution", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig
		config.Parallel = true

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err)
		found := false
		for _, exec := range utils.executedExecutables {
			if strings.Contains(exec.executable, "bru") {
				for _, param := range exec.params {
					if param == "--parallel" {
						found = true
						break
					}
				}
			}
		}
		assert.True(t, found, "Expected --parallel in Bruno command")
	})

	t.Run("with recursive flag", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig
		config.Recursive = true

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err)
		found := false
		for _, exec := range utils.executedExecutables {
			if strings.Contains(exec.executable, "bru") {
				for _, param := range exec.params {
					if param == "-r" {
						found = true
						break
					}
				}
			}
		}
		assert.True(t, found, "Expected -r in Bruno command")
	})

	t.Run("with bail flag", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig
		config.Bail = true

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err)
		found := false
		for _, exec := range utils.executedExecutables {
			if strings.Contains(exec.executable, "bru") {
				for _, param := range exec.params {
					if param == "--bail" {
						found = true
						break
					}
				}
			}
		}
		assert.True(t, found, "Expected --bail in Bruno command")
	})

	t.Run("with developer sandbox mode", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig
		config.SandboxMode = "developer"

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err)
		found := false
		for _, exec := range utils.executedExecutables {
			if strings.Contains(exec.executable, "bru") {
				for i, param := range exec.params {
					if param == "--sandbox" && i+1 < len(exec.params) && exec.params[i+1] == "developer" {
						found = true
						break
					}
				}
			}
		}
		assert.True(t, found, "Expected --sandbox developer in Bruno command")
	})

	t.Run("with CSV data file", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig
		config.CsvFilePath = "test-data.csv"

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err)
		found := false
		for _, exec := range utils.executedExecutables {
			if strings.Contains(exec.executable, "bru") {
				for i, param := range exec.params {
					if param == "--csv-file-path" && i+1 < len(exec.params) && exec.params[i+1] == "test-data.csv" {
						found = true
						break
					}
				}
			}
		}
		assert.True(t, found, "Expected --csv-file-path test-data.csv in Bruno command")
	})

	t.Run("with JSON data file", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig
		config.JSONFilePath = "test-data.json"

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err)
		found := false
		for _, exec := range utils.executedExecutables {
			if strings.Contains(exec.executable, "bru") {
				for i, param := range exec.params {
					if param == "--json-file-path" && i+1 < len(exec.params) && exec.params[i+1] == "test-data.json" {
						found = true
						break
					}
				}
			}
		}
		assert.True(t, found, "Expected --json-file-path test-data.json in Bruno command")
	})

	t.Run("with tags filtering", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig
		config.Tags = "smoke,critical"

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err)
		found := false
		for _, exec := range utils.executedExecutables {
			if strings.Contains(exec.executable, "bru") {
				for i, param := range exec.params {
					if param == "--tags" && i+1 < len(exec.params) && exec.params[i+1] == "smoke,critical" {
						found = true
						break
					}
				}
			}
		}
		assert.True(t, found, "Expected --tags smoke,critical in Bruno command")
	})

	t.Run("with exclude tags", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig
		config.ExcludeTags = "slow,flaky"

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err)
		found := false
		for _, exec := range utils.executedExecutables {
			if strings.Contains(exec.executable, "bru") {
				for i, param := range exec.params {
					if param == "--exclude-tags" && i+1 < len(exec.params) && exec.params[i+1] == "slow,flaky" {
						found = true
						break
					}
				}
			}
		}
		assert.True(t, found, "Expected --exclude-tags slow,flaky in Bruno command")
	})

	t.Run("with tests only", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig
		config.TestsOnly = true

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.NoError(t, err)
		found := false
		for _, exec := range utils.executedExecutables {
			if strings.Contains(exec.executable, "bru") {
				for _, param := range exec.params {
					if param == "--tests-only" {
						found = true
						break
					}
				}
			}
		}
		assert.True(t, found, "Expected --tests-only in Bruno command")
	})

	t.Run("error on Bruno execution", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		utils.errorOnBrunoExecution = true
		config := defaultConfig

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.EqualError(t, err, "The execution of the Bruno tests failed, see the log for details.: error on Bruno execution")
	})

	t.Run("error on Bruno installation", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		utils.errorOnBrunoInstall = true
		config := defaultConfig

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.EqualError(t, err, "error installing Bruno CLI: error on Bruno install")
	})

	t.Run("error on npm version logging", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		utils.errorOnLoggingNpm = true
		config := defaultConfig

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.EqualError(t, err, "error logging npm version: error on RunExecutable")
	})

	t.Run("error on node version logging", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		utils.errorOnLoggingNode = true
		config := defaultConfig

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.EqualError(t, err, "error logging node version: error on RunExecutable")
	})

	t.Run("error on template resolution", func(t *testing.T) {
		t.Parallel()
		// init
		utils := newBrunoExecuteMockUtils()
		config := defaultConfig
		config.RunOptions = []string{"run", "{{.InvalidField}"}

		// test
		err := runBrunoExecute(&config, &utils)

		// assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not parse Bruno command template")
	})
}

func TestDefineBrunoCollectionDisplayName(t *testing.T) {
	t.Parallel()

	t.Run("simple directory name", func(t *testing.T) {
		t.Parallel()
		result := defineBrunoCollectionDisplayName("api-tests")
		assert.Equal(t, "api-tests", result)
	})

	t.Run("nested path", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join("tests", "integration", "api-tests")
		result := defineBrunoCollectionDisplayName(path)
		assert.Equal(t, "tests_integration_api-tests", result)
	})

	t.Run("path with dot prefix", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(".tests", "api-tests")
		result := defineBrunoCollectionDisplayName(path)
		assert.Equal(t, "tests_api-tests", result)
	})
}

func TestResolveRunOptions(t *testing.T) {
	t.Parallel()

	t.Run("nothing to replace", func(t *testing.T) {
		t.Parallel()
		config := brunoExecuteOptions{
			BrunoCollection: "api-tests",
			RunOptions:      []string{"run", "my-collection"},
		}

		cmd, err := resolveRunOptions(&config)
		assert.NoError(t, err)
		assert.Equal(t, []string{"run", "my-collection"}, cmd)
	})

	t.Run("replace collection", func(t *testing.T) {
		t.Parallel()
		config := brunoExecuteOptions{
			BrunoCollection: "my-api-tests",
			RunOptions:      []string{"run", "{{.BrunoCollection}}"},
		}

		cmd, err := resolveRunOptions(&config)
		assert.NoError(t, err)
		assert.Equal(t, []string{"run", "my-api-tests"}, cmd)
	})

	t.Run("replace display name", func(t *testing.T) {
		t.Parallel()
		config := brunoExecuteOptions{
			BrunoCollection: "api-tests",
			RunOptions:      []string{"run", "{{.BrunoCollection}}", "--reporter-junit", "TEST-{{.CollectionDisplayName}}.xml"},
		}

		cmd, err := resolveRunOptions(&config)
		assert.NoError(t, err)
		assert.Equal(t, []string{"run", "api-tests", "--reporter-junit", "TEST-api-tests.xml"}, cmd)
	})

	t.Run("get environment variable", func(t *testing.T) {
		t.Parallel()
		temporaryEnvVarName := uuid.New().String()
		os.Setenv(temporaryEnvVarName, "myEnvVar")
		defer os.Unsetenv(temporaryEnvVarName)

		config := brunoExecuteOptions{
			BrunoCollection: "api-tests",
			RunOptions:      []string{"run", "{{.BrunoCollection}}", "--env-var", "key={{getenv \"" + temporaryEnvVarName + "\"}}"},
		}

		cmd, err := resolveRunOptions(&config)
		assert.NoError(t, err)
		assert.Equal(t, []string{"run", "api-tests", "--env-var", "key=myEnvVar"}, cmd)
	})

	t.Run("error when template cannot be parsed", func(t *testing.T) {
		t.Parallel()
		config := brunoExecuteOptions{
			BrunoCollection: "api-tests",
			RunOptions:      []string{"run", "{{.InvalidField}"},
		}

		_, err := resolveRunOptions(&config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not parse Bruno command template")
	})
}

func TestBuildBrunoOptions(t *testing.T) {
	t.Parallel()

	t.Run("empty config returns minimal options", func(t *testing.T) {
		t.Parallel()
		config := brunoExecuteOptions{
			SandboxMode: "safe",
		}

		options := buildBrunoOptions(&config)
		assert.Contains(t, options, "--sandbox")
		assert.Contains(t, options, "safe")
	})

	t.Run("all options set", func(t *testing.T) {
		t.Parallel()
		config := brunoExecuteOptions{
			BrunoEnvironment:       "ci",
			BrunoGlobalEnv:         "global",
			EnvFile:                "env.json",
			EnvVars:                []string{"KEY=value"},
			SandboxMode:            "developer",
			Recursive:              true,
			Bail:                   true,
			Parallel:               true,
			TestsOnly:              true,
			Insecure:               true,
			Tags:                   "smoke",
			ExcludeTags:            "slow",
			CsvFilePath:            "data.csv",
			JSONFilePath:           "data.json",
			ReporterJSON:           "report.json",
			ReporterSkipAllHeaders: true,
			ReporterSkipHeaders:    []string{"Authorization"},
		}

		options := buildBrunoOptions(&config)
		assert.Contains(t, options, "--env")
		assert.Contains(t, options, "ci")
		assert.Contains(t, options, "--global-env")
		assert.Contains(t, options, "global")
		assert.Contains(t, options, "--env-file")
		assert.Contains(t, options, "env.json")
		assert.Contains(t, options, "--env-var")
		assert.Contains(t, options, "KEY=value")
		assert.Contains(t, options, "--sandbox")
		assert.Contains(t, options, "developer")
		assert.Contains(t, options, "-r")
		assert.Contains(t, options, "--bail")
		assert.Contains(t, options, "--parallel")
		assert.Contains(t, options, "--tests-only")
		assert.Contains(t, options, "--insecure")
		assert.Contains(t, options, "--tags")
		assert.Contains(t, options, "smoke")
		assert.Contains(t, options, "--exclude-tags")
		assert.Contains(t, options, "slow")
		assert.Contains(t, options, "--csv-file-path")
		assert.Contains(t, options, "data.csv")
		assert.Contains(t, options, "--json-file-path")
		assert.Contains(t, options, "data.json")
		assert.Contains(t, options, "--reporter-json")
		assert.Contains(t, options, "report.json")
		assert.Contains(t, options, "--reporter-skip-all-headers")
		assert.Contains(t, options, "--reporter-skip-headers")
		assert.Contains(t, options, "Authorization")
	})
}

// Mock implementations

func (e *brunoExecuteMockUtils) RunExecutable(executable string, params ...string) error {
	if e.errorOnRunShell {
		return errors.New("error on RunExecutable")
	}
	if e.errorOnLoggingNode && executable == "node" && params[0] == "--version" {
		return errors.New("error on RunExecutable")
	}
	if e.errorOnLoggingNpm && executable == "npm" && params[0] == "--version" {
		return errors.New("error on RunExecutable")
	}
	if e.errorOnBrunoExecution && strings.Contains(executable, "bru") {
		return errors.New("error on Bruno execution")
	}
	if e.errorOnBrunoInstall && slices.Contains(params, "install") {
		return errors.New("error on Bruno install")
	}

	length := len(e.executedExecutables)
	if length < e.commandIndex+1 {
		e.executedExecutables = append(e.executedExecutables, executedBrunoExecutables{})
		length++
	}

	e.executedExecutables[length-1].executable = executable
	e.executedExecutables[length-1].params = params
	e.commandIndex++

	return nil
}

func (e *brunoExecuteMockUtils) Getenv(key string) string {
	if key == "HOME" {
		return "/home/node"
	}
	return ""
}
