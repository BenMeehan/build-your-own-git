package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Usage: your_program.sh <command> <arg1> <arg2> ...
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":
		initCommand()
	case "cat-file":
		if len(os.Args) < 4 || os.Args[2] != "-p" {
			fmt.Fprintf(os.Stderr, "usage: mygit cat-file -p <object>\n")
			os.Exit(1)
		}
		catFileCommand(os.Args[3])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}

func initCommand() {
	for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
			os.Exit(1)
		}
	}

	headFileContents := []byte("ref: refs/heads/main\n")
	if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("Initialized git directory")
}

func catFileCommand(objectHash string) {
	if len(objectHash) != 40 {
		fmt.Fprintf(os.Stderr, "Invalid object hash\n")
		os.Exit(1)
	}

	// Determine the path to the object file
	objectDir := filepath.Join(".git", "objects", objectHash[:2])
	objectFile := filepath.Join(objectDir, objectHash[2:])

	// Open the object file
	file, err := os.Open(objectFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening object file: %s\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Decompress the file using zlib
	zlibReader, err := zlib.NewReader(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decompressing object file: %s\n", err)
		os.Exit(1)
	}
	defer zlibReader.Close()

	// Read the decompressed data
	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, zlibReader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading decompressed data: %s\n", err)
		os.Exit(1)
	}

	// Extract the content after "blob <size>\0"
	data := buffer.Bytes()
	nullByteIndex := bytes.IndexByte(data, 0)
	if nullByteIndex == -1 {
		fmt.Fprintf(os.Stderr, "Invalid object format\n")
		os.Exit(1)
	}

	// Print the content without a newline at the end
	fmt.Print(string(data[nullByteIndex+1:]))
}
