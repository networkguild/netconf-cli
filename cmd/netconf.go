package cmd

import (
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	copyconfig "github.devcloud.elisa.fi/netops/netconf-go/cmd/copy-config"
	"github.devcloud.elisa.fi/netops/netconf-go/cmd/dispatch"
	editconfig "github.devcloud.elisa.fi/netops/netconf-go/cmd/edit-config"
	"github.devcloud.elisa.fi/netops/netconf-go/cmd/get"
	getconfig "github.devcloud.elisa.fi/netops/netconf-go/cmd/get-config"
	"github.devcloud.elisa.fi/netops/netconf-go/cmd/notification"
)

var (
	debug   bool
	caller  bool
	file    string
	rootCmd = &cobra.Command{
		Use:   "netconf",
		Short: "Cli tool for running netconf operations",
		Long: `Cli tool for running netconf operations on network devices.

Supported netconf operations are get-config, get, edit-config, copy-config, notifications and custom dispatch.

All commands support parallel run with multiple devices, --inventory, -i file is needed with multiple hosts.

Global flags can be configured via environment variables (prefix NETCONF) or via command-line flags
`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if cmd.Name() != "help" {
				if caller {
					log.SetReportCaller(caller)
				}

				if file != "" {
					f, err := os.Create(file)
					if err != nil {
						log.Warnf("Failed to open logging file: %v. Using default logger", err)
					} else {
						log.SetOutput(f)
					}
				}

				if debug {
					log.SetLevel(log.DebugLevel)
					log.Debug("Debug logging enabled")
				}
			}
		},
	}
	completionCmd = &cobra.Command{
		Use:   "completion [bash|posh|zsh|fish]",
		Short: "Generate completion script",
		Long: `To load completions:

Bash (Linux):

$ source <(netconf completion bash)

# To load completions for each session, execute once:
$ netconf-cli completion bash > /usr/local/etc/bash_completion.d/netconf

PowerShell (Windows):

$ Register-ArgumentCompleter -Native -CommandName netconf-cli -ScriptBlock | netconf-cli completion posh

Zsh (MacOS):

$ source <(netconf completion zsh)

# To load completions for each session, execute once:
$ netconf completion zsh > "${fpath[1]}/netconf"

Fish:

$ netconf completion fish | source

# To load completions for each session, execute once:
$ netconf completion fish > ~/.config/fish/completions/netconf.fish
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "posh", "zsh", "fish"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1)),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				_ = cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				_ = cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				_ = cmd.Root().GenFishCompletion(os.Stdout, true)
			case "posh":
				_ = cmd.Root().GenPowerShellCompletion(os.Stdout)
			}
		},
	}
)

func init() {
	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportTimestamp: true,
		TimeFormat:      time.TimeOnly,
		Prefix:          "netconf",
	})
	log.SetDefault(logger)
	rootCmd.AddCommand(
		completionCmd,
		get.NewGetCommand(),
		getconfig.NewGetConfigCommand(),
		editconfig.NewEditConfigCommand(),
		notification.NewNotificationCommand(),
		dispatch.NewDispatchCommand(),
		copyconfig.NewCopyConfigCommand(),
	)
	persistentFlags := rootCmd.PersistentFlags()
	persistentFlags.StringP("username", "u", "admin", "SSH username or env NETCONF_USERNAME")
	persistentFlags.StringP("password", "p", "admin", "SSH password or env NETCONF_PASSWORD")
	persistentFlags.IntP("port", "P", 830, "Netconf port or env NETCONF_PORT")
	persistentFlags.BoolVar(&debug, "debug", false, "Enables debug level logging, logs raw replies")
	persistentFlags.BoolVar(&caller, "caller", false, "Enables logging to show caller func")
	persistentFlags.StringVar(&file, "logfile", "", "Enables logging to specific file")
	persistentFlags.StringP("inventory", "i", "", "Inventory file containing IP's")
	persistentFlags.String("host", "", "IP or hostname of device to connect")
	rootCmd.MarkFlagsMutuallyExclusive("inventory", "host")
	if err := viper.BindPFlags(persistentFlags); err != nil {
		log.Fatalf("Failed to bind cobra persistentFlags to viper, error: %v", err)
	}
	viper.SetEnvPrefix("NETCONF")
	viper.AutomaticEnv()
}

func Execute() error {
	return rootCmd.Execute()
}
