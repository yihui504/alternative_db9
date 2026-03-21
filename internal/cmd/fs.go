package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var fsCmd = &cobra.Command{
	Use:   "fs",
	Short: "File system operations",
	Long:  "Upload, list, download, and delete files in database storage",
}

var fsCpCmd = &cobra.Command{
	Use:   "cp <source> <destination>",
	Short: "Copy files between local and database storage",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		source := args[0]
		dest := args[1]
		dbID, _ := cmd.Flags().GetString("db")

		if dbID == "" {
			fmt.Fprintln(os.Stderr, "Error: --db is required")
			os.Exit(1)
		}

		if _, err := os.Stat(source); err == nil {
			uploadFile(source, dest, dbID)
		} else {
			fmt.Fprintln(os.Stderr, "Error: source file not found")
			os.Exit(1)
		}
	},
}

func uploadFile(localPath, dbPath, dbID string) {
	file, err := os.Open(localPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error opening file:", err)
		os.Exit(1)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating form file:", err)
		os.Exit(1)
	}
	io.Copy(part, file)

	writer.WriteField("database_id", dbID)
	writer.WriteField("path", dbPath)

	writer.Close()

	req, _ := http.NewRequest("POST", apiURL+"/api/v1/files/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	fmt.Println(string(data))
}

var fsLsCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "List files in database storage",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dbID, _ := cmd.Flags().GetString("db")

		if dbID == "" {
			fmt.Fprintln(os.Stderr, "Error: --db is required")
			os.Exit(1)
		}

		resp, err := http.Get(apiURL + "/api/v1/files?database_id=" + dbID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		
		var files []map[string]interface{}
		json.Unmarshal(data, &files)
		
		for _, f := range files {
			fmt.Printf("%s\t%v\t%s\n", f["id"], f["size"], f["path"])
		}
	},
}

var fsRmCmd = &cobra.Command{
	Use:   "rm <file-id>",
	Short: "Delete a file from database storage",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		
		req, _ := http.NewRequest("DELETE", apiURL+"/api/v1/files/"+id, nil)
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

var fsCatCmd = &cobra.Command{
	Use:   "cat <file-id>",
	Short: "Download and display a file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		
		resp, err := http.Get(apiURL + "/api/v1/files/" + id)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		io.Copy(os.Stdout, resp.Body)
	},
}

var fsQueryCmd = &cobra.Command{
	Use:   "query <path>",
	Short: "Query file content as table",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		dbID, _ := cmd.Flags().GetString("db")

		if dbID == "" {
			fmt.Fprintln(os.Stderr, "Error: --db is required")
			os.Exit(1)
		}

		resp, err := http.Get(apiURL + "/api/v1/files/query?database_id=" + dbID + "&path=" + url.QueryEscape(path))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)

		var result struct {
			Results []map[string]interface{} `json:"results"`
			Count   int                      `json:"count"`
		}
		json.Unmarshal(data, &result)

		// 表格输出
		if len(result.Results) > 0 {
			// 打印表头
			for key := range result.Results[0] {
				fmt.Printf("%s\t", key)
			}
			fmt.Println()

			// 打印分隔线
			for range result.Results[0] {
				fmt.Printf("%s\t", "----")
			}
			fmt.Println()

			// 打印数据
			for _, row := range result.Results {
				for _, val := range row {
					fmt.Printf("%v\t", val)
				}
				fmt.Println()
			}
		}

		fmt.Printf("\nTotal: %d rows\n", result.Count)
	},
}

func init() {
	rootCmd.AddCommand(fsCmd)
	fsCmd.AddCommand(fsCpCmd)
	fsCmd.AddCommand(fsLsCmd)
	fsCmd.AddCommand(fsRmCmd)
	fsCmd.AddCommand(fsCatCmd)
	fsCmd.AddCommand(fsQueryCmd)

	fsCpCmd.Flags().String("db", "", "Database ID")
	fsLsCmd.Flags().String("db", "", "Database ID")
	fsQueryCmd.Flags().String("db", "", "Database ID")
}
