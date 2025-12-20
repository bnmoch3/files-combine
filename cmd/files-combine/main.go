package main

import (
	"fmt"
	"log"
	"os"

	filescombine "github.com/bnmoch3/files-combine"
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
		// validate format flag
		if format != "xml" && format != "markdown" {
			log.Fatalf("Invalid format: %s. Must be 'xml' or 'markdown'", format)
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

		// set default output file if not provided
		if outputFile == "" {
			if format == "markdown" {
				outputFile = "output.md"
			} else {
				outputFile = "output.xml"
			}
		}

		log.Printf("Processing path: %s", path)
		log.Printf("Extensions: %v", extensions)
		log.Printf("Output file: %s", outputFile)
		log.Printf("Format: %s", format)
		log.Printf("Dry run: %v", dryRun)

		// build gather opts
		gatherOpts := filescombine.GatherOptions{
			Extensions:      extensions,
			IncludeHidden:   includeHidden,
			IgnoreGitignore: ignoreGitignore,
			IgnorePatterns:  ignorePatterns,
			IgnoreFilesOnly: ignoreFilesOnly,
		}

		// gather files
		results, err := filescombine.Gather(path, gatherOpts)
		if err != nil {
			log.Fatalf("Error gathering files: %v", err)
		}

		// handle dry run
		if dryRun {
			for _, result := range results {
				if result.Err != nil {
					log.Printf("Error reading %s: %v", result.RelPath, result.Err)
					continue
				}
				fmt.Println(result.RelPath)
			}
			return
		}

		// combine files
		combineOpts := filescombine.CombineOpts{
			OutputFile:  outputFile,
			Format:      format,
			LineNumbers: lineNumbers,
		}

		if err := filescombine.Combine(results, combineOpts); err != nil {
			log.Fatalf("Error combining files: %v", err)
		}

		log.Printf("Successfully combined files to %s", outputFile)
	},
}

const version = "1.0.0"

func init() {
	rootCmd.Version = version
	rootCmd.Flags().StringSliceVar(&extensions, "ext", []string{}, "File extensions to include")
	rootCmd.Flags().BoolVar(&includeHidden, "include-hidden", true, "Include files starting with . (default: true)")
	rootCmd.Flags().BoolVar(&ignoreFilesOnly, "ignore-files-only", false, "--ignore only applies to files")
	rootCmd.Flags().BoolVar(&ignoreGitignore, "ignore-gitignore", false, "Ignore .gitignore files")
	rootCmd.Flags().StringSliceVar(&ignorePatterns, "ignore", []string{}, "Patterns to ignore")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: output.md or output.xml based on format)")
	rootCmd.Flags().StringVarP(&format, "format", "f", "markdown", "Output format: 'xml' or 'markdown' (default: markdown)")
	rootCmd.Flags().BoolVarP(&lineNumbers, "line-numbers", "n", false, "Add line numbers")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print files that will be combined without processing")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
