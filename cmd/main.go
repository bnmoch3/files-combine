package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// FileInput input for downstream processing
type FileInput struct {
	Path    string
	RelPath string
}

// FileResult output from processing
type FileResult struct {
	Path    string
	RelPath string
	Hash    string
	Err     error
}

func walkFiles(done <-chan struct{}, dirPath string) (<-chan FileInput, <-chan error) {
	out := make(chan FileInput)
	errCh := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errCh)

		err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// skip directories and non-regular files
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}

			relPath, err := filepath.Rel(dirPath, path)
			if err != nil {
				return err
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

func calculateMD5(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func processFile(done <-chan struct{}, in <-chan FileInput) <-chan FileResult {
	out := make(chan FileResult)

	go func() {
		defer close(out)

		for input := range in {
			hash, err := calculateMD5(input.Path)

			result := FileResult{
				Path:    input.Path,
				RelPath: input.RelPath,
				Hash:    hash,
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

func main() {
	done := make(chan struct{})
	defer close(done)

	dirPath := "."

	// stage 1: walk dirPath and generate file inputs
	filesCh, walkErrCh := walkFiles(done, dirPath)

	// stage 2: process files with multiple workers
	numWorkers := runtime.NumCPU()
	workerChs := make([]<-chan FileResult, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workerChs[i] = processFile(done, filesCh)
	}

	// stage 3: merge & consume results
	resultsCh := merge(done, workerChs...)
	var errors []error
	for result := range resultsCh {
		if result.Err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", result.RelPath, result.Err))
			continue
		}
		fmt.Printf("%s: %s\n", result.RelPath, result.Hash)
	}

	if len(errors) > 0 {
		log.Println("\nErrors encountered:")
		for _, err := range errors {
			log.Printf("  - %v", err)
		}
		os.Exit(1)
	}

	// check for walk errors
	if err := <-walkErrCh; err != nil {
		log.Printf("Error walking directory: %v", err)
		os.Exit(1)
	}
}
