package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"oriowambert.com/ask-golang/internal/config"
	"oriowambert.com/ask-golang/internal/render"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or change saved defaults",
	Long: `Keys: model, format, system_prompt, budget, history and persona.<name>.

Flags given on the command line always win over saved defaults.`,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the location of the config file",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.Path()
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Print all saved defaults",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Print one saved default",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		value, ok, err := cfg.Get(args[0])
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s is not set", args[0])
		}
		fmt.Println(value)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Save a default",
	Example: `  ask config set model haiku
  ask config set format markdown
  ask config set persona.reviewer "You review code and only list real bugs."`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] == "format" {
			if _, err := render.New(args[1], render.Options{}); err != nil {
				return err
			}
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := cfg.Set(args[0], args[1]); err != nil {
			return err
		}
		return cfg.Save()
	},
}

var configUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Remove a saved default",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := cfg.Unset(args[0]); err != nil {
			return err
		}
		return cfg.Save()
	},
}

func init() {
	configCmd.AddCommand(configPathCmd, configListCmd, configGetCmd, configSetCmd, configUnsetCmd)
	rootCmd.AddCommand(configCmd)
}
