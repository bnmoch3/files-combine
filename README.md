# files-combine

A command-line tool to combine multiple files into a single document formatted
for use LLMs like Claude. Inspired by simonw's
[files-to-prompt](https://github.com/simonw/files-to-prompt)

## Installation

```bash
go install github.com/bnmoch3/files-combine/cmd/files-combine@latest
```

Or build from source:

```bash
git clone https://github.com/bnmoch3/files-combine
cd files-combine
go build -o files-combine cmd/main.go
```

## Usage

```
files-combine [path] [flags]
```

If no `[path]` is provided then the current working directory is used.

## Examples

Basic usage (combines all files in current dir to output.md):

```bash
files-combine
```

Filter by extension and add line numbers:

```bash
files-combine --ext go,js --line-numbers
```

Output to XML:

```
files-combine --format xml --output context.xml
```

Preview files to be processed without writing:

```bash
files-combine --dry-run
```

Ignore specific patterns:

```bash
files-combine --ignore "vendor" --ignore "*_test.go"
```

## Flags

| Flag                  | Short | Default                     | Description                                   |
| --------------------- | ----- | --------------------------- | --------------------------------------------- |
| `--ext`               |       |                             | File extensions to include (comma-separated)  |
| `--include-hidden`    |       | `true`                      | Include files starting with `.`               |
| `--ignore-gitignore`  |       | `false`                     | Ignore .gitignore rules                       |
| `--ignore`            |       |                             | Additional patterns to ignore                 |
| `--ignore-files-only` |       | `false`                     | Apply --ignore only to files, not directories |
| `--output`            | `-o`  | `output.md` or `output.xml` | Output file path                              |
| `--format`            | `-f`  | `markdown`                  | Output format (`markdown` or `xml`)           |
| `--line-numbers`      | `-n`  | `false`                     | Add line numbers to output                    |
| `--dry-run`           |       | `false`                     | Print files that would be combined            |

## Output Formats

**Markdown (Default)** Wraps file content in code blocks with language syntax
highlighting based on file extension. Handles nested backticks automatically.

**XML** Wraps content in a structure designed for parsing:

```xml
<documents>
  <document index="1">
    <source>path/to/file.go</source>
    <document_content>
      ... code ...
    </document_content>
  </document>
</documents>
```

## Features

- Concurrent file processing for performance
- Respects .gitignore by default
- Supports multiple file extensions
- Syntax highlighting in Markdown output
- Customizable ignore patterns
- Line numbering support

## License

MIT
