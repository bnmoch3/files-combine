package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

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

func normalizeOutputFileAndFormat(outputFile, format string) (string, string, error) {
	// normalize format string to canonical values
	switch format {
	case "md", "markdown", "gemini", "chatgpt":
		format = "markdown"
	case "xml", "claude":
		format = "xml"
	case "":
		// empty format is ok, we'll derive it from filename if possible
	default:
		return "", "", fmt.Errorf("invalid format: %q (must be 'markdown' or 'xml')", format)
	}

	// case 1: no output file specified
	if outputFile == "" {
		if format == "" {
			return "", "", fmt.Errorf("must specify either output file or format")
		}
		// generate default filename based on format
		switch format {
		case "markdown":
			outputFile = "output.md"
		case "xml":
			outputFile = "output.xml"
		}
		return outputFile, format, nil
	}

	// case 2: output file specified, preserve filename exactly as provided
	ext := filepath.Ext(outputFile)

	// Case 2a: file has valid extension (md, xml)
	if ext == ".md" || ext == ".xml" {
		// extension determines the format
		derivedFormat := ""
		switch ext {
		case ".md":
			derivedFormat = "markdown"
		case ".xml":
			derivedFormat = "xml"
		}

		// Warn if user specified a conflicting format flag
		if format != "" && format != derivedFormat {
			log.Printf("Warning: format %q overridden by file extension %q", format, ext)
		}

		return outputFile, derivedFormat, nil
	}

	// case 2b: File has no extension or invalid extension
	// filename is preserved as-is; format must be provided via flag
	if format == "" {
		return "", "", fmt.Errorf("cannot determine format from filename %q and no format specified", outputFile)
	}

	// use the provided format flag, keep filename unchanged
	return outputFile, format, nil
}

var rootCmd = &cobra.Command{
	Use:   "files-combine [path]",
	Short: "Combine files into a prompt for LLMs",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
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

		outputFile, format, err := normalizeOutputFileAndFormat(outputFile, format)
		if err != nil {
			log.Fatal(err)
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

const version = "1.0.2"

func init() {
	rootCmd.Version = version
	rootCmd.Flags().StringSliceVar(&extensions, "ext", []string{}, "File extensions to include")
	rootCmd.Flags().BoolVar(&includeHidden, "include-hidden", true, "Include files starting with . (default: true)")
	rootCmd.Flags().BoolVar(&ignoreFilesOnly, "ignore-files-only", false, "--ignore only applies to files")
	rootCmd.Flags().BoolVar(&ignoreGitignore, "ignore-gitignore", false, "Ignore .gitignore files")
	rootCmd.Flags().StringSliceVar(&ignorePatterns, "ignore", []string{}, "Patterns to ignore")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: output.md or output.xml based on format)")
	rootCmd.Flags().StringVarP(&format, "format", "f", "xml", "Output format: 'xml' or 'markdown' (default: xml)")
	rootCmd.Flags().BoolVarP(&lineNumbers, "line-numbers", "n", false, "Add line numbers")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print files that will be combined without processing")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
