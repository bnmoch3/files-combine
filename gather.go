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

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// GatherOptions configuration for file gathering
type GatherOptions struct {
	Extensions      []string
	IncludeHidden   bool
	IgnoreGitignore bool
	IgnorePatterns  []string
	IgnoreFilesOnly bool
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

		// load .gitignore patterns
		var matcher gitignore.Matcher
		if !opts.IgnoreGitignore {
			patterns, err := loadGitignorePatterns(dirPath)
			if err != nil {
				errCh <- fmt.Errorf("loading gitignore: %w", err)
				return
			}
			if len(patterns) > 0 {
				matcher = gitignore.NewMatcher(patterns)
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

			// check gitignore (skip root)
			if matcher != nil && relPath != "." {
				if shouldIgnore(matcher, relPath, d.IsDir()) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			// check custom ignore patterns
			if len(opts.IgnorePatterns) > 0 {
				if shouldIgnorePatterns(d.Name(), d.IsDir(), opts.IgnorePatterns, opts.IgnoreFilesOnly) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
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

func shouldIgnorePatterns(name string, isDir bool, patterns []string, filesOnly bool) bool {
	// if filesOnly is true and this is a directory, don't ignore
	if filesOnly && isDir {
		return false
	}

	for _, pattern := range patterns {
		matched, _ := filepath.Match(pattern, name)
		if matched {
			return true
		}
	}
	return false
}

func loadGitignorePatterns(dirPath string) ([]gitignore.Pattern, error) {
	gitignorePath := filepath.Join(dirPath, ".gitignore")

	file, err := os.Open(gitignorePath)
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

		patterns = append(patterns, gitignore.ParsePattern(line, nil))
	}

	if err := scanner.Err(); err != nil {
		return nil, err
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
