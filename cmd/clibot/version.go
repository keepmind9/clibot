package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// Build information variables (set by Makefile during build)
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitBranch = "unknown"
	GitCommit = "unknown"
)

var versionJSON bool

// VersionOutput represents the version output structure
type VersionOutput struct {
	Version    string `json:"version"`
	BuildTime  string `json:"build_time"`
	GitBranch  string `json:"git_branch"`
	GitCommit  string `json:"git_commit"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  "Display version number, build time, git branch, and commit ID",
	Run: func(cmd *cobra.Command, args []string) {
		version := VersionOutput{
			Version:    Version,
			BuildTime:  BuildTime,
			GitBranch:  GitBranch,
			GitCommit:  GitCommit,
		}

		if versionJSON {
			output, err := json.MarshalIndent(version, "", "  ")
			if err != nil {
				fmt.Printf("{\"error\": \"failed to marshal json: %v\"}\n", err)
				return
			}
			fmt.Println(string(output))
		} else {
			fmt.Println("clibot version information:")
			fmt.Printf("  Version:    %s\n", version.Version)
			fmt.Printf("  BuildTime: %s\n", version.BuildTime)
			fmt.Printf("  GitBranch: %s\n", version.GitBranch)
			fmt.Printf("  GitCommit: %s\n", version.GitCommit)
		}
	},
}

func init() {
	versionCmd.Flags().BoolVar(&versionJSON, "json", false, "Output in JSON format")
}
