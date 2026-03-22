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

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "AI Long-Term Memory operations",
	Long:  "Store, search, and manage AI agent memories and preferences",
}

var memoryStoreCmd = &cobra.Command{
	Use:   "store",
	Short: "Store a new memory",
	Run: func(cmd *cobra.Command, args []string) {
		dbID, _ := cmd.Flags().GetString("db")
		content, _ := cmd.Flags().GetString("content")
		metadataStr, _ := cmd.Flags().GetString("metadata")

		if dbID == "" || content == "" {
			fmt.Fprintln(os.Stderr, "Error: --db and --content are required")
			os.Exit(1)
		}

		var metadata map[string]interface{}
		if metadataStr != "" {
			if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
				fmt.Fprintln(os.Stderr, "Error parsing metadata JSON:", err)
				os.Exit(1)
			}
		} else {
			metadata = make(map[string]interface{})
		}

		payload := map[string]interface{}{
			"database_id": dbID,
			"table_name":  "knowledge_base",
			"content":     content,
			"metadata":    metadata,
		}

		body, _ := json.Marshal(payload)
		resp, err := http.Post(apiURL+"/api/v1/embeddings/insert", "application/json", bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			fmt.Fprintln(os.Stderr, "API Error:", string(data))
			os.Exit(1)
		}
		fmt.Println("Memory stored successfully:", string(data))
	},
}

var memorySearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search memories by natural language query",
	Run: func(cmd *cobra.Command, args []string) {
		dbID, _ := cmd.Flags().GetString("db")
		query, _ := cmd.Flags().GetString("query")
		limit, _ := cmd.Flags().GetInt("limit")

		if dbID == "" || query == "" {
			fmt.Fprintln(os.Stderr, "Error: --db and --query are required")
			os.Exit(1)
		}

		payload := map[string]interface{}{
			"database_id": dbID,
			"table_name":  "knowledge_base",
			"query":       query,
			"limit":       limit,
		}

		body, _ := json.Marshal(payload)
		resp, err := http.Post(apiURL+"/api/v1/embeddings/search", "application/json", bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			fmt.Fprintln(os.Stderr, "API Error:", string(data))
			os.Exit(1)
		}
		fmt.Println(string(data))
	},
}

var memorySetPrefCmd = &cobra.Command{
	Use:   "set-pref",
	Short: "Set a user preference",
	Run: func(cmd *cobra.Command, args []string) {
		dbID, _ := cmd.Flags().GetString("db")
		userID, _ := cmd.Flags().GetString("user")
		key, _ := cmd.Flags().GetString("key")
		valStr, _ := cmd.Flags().GetString("value")

		if dbID == "" || userID == "" || key == "" || valStr == "" {
			fmt.Fprintln(os.Stderr, "Error: --db, --user, --key, and --value are required")
			os.Exit(1)
		}

		var val interface{}
		if err := json.Unmarshal([]byte(valStr), &val); err != nil {
			// If not valid JSON, treat as string
			valStr = fmt.Sprintf(`"%s"`, valStr)
		}

		sqlQuery := `
			INSERT INTO user_preferences (user_id, key, value)
			VALUES ($1, $2, $3::jsonb)
			ON CONFLICT (user_id, key) DO UPDATE 
			SET value = EXCLUDED.value, updated_at = NOW();
		`

		payload := map[string]interface{}{
			"sql":    sqlQuery,
			"params": []interface{}{userID, key, valStr},
		}

		body, _ := json.Marshal(payload)
		resp, err := http.Post(apiURL+"/api/v1/databases/"+dbID+"/sql", "application/json", bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			fmt.Fprintln(os.Stderr, "API Error:", string(data))
			os.Exit(1)
		}
		fmt.Println("Preference set successfully!")
	},
}

var memoryGetPrefCmd = &cobra.Command{
	Use:   "get-pref",
	Short: "Get user preference(s)",
	Run: func(cmd *cobra.Command, args []string) {
		dbID, _ := cmd.Flags().GetString("db")
		userID, _ := cmd.Flags().GetString("user")
		key, _ := cmd.Flags().GetString("key")

		if dbID == "" || userID == "" {
			fmt.Fprintln(os.Stderr, "Error: --db and --user are required")
			os.Exit(1)
		}

		sqlQuery := `SELECT key, value, updated_at FROM user_preferences WHERE user_id = $1`
		params := []interface{}{userID}

		if key != "" {
			sqlQuery += ` AND key = $2`
			params = append(params, key)
		}

		payload := map[string]interface{}{
			"sql":    sqlQuery,
			"params": params,
		}

		body, _ := json.Marshal(payload)
		resp, err := http.Post(apiURL+"/api/v1/databases/"+dbID+"/sql", "application/json", bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			fmt.Fprintln(os.Stderr, "API Error:", string(data))
			os.Exit(1)
		}
		fmt.Println(string(data))
	},
}

func init() {
	rootCmd.AddCommand(memoryCmd)
	memoryCmd.AddCommand(memoryStoreCmd)
	memoryCmd.AddCommand(memorySearchCmd)
	memoryCmd.AddCommand(memorySetPrefCmd)
	memoryCmd.AddCommand(memoryGetPrefCmd)

	memoryStoreCmd.Flags().String("db", "", "Database ID (required)")
	memoryStoreCmd.Flags().String("content", "", "Memory content (required)")
	memoryStoreCmd.Flags().String("metadata", "", "Optional JSON metadata")

	memorySearchCmd.Flags().String("db", "", "Database ID (required)")
	memorySearchCmd.Flags().String("query", "", "Search query (required)")
	memorySearchCmd.Flags().Int("limit", 5, "Number of results to return")

	memorySetPrefCmd.Flags().String("db", "", "Database ID (required)")
	memorySetPrefCmd.Flags().String("user", "", "User ID (required)")
	memorySetPrefCmd.Flags().String("key", "", "Preference key (required)")
	memorySetPrefCmd.Flags().String("value", "", "Preference value in JSON format (required)")

	memoryGetPrefCmd.Flags().String("db", "", "Database ID (required)")
	memoryGetPrefCmd.Flags().String("user", "", "User ID (required)")
	memoryGetPrefCmd.Flags().String("key", "", "Optional preference key to filter")
}