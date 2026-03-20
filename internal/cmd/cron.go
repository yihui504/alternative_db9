package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Cron job operations",
	Long:  "Create, list, and delete scheduled jobs",
}

var cronCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new cron job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		dbID, _ := cmd.Flags().GetString("db")
		schedule, _ := cmd.Flags().GetString("schedule")
		sqlCmd, _ := cmd.Flags().GetString("command")

		if dbID == "" || schedule == "" || sqlCmd == "" {
			fmt.Fprintln(os.Stderr, "Error: --db, --schedule, and --command are required")
			os.Exit(1)
		}

		payload := map[string]string{
			"database_id": dbID,
			"name":        name,
			"schedule":    schedule,
			"sql_command": sqlCmd,
		}

		body, _ := json.Marshal(payload)
		resp, err := http.Post(apiURL+"/api/v1/cron", "application/json", bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		fmt.Println(string(data))
	},
}

var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cron jobs",
	Run: func(cmd *cobra.Command, args []string) {
		dbID, _ := cmd.Flags().GetString("db")

		if dbID == "" {
			fmt.Fprintln(os.Stderr, "Error: --db is required")
			os.Exit(1)
		}

		resp, err := http.Get(apiURL + "/api/v1/cron?database_id=" + dbID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		
		var jobs []map[string]interface{}
		json.Unmarshal(data, &jobs)
		
		for _, j := range jobs {
			active := "inactive"
			if j["is_active"].(bool) {
				active = "active"
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", j["id"], j["name"], j["schedule"], active)
		}
	},
}

var cronDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a cron job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		
		req, _ := http.NewRequest("DELETE", apiURL+"/api/v1/cron/"+id, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		fmt.Println(string(data))
	},
}

func init() {
	rootCmd.AddCommand(cronCmd)
	cronCmd.AddCommand(cronCreateCmd)
	cronCmd.AddCommand(cronListCmd)
	cronCmd.AddCommand(cronDeleteCmd)

	cronCreateCmd.Flags().String("db", "", "Database ID")
	cronCreateCmd.Flags().String("schedule", "", "Cron schedule expression (e.g., '*/5 * * * *')")
	cronCreateCmd.Flags().String("command", "", "SQL command to execute")
	cronListCmd.Flags().String("db", "", "Database ID")
}
