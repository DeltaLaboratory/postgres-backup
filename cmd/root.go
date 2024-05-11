package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/DeltaLaboratory/postgres-backup/internal/config"
)

var configFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "postgres-backup",
	Short: "Backup PostgreSQL databases to somewhere",
	Long: `Backup PostgreSQL databases to somewhere.

postgresl-backup is a tool to backup PostgreSQL databases to somewhere. It can backup to local filesystem, S3.

example:
	"postgres-backup backup" to backup a PostgreSQL database now
	"postgres-backup schedule run" to run the backup schedule defined in the configuration file
`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(func() {
		if configFile != "" {
			if err := config.LoadConfig(configFile); err != nil {
				panic(err)
			}
			return
		}

		if err := config.LoadConfig(""); err != nil {
			panic(err)
		}
	})

	RootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file (default is /etc/postgres_backup/config.hcl)")
}
