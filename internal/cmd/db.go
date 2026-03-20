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

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database operations",
	Long:  "Create, list, delete, and query databases",
}

var dbCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new database",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		from, _ := cmd.Flags().GetString("from")
		seed, _ := cmd.Flags().GetString("seed")

		payload := map[string]string{"name": name}
		if from != "" {
			payload["from"] = from
		}
		if seed != "" {
			payload["seed"] = seed
		}

		body, _ := json.Marshal(payload)
		resp, err := http.Post(apiURL+"/api/v1/databases", "application/json", bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		fmt.Println(string(data))
	},
}

var dbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all databases",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := http.Get(apiURL + "/api/v1/databases")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		
		var databases []map[string]interface{}
		json.Unmarshal(data, &databases)
		
		for _, db := range databases {
			fmt.Printf("%s\t%s\t%s\n", db["id"], db["name"], db["created_at"])
		}
	},
}

var dbDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a database",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		
		req, _ := http.NewRequest("DELETE", apiURL+"/api/v1/databases/"+id, nil)
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

var dbSqlCmd = &cobra.Command{
	Use:   "sql <id>",
	Short: "Execute SQL on a database",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		sqlQuery, _ := cmd.Flags().GetString("command")
		file, _ := cmd.Flags().GetString("file")

		if file != "" {
			data, err := os.ReadFile(file)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error reading file:", err)
				os.Exit(1)
			}
			sqlQuery = string(data)
		}

		if sqlQuery == "" {
			fmt.Fprintln(os.Stderr, "Error: --command or --file is required")
			os.Exit(1)
		}

		payload := map[string]string{"sql": sqlQuery}
		body, _ := json.Marshal(payload)
		
		resp, err := http.Post(apiURL+"/api/v1/databases/"+id+"/sql", "application/json", bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		
		var result map[string]interface{}
		json.Unmarshal(data, &result)
		
		if results, ok := result["results"].([]interface{}); ok {
			for _, row := range results {
				fmt.Printf("%v\n", row)
			}
		} else {
			fmt.Println(string(data))
		}
	},
}

var dbConnectCmd = &cobra.Command{
	Use:   "connect <id>",
	Short: "Get connection info for a database",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		
		resp, err := http.Get(apiURL + "/api/v1/databases/" + id + "/connect")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		
		var info map[string]interface{}
		json.Unmarshal(data, &info)
		
		fmt.Printf("Host: %s\n", info["host"])
		fmt.Printf("Port: %v\n", info["port"])
		fmt.Printf("Database: %s\n", info["database"])
		fmt.Printf("User: %s\n", info["user"])
		fmt.Printf("Connection String: %s\n", info["connection_string"])
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(dbCreateCmd)
	dbCmd.AddCommand(dbListCmd)
	dbCmd.AddCommand(dbDeleteCmd)
	dbCmd.AddCommand(dbSqlCmd)
	dbCmd.AddCommand(dbConnectCmd)

	dbCreateCmd.Flags().String("from", "", "Source database to clone from")
	dbCreateCmd.Flags().String("seed", "", "SQL seed file to run on creation")
	
	dbSqlCmd.Flags().StringP("command", "c", "", "SQL command to execute")
	dbSqlCmd.Flags().StringP("file", "f", "", "SQL file to execute")
}
