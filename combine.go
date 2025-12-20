package filescombine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CombineOpts struct {
	OutputFile  string
	Format      string // "markdown" or "xml"
	LineNumbers bool
}

var extToLang = map[string]string{
	".py":   "python",
	".c":    "c",
	".cpp":  "cpp",
	".go":   "go",
	".java": "java",
	".js":   "javascript",
	".jsx":  "jsx",
	".ts":   "typescript",
	".tsx":  "tsx",
	".rs":   "rust",
	".html": "html",
	".css":  "css",
	".xml":  "xml",
	".json": "json",
	".yaml": "yaml",
	".yml":  "yaml",
	".sh":   "bash",
	".rb":   "ruby",
}

func Combine(results []FileResult, opts CombineOpts) error {
	file, err := os.Create(opts.OutputFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer file.Close()

	if opts.Format == "xml" {
		return combineAsXML(file, results, opts.LineNumbers)
	}
	return combineAsMarkdown(file, results, opts.LineNumbers)
}

func combineAsMarkdown(file *os.File, results []FileResult, lineNumbers bool) error {
	for _, result := range results {
		if result.Err != nil {
			continue // skip files with errors
		}

		content := result.Content
		if lineNumbers {
			content = addLineNumbers(content)
		}

		// get language from extension
		ext := filepath.Ext(result.RelPath)
		lang := extToLang[ext]

		// determine backtick count (handle content with backticks)
		backticks := "```"
		for strings.Contains(content, backticks) {
			backticks += "`"
		}

		// Write markdown format
		fmt.Fprintf(file, "%s\n", result.RelPath)
		fmt.Fprintf(file, "%s%s\n", backticks, lang)
		fmt.Fprintf(file, "%s\n", content)
		fmt.Fprintf(file, "%s\n", backticks)
	}

	return nil
}

func combineAsXML(file *os.File, results []FileResult, lineNumbers bool) error {
	fmt.Fprintln(file, "<documents>")

	index := 1
	for _, result := range results {
		if result.Err != nil {
			continue
		}

		content := result.Content
		if lineNumbers {
			content = addLineNumbers(content)
		}

		fmt.Fprintf(file, "<document index=\"%d\">\n", index)
		fmt.Fprintf(file, "<source>%s</source>\n", result.RelPath)
		fmt.Fprintln(file, "<document_content>")
		fmt.Fprintln(file, content)
		fmt.Fprintln(file, "</document_content>")
		fmt.Fprintln(file, "</document>")

		index++
	}

	fmt.Fprintln(file, "</documents>")
	return nil
}

func addLineNumbers(content string) string {
	lines := strings.Split(content, "\n")

	// calculate padding based on total line count
	padding := len(fmt.Sprintf("%d", len(lines)))

	var numbered []string
	for i, line := range lines {
		numbered = append(numbered, fmt.Sprintf("%*d  %s", padding, i+1, line))
	}

	return strings.Join(numbered, "\n")
}
