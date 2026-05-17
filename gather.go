package filescombine

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/go-enry/go-enry/v2"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// GatherOptions configuration for file gathering
type GatherOptions struct {
	Extensions     []string
	IncludeHidden  bool
	NoGitignore    bool
	IgnorePatterns []string
	AllFiles       bool
}

var docFilenames = map[string]bool{
	"README": true, "README.md": true, "README.txt": true,
	"CHANGELOG": true, "CHANGELOG.md": true,
	"CONTRIBUTING": true, "CONTRIBUTING.md": true,
	"LICENSE": true, "LICENSE.md": true,
}

// FileInput input for downstream processing
type FileInput struct {
	Path    string
	RelPath string
}

// FileResult output from processing
type FileResult struct {
	Path    string
	RelPath string
	Content string
	Err     error
}

func walkFiles(done <-chan struct{}, dirPath string, opts GatherOptions) (<-chan FileInput, <-chan error) {
	out := make(chan FileInput)
	errCh := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errCh)

		// load .gitignore patterns from project root down to dirPath
		root := FindProjectRoot(dirPath)
		var patterns []gitignore.Pattern
		var matcher gitignore.Matcher
		loadedGitignoreDirs := map[string]bool{}
		if !opts.NoGitignore {
			p, err := loadGitignorePatterns(dirPath)
			if err != nil {
				errCh <- fmt.Errorf("loading gitignore: %w", err)
				return
			}
			patterns = p
			if len(patterns) > 0 {
				matcher = gitignore.NewMatcher(patterns)
			}
			// mark dirs from root to dirPath as already loaded
			loadedGitignoreDirs[root] = true
			rel, _ := filepath.Rel(root, dirPath)
			rel = filepath.ToSlash(rel)
			if rel != "." {
				parts := strings.Split(rel, "/")
				for i := range parts {
					subPath := filepath.Join(root, filepath.Join(parts[:i+1]...))
					loadedGitignoreDirs[subPath] = true
				}
			}
		}

		err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// skip hidden files/dirs if not included
			if !opts.IncludeHidden && strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// skip .git and other VCS directories
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == ".svn" || name == ".hg" {
					return filepath.SkipDir
				}
			}

			// get relative path from root
			relPath, err := filepath.Rel(dirPath, path)
			if err != nil {
				return err
			}

			// convert to forward slashes for gitignore matching
			relPath = filepath.ToSlash(relPath)

			// relative path from project root for correctly scoped gitignore matching
			relFromRoot, rfErr := filepath.Rel(root, path)
			if rfErr != nil {
				relFromRoot = relPath
			}
			relFromRoot = filepath.ToSlash(relFromRoot)

			// dynamically load .gitignore files as we descend into subdirectories
			if !opts.NoGitignore && d.IsDir() && !loadedGitignoreDirs[path] {
				domain := strings.Split(relFromRoot, "/")
				if p, err := parseGitignoreFile(filepath.Join(path, ".gitignore"), domain); err == nil && len(p) > 0 {
					patterns = append(patterns, p...)
					matcher = gitignore.NewMatcher(patterns)
				}
				loadedGitignoreDirs[path] = true
			}

			// check gitignore (skip root of walk)
			if matcher != nil && relPath != "." {
				if shouldIgnore(matcher, relFromRoot, d.IsDir()) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			// check custom ignore patterns
			if len(opts.IgnorePatterns) > 0 {
				if shouldIgnorePatterns(d.Name(), opts.IgnorePatterns) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			// smart filtering: skip vendor directories
			if !opts.AllFiles && d.IsDir() && enry.IsVendor(relPath) {
				return filepath.SkipDir
			}

			// skip directories and non-regular files
			if d.IsDir() {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}

			// filter by extensions if provided
			if len(opts.Extensions) > 0 {
				matched := false
				fileExt := filepath.Ext(d.Name()) // e.g., ".go", ".mod", ".sum"

				for _, ext := range opts.Extensions {
					// add dot if not present
					wantedExt := ext
					if !strings.HasPrefix(wantedExt, ".") {
						wantedExt = "." + wantedExt
					}

					if fileExt == wantedExt {
						matched = true
						break
					}
				}
				if !matched {
					return nil
				}
			}

			// smart filtering: skip vendor, binary, generated, configuration files
			if !opts.AllFiles {
				sample, err := readFileSample(path, 512)
				if err == nil {
					if enry.IsBinary(sample) {
						return nil
					}
					isDoc := enry.IsDocumentation(relPath) || docFilenames[d.Name()]
					if !isDoc {
						if enry.IsVendor(relPath) || enry.IsGenerated(relPath, sample) || enry.IsConfiguration(relPath) {
							return nil
						}
					}
				}
			}

			select {
			case out <- FileInput{Path: path, RelPath: relPath}:
			case <-done:
				return fmt.Errorf("walk cancelled")
			}

			return nil
		})
		if err != nil {
			errCh <- err
		}
	}()

	return out, errCh
}

func shouldIgnorePatterns(name string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, _ := filepath.Match(pattern, name)
		if matched {
			return true
		}
	}
	return false
}

var otherRootMarkers = []string{
	"go.mod", "package.json", "Cargo.toml", "pyproject.toml", "pom.xml", "Makefile",
}

func FindProjectRoot(startPath string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return startPath
	}

	// first pass: .git takes priority — a git root beats any other marker
	dir := startPath
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		if dir == home {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// second pass: no .git found anywhere; fall back to other markers
	dir = startPath
	for {
		for _, marker := range otherRootMarkers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		if dir == home {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return startPath
		}
		dir = parent
	}
}

func parseGitignoreFile(path string, domain []string) ([]gitignore.Pattern, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []gitignore.Pattern
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, domain))
	}
	return patterns, scanner.Err()
}

func loadGitignorePatterns(dirPath string) ([]gitignore.Pattern, error) {
	root := FindProjectRoot(dirPath)

	rel, err := filepath.Rel(root, dirPath)
	if err != nil {
		rel = "."
	}
	rel = filepath.ToSlash(rel)

	type dirEntry struct {
		path   string
		domain []string
	}

	dirs := []dirEntry{{root, nil}}
	if rel != "." {
		parts := strings.Split(rel, "/")
		for i := range parts {
			subPath := filepath.Join(root, filepath.Join(parts[:i+1]...))
			dirs = append(dirs, dirEntry{subPath, append([]string(nil), parts[:i+1]...)})
		}
	}

	var patterns []gitignore.Pattern
	for _, d := range dirs {
		p, err := parseGitignoreFile(filepath.Join(d.path, ".gitignore"), d.domain)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, p...)
	}
	return patterns, nil
}

func shouldIgnore(matcher gitignore.Matcher, path string, isDir bool) bool {
	if path == "." {
		return false
	}

	parts := strings.Split(path, "/")
	for i := range parts {
		partialPath := strings.Join(parts[:i+1], "/")
		if matcher.Match(strings.Split(partialPath, "/"), isDir && i == len(parts)-1) {
			return true
		}
	}

	return false
}

func merge(done <-chan struct{}, channels ...<-chan FileResult) <-chan FileResult {
	out := make(chan FileResult)
	var wg sync.WaitGroup
	wg.Add(len(channels))

	output := func(ch <-chan FileResult) {
		defer wg.Done()
		for result := range ch {
			select {
			case out <- result:
			case <-done:
				return
			}
		}
	}

	for _, ch := range channels {
		go output(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func readFileSample(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	nr, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}
	return buf[:nr], nil
}

func readFileContent(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func processFile(done <-chan struct{}, in <-chan FileInput) <-chan FileResult {
	out := make(chan FileResult)

	go func() {
		defer close(out)

		for input := range in {
			content, err := readFileContent(input.Path)

			result := FileResult{
				Path:    input.Path,
				RelPath: input.RelPath,
				Content: content,
				Err:     err,
			}

			select {
			case out <- result:
			case <-done:
				return
			}
		}
	}()

	return out
}

func Gather(dirPath string, opts GatherOptions) ([]FileResult, error) {
	done := make(chan struct{})
	defer close(done)

	// stage 1: walk dirPath and generate file inputs
	filesCh, walkErrCh := walkFiles(done, dirPath, opts)

	// stage 2: process files with multiple workers
	numWorkers := runtime.NumCPU()
	workerChs := make([]<-chan FileResult, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workerChs[i] = processFile(done, filesCh)
	}

	// stage 3: merge & collect results
	resultsCh := merge(done, workerChs...)
	var results []FileResult
	for result := range resultsCh {
		results = append(results, result)
	}

	// check for walk errors
	if err := <-walkErrCh; err != nil {
		return results, fmt.Errorf("error walking directory: %w", err)
	}

	return results, nil
}
