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

## Features

- Respects .gitignore by default
- Customizable ignore patterns
- Line numbering support

## Smart filtering

By default, files-combine uses [go-enry](https://github.com/go-enry/go-enry)
to automatically exclude files that add noise rather than useful context:

- **Excluded**: binary files, generated files (e.g. `*.pb.go`, minified JS),
  vendored dependencies, and configuration files (e.g. `.env`, `*.lock`)
- **Always included**: source code, documentation files, and known doc filenames
  (`README`, `CHANGELOG`, `CONTRIBUTING`, `LICENSE` and their `.md`/`.txt` variants)

Use `--all` to disable smart filtering and include every file (still respects
`.gitignore` and `--ignore` patterns).

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

| Flag               | Short | Default                     | Description                                          |
| ------------------ | ----- | --------------------------- | ---------------------------------------------------- |
| `--ext`            |       |                             | File extensions to include (comma-separated)         |
| `--include-hidden` |       | `false`                     | Include files starting with `.`                      |
| `--no-gitignore`   |       | `false`                     | Ignore .gitignore rules                              |
| `--ignore`         |       |                             | Additional patterns to ignore                        |
| `--all`            |       | `false`                     | Disable smart filtering, include all file types      |
| `--output`         | `-o`  | `output.md` or `output.xml` | Output file path                                     |
| `--format`         | `-f`  | `markdown`                  | Output format (`markdown` or `xml`)                  |
| `--line-numbers`   | `-n`  | `false`                     | Add line numbers to output                           |
| `--dry-run`        |       | `false`                     | Print files that would be combined                   |

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

## License

MIT
