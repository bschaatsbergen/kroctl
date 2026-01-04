package command

import (
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bschaatsbergen/kroctl/internal/view"
	"github.com/bschaatsbergen/kroctl/version"
)

var (
	jsonFlag  bool
	debugFlag bool
	rootCmd   *cobra.Command
)

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "kroctl",
		Short: color.RGB(50, 108, 229).Sprintf("kroctl [global options] <subcommand> [args]") + `\n` +
			"A utility to work with kro's resource graph definitions and package them as OCI artifacts",
		Long: color.RGB(50, 108, 229).Sprintf("Usage: kroctl [global options] <subcommand> [args]\n") +
			`
 __
|  | _________  ____
|  |/ /\_  __ \/  _ \
|    <  |  | \(  <_> )
|__|_ \ |__|   \____/
     \/
		` + "\n" +
			"kroctl is a CLI utility for working with kro ResourceGraphDefinitions\n" +
			"(RGDs) and packaging them as OCI artifacts for distribution and reuse\n" +
			"across Kubernetes clusters.\n\n" +
			"kro (Kube Resource Orchestrator) is a Kubernetes-native project that\n" +
			"lets you define custom Kubernetes APIs using simple configuration.\n" +
			"ResourceGraphDefinitions bundle multiple Kubernetes resources together\n" +
			"with logical operations, conditions, and dependencies using CEL\n" +
			"(Common Expression Language). Platform teams use RGDs to encapsulate\n" +
			"best practices and security policies, while development teams consume\n" +
			"these simplified APIs to deploy complex application stacks.\n\n" +
			"With kroctl, you can package RGDs as OCI artifacts and publish them to\n" +
			"OCI-compliant registries for sharing and distribution across teams and\n" +
			"clusters.\n\n" +
			"Learn more about kro at https://kro.run\n\n",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				_ = cmd.Help()
			}
		},
	}

	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	cmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Set log level to debug")
	return cmd
}

func setCobraUsageTemplate() {
	cobra.AddTemplateFunc("StyleHeading", color.RGB(50, 108, 229).SprintFunc())
	usageTemplate := rootCmd.UsageTemplate()
	usageTemplate = strings.NewReplacer(
		`Usage:`, `{{StyleHeading "Usage:"}}`,
		`Examples:`, `{{StyleHeading "Examples:"}}`,
		`Available Commands:`, `{{StyleHeading "Available Commands:"}}`,
		`Additional Commands:`, `{{StyleHeading "Additional Commands:"}}`,
		`Flags:`, `{{StyleHeading "Options:"}}`,
		`Global Flags:`, `{{StyleHeading "Global Options:"}}`,
	).Replace(usageTemplate)
	rootCmd.SetUsageTemplate(usageTemplate)
}

func setVersionTemplate() {
	rootCmd.SetVersionTemplate("{{.Version}}")
}

func Execute() {
	rootCmd = NewRootCommand()

	// Templates are used to standardize the output format of kroctl.
	setCobraUsageTemplate()
	setVersionTemplate()

	// Parse flags early so the root command is aware of global flags
	// before any subcommand executes. This is necessary to configure
	// things like the output format (view type) and writer upfront.
	_ = rootCmd.ParseFlags(os.Args[1:])

	// Disable color output if NO_COLOR is set in the environment
	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		color.NoColor = true
	} else {
		color.NoColor = false
	}

	// Set up the view type based on the `--json` flag
	viewType := view.ViewHuman
	if jsonFlag {
		viewType = view.ViewJSON
	}

	logLevel := view.LogLevelSilent
	logEnv := os.Getenv("KROCTL_LOG")
	switch strings.ToLower(logEnv) {
	case "debug":
		logLevel = view.LogLevelDebug
	case "info":
		logLevel = view.LogLevelInfo
	default:
		// Unknown value: keep default (silent)
	}
	if debugFlag {
		logLevel = view.LogLevelDebug
	}

	// Create a new CLI instance, which is a global context that each command
	// can use to access, useful for view rendering, etc.
	cli := NewCLI(viewType, os.Stdout, logLevel)

	// Add all subcommands to the root command
	AddCommands(rootCmd, cli)

	// Walk and execute the resolved command with flags.
	if err := rootCmd.Execute(); err != nil {
		cli.Println(err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

// AddCommands registers all subcommands to the root command.
func AddCommands(root *cobra.Command, cli *CLI) {
	root.AddCommand(
		newVersionCommand(cli),
		NewPushCommand(cli),
		NewInspectCommand(cli),
	)
}
