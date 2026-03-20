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

var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Database branch operations",
	Long:  "Create, list, and delete database branches",
}

var branchCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new branch",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		dbID, _ := cmd.Flags().GetString("db")
		from, _ := cmd.Flags().GetString("from")

		if dbID == "" {
			fmt.Fprintln(os.Stderr, "Error: --db is required")
			os.Exit(1)
		}

		payload := map[string]string{
			"database_id": dbID,
			"name":        name,
		}
		if from != "" {
			payload["source_branch"] = from
		}

		body, _ := json.Marshal(payload)
		resp, err := http.Post(apiURL+"/api/v1/branches", "application/json", bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		fmt.Println(string(data))
	},
}

var branchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all branches",
	Run: func(cmd *cobra.Command, args []string) {
		dbID, _ := cmd.Flags().GetString("db")

		if dbID == "" {
			fmt.Fprintln(os.Stderr, "Error: --db is required")
			os.Exit(1)
		}

		resp, err := http.Get(apiURL + "/api/v1/branches?database_id=" + dbID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		
		var branches []map[string]interface{}
		json.Unmarshal(data, &branches)
		
		for _, b := range branches {
			fmt.Printf("%s\t%s\t%s\t%s\n", b["id"], b["name"], b["source_branch"], b["created_at"])
		}
	},
}

var branchDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a branch",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		
		req, _ := http.NewRequest("DELETE", apiURL+"/api/v1/branches/"+id, nil)
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
	rootCmd.AddCommand(branchCmd)
	branchCmd.AddCommand(branchCreateCmd)
	branchCmd.AddCommand(branchListCmd)
	branchCmd.AddCommand(branchDeleteCmd)

	branchCreateCmd.Flags().String("db", "", "Database ID")
	branchCreateCmd.Flags().String("from", "", "Source branch name")
	branchListCmd.Flags().String("db", "", "Database ID")
}
