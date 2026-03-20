package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var apiURL string

var rootCmd = &cobra.Command{
	Use:   "oc-db9",
	Short: "OpenClaw-db9 CLI - A self-hosted db9.ai alternative",
	Long: `OpenClaw-db9 (oc-db9) is a fully self-hosted db9.ai alternative
that provides complete database infrastructure for AI agents.

Features:
  - Instant database creation
  - Built-in vector search with pgvector
  - File storage with MinIO
  - Database branching
  - Scheduled jobs with pg_cron
  - Type generation for TypeScript, Python, and Go`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.oc-db9.yaml)")
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "http://localhost:8080", "API server URL")
	
	viper.BindPFlag("api-url", rootCmd.PersistentFlags().Lookup("api-url"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".oc-db9")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
