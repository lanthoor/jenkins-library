package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/SAP/jenkins-library/pkg/command"
	"github.com/SAP/jenkins-library/pkg/log"
	"github.com/SAP/jenkins-library/pkg/piperutils"
	"github.com/SAP/jenkins-library/pkg/telemetry"
	"github.com/pkg/errors"
)

type brunoExecuteUtils interface {
	RunExecutable(executable string, params ...string) error
	Getenv(key string) string
}

type brunoExecuteUtilsBundle struct {
	*command.Command
	*piperutils.Files
}

func newBrunoExecuteUtils() brunoExecuteUtils {
	utils := brunoExecuteUtilsBundle{
		Command: &command.Command{
			ErrorCategoryMapping: map[string][]string{
				log.ErrorConfiguration.String(): {
					"ENOENT: no such file or directory",
					"collection not found",
				},
				log.ErrorTest.String(): {
					"AssertionError",
					"TypeError",
					"test failed",
				},
			},
		},
		Files: &piperutils.Files{},
	}
	// Reroute command output to logging framework
	utils.Stdout(log.Writer())
	utils.Stderr(log.Writer())
	return &utils
}

func brunoExecute(config brunoExecuteOptions, _ *telemetry.CustomData, influx *brunoExecuteInflux) {
	utils := newBrunoExecuteUtils()

	influx.step_data.fields.bruno = false
	err := runBrunoExecute(&config, utils)
	if err != nil {
		log.Entry().WithError(err).Fatal("step execution failed")
	}
	influx.step_data.fields.bruno = true
}

func runBrunoExecute(config *brunoExecuteOptions, utils brunoExecuteUtils) error {
	err := logVersionsBruno(utils)
	if err != nil {
		return err
	}

	err = installBruno(config.BrunoInstallCommand, utils)
	if err != nil {
		return err
	}

	runOptions, err := resolveRunOptions(config)
	if err != nil {
		return err
	}

	// Build additional options from config parameters
	additionalOptions := buildBrunoOptions(config)
	runOptions = append(runOptions, additionalOptions...)

	brunoPath := filepath.Join(utils.Getenv("HOME"), "/.npm-global/bin/bru")
	err = utils.RunExecutable(brunoPath, runOptions...)
	if err != nil {
		if !config.FailOnError {
			log.Entry().WithError(err).Warn("Bruno tests failed, but failOnError is set to false")
			return nil
		}
		return errors.Wrap(err, "The execution of the Bruno tests failed, see the log for details.")
	}

	return nil
}

func logVersionsBruno(utils brunoExecuteUtils) error {
	err := utils.RunExecutable("node", "--version")
	if err != nil {
		log.SetErrorCategory(log.ErrorInfrastructure)
		return errors.Wrap(err, "error logging node version")
	}
	err = utils.RunExecutable("npm", "--version")
	if err != nil {
		log.SetErrorCategory(log.ErrorInfrastructure)
		return errors.Wrap(err, "error logging npm version")
	}
	return nil
}

func installBruno(brunoInstallCommand string, utils brunoExecuteUtils) error {
	installCommandTokens := strings.Split(brunoInstallCommand, " ")
	installCommandTokens = append(installCommandTokens, "--prefix=~/.npm-global")
	err := utils.RunExecutable(installCommandTokens[0], installCommandTokens[1:]...)
	if err != nil {
		log.SetErrorCategory(log.ErrorConfiguration)
		return errors.Wrap(err, "error installing Bruno CLI")
	}
	return nil
}

func buildBrunoOptions(config *brunoExecuteOptions) []string {
	options := []string{}

	// Environment options
	if config.BrunoEnvironment != "" {
		options = append(options, "--env", config.BrunoEnvironment)
	}
	if config.BrunoGlobalEnv != "" {
		options = append(options, "--global-env", config.BrunoGlobalEnv)
	}
	if config.EnvFile != "" {
		options = append(options, "--env-file", config.EnvFile)
	}
	for _, envVar := range config.EnvVars {
		options = append(options, "--env-var", envVar)
	}

	// Sandbox mode
	if config.SandboxMode != "" {
		options = append(options, "--sandbox", config.SandboxMode)
	}

	// Execution options
	if config.Recursive {
		options = append(options, "-r")
	}
	if config.Bail {
		options = append(options, "--bail")
	}
	if config.Parallel {
		options = append(options, "--parallel")
	}
	if config.TestsOnly {
		options = append(options, "--tests-only")
	}
	if config.Insecure {
		options = append(options, "--insecure")
	}
	if config.Delay > 0 {
		options = append(options, "--delay", strconv.Itoa(config.Delay))
	}

	// Data-driven testing options
	if config.CsvFilePath != "" {
		options = append(options, "--csv-file-path", config.CsvFilePath)
	}
	if config.JSONFilePath != "" {
		options = append(options, "--json-file-path", config.JSONFilePath)
	}
	if config.IterationCount > 0 {
		options = append(options, "--iteration-count", strconv.Itoa(config.IterationCount))
	}

	// Tag filtering options
	if config.Tags != "" {
		options = append(options, "--tags", config.Tags)
	}
	if config.ExcludeTags != "" {
		options = append(options, "--exclude-tags", config.ExcludeTags)
	}

	// Reporter options (only if not already in runOptions via templating)
	if config.ReporterJSON != "" {
		options = append(options, "--reporter-json", config.ReporterJSON)
	}
	if config.ReporterJunit != "" && !containsReporterJunit(config.RunOptions) {
		options = append(options, "--reporter-junit", config.ReporterJunit)
	}
	if config.ReporterHtml != "" && !containsReporterHtml(config.RunOptions) {
		options = append(options, "--reporter-html", config.ReporterHtml)
	}
	if config.ReporterSkipAllHeaders {
		options = append(options, "--reporter-skip-all-headers")
	}
	for _, header := range config.ReporterSkipHeaders {
		options = append(options, "--reporter-skip-headers", header)
	}

	return options
}

func containsReporterJunit(runOptions []string) bool {
	for _, opt := range runOptions {
		if strings.Contains(opt, "--reporter-junit") {
			return true
		}
	}
	return false
}

func containsReporterHtml(runOptions []string) bool {
	for _, opt := range runOptions {
		if strings.Contains(opt, "--reporter-html") {
			return true
		}
	}
	return false
}

func resolveRunOptions(config *brunoExecuteOptions) ([]string, error) {
	cmd := []string{}
	collectionDisplayName := defineBrunoCollectionDisplayName(config.BrunoCollection)

	type TemplateConfig struct {
		Config                interface{}
		CollectionDisplayName string
		BrunoCollection       string
	}

	for _, runOption := range config.RunOptions {
		templ, err := template.New("template").Funcs(template.FuncMap{
			"getenv": func(varName string) string {
				return os.Getenv(varName)
			},
		}).Parse(runOption)
		if err != nil {
			log.SetErrorCategory(log.ErrorConfiguration)
			return nil, errors.Wrap(err, "could not parse Bruno command template")
		}
		buf := new(bytes.Buffer)
		err = templ.Execute(buf, TemplateConfig{
			Config:                config,
			CollectionDisplayName: collectionDisplayName,
			BrunoCollection:       config.BrunoCollection,
		})
		if err != nil {
			log.SetErrorCategory(log.ErrorConfiguration)
			return nil, errors.Wrap(err, "error on executing template")
		}
		cmd = append(cmd, buf.String())
	}

	return cmd, nil
}

func defineBrunoCollectionDisplayName(collection string) string {
	replacedSeparators := strings.Replace(collection, string(filepath.Separator), "_", -1)
	displayName := strings.Split(replacedSeparators, ".")
	if displayName[0] == "" && len(displayName) >= 2 {
		return displayName[1]
	}
	return displayName[0]
}

func (utils brunoExecuteUtilsBundle) Getenv(key string) string {
	return os.Getenv(key)
}
