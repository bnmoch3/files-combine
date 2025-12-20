package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

var (
	extensions      []string
	includeHidden   bool
	ignoreFilesOnly bool
	ignoreGitignore bool
	ignorePatterns  []string
	outputFile      string
	format          string
	lineNumbers     bool
	dryRun          bool
)

var rootCmd = &cobra.Command{
	Use:   "files-combine [path]",
	Short: "Combine files into a prompt for LLMs",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Validate format flag
		if format != "cxml" && format != "markdown" {
			log.Fatalf("Invalid format: %s. Must be 'cxml' or 'markdown'", format)
		}

		var path string
		if len(args) == 0 {
			cwd, err := os.Getwd()
			if err != nil {
				log.Fatal(err)
			}
			path = cwd
		} else {
			path = args[0]
		}

		// Set default output file if not provided
		if outputFile == "" {
			outputFile = "combined.txt"
		}

		log.Printf("Processing path: %s", path)
		log.Printf("Extensions: %v", extensions)
		log.Printf("Output file: %s", outputFile)
		log.Printf("Format: %s", format)
		log.Printf("Dry run: %v", dryRun)
		// Call your processing logic here
	},
}

func init() {
	rootCmd.Flags().StringSliceVar(&extensions, "ext", []string{}, "File extensions to include")
	rootCmd.Flags().BoolVar(&includeHidden, "include-hidden", true, "Include files starting with . (default: true)")
	rootCmd.Flags().BoolVar(&ignoreFilesOnly, "ignore-files-only", false, "--ignore only applies to files")
	rootCmd.Flags().BoolVar(&ignoreGitignore, "ignore-gitignore", false, "Ignore .gitignore files")
	rootCmd.Flags().StringSliceVar(&ignorePatterns, "ignore", []string{}, "Patterns to ignore")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: combined.txt)")
	rootCmd.Flags().StringVarP(&format, "format", "f", "markdown", "Output format: 'cxml' or 'markdown' (default: markdown)")
	rootCmd.Flags().BoolVarP(&lineNumbers, "line-numbers", "n", false, "Add line numbers")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print files that will be combined without processing")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
