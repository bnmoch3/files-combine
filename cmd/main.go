package main

import (
	filescombine "github.com/bnmoch3/files-combine"
)

func main() {
	done := make(chan struct{})
	defer close(done)

	dirPath := "."

	filescombine.Run(done, dirPath)
}
