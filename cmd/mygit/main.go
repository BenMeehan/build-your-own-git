package main

import (
	"bytes"
	"cmp"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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
	case "hash-object":
		if len(os.Args) < 4 || os.Args[2] != "-w" {
			fmt.Fprintf(os.Stderr, "usage: mygit hash-object -w <file>\n")
			os.Exit(1)
		}
		hashObjectCommand(os.Args[3])
	case "ls-tree":
		if len(os.Args) < 4 || os.Args[2] != "--name-only" {
			fmt.Fprintf(os.Stderr, "usage: mygit ls-tree --name-only <tree_sha>\n")
			os.Exit(1)
		}
		treeSHA := os.Args[3]
		err := lsTree(treeSHA)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
	case "write-tree":
		if len(os.Args) < 2 {
			fmt.Fprintf(os.Stderr, "usage: mygit write-tree\n")
			os.Exit(1)
		}
		cwd, _ := os.Getwd()
		if hash, err := writeTree(cwd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		} else {
			fmt.Printf("%x\n", hash)
		}
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

func hashObjectCommand(filePath string) {
	// Read the file contents
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
		os.Exit(1)
	}

	// Create the header and the full blob
	header := fmt.Sprintf("blob %d\x00", len(content))
	fullBlob := append([]byte(header), content...)

	// Compute the SHA-1 hash
	hash := sha1.Sum(fullBlob)
	hashStr := fmt.Sprintf("%x", hash)

	// Prepare the path to store the object
	objectDir := filepath.Join(".git", "objects", hashStr[:2])
	objectFile := filepath.Join(objectDir, hashStr[2:])

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(objectDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
		os.Exit(1)
	}

	// Compress the full blob using zlib
	var compressedBlob bytes.Buffer
	zlibWriter := zlib.NewWriter(&compressedBlob)
	_, err = zlibWriter.Write(fullBlob)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error compressing data: %s\n", err)
		os.Exit(1)
	}
	zlibWriter.Close()

	// Write the compressed blob to the object file
	if err := os.WriteFile(objectFile, compressedBlob.Bytes(), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing object file: %s\n", err)
		os.Exit(1)
	}

	// Print the hash
	fmt.Println(hashStr)
}

func lsTree(treeSHA string) error {
	// Convert SHA-1 hash to file path
	objectPath := filepath.Join(".git", "objects", treeSHA[:2], treeSHA[2:])
	file, err := os.Open(objectPath)
	if err != nil {
		return fmt.Errorf("could not open object file: %w", err)
	}
	defer file.Close()

	// Decompress the file using zlib
	zlibReader, err := zlib.NewReader(file)
	if err != nil {
		return fmt.Errorf("could not decompress object file: %w", err)
	}
	defer zlibReader.Close()

	// Read the decompressed data
	var buffer bytes.Buffer
	if _, err := io.Copy(&buffer, zlibReader); err != nil {
		return fmt.Errorf("could not read decompressed data: %w", err)
	}
	data := buffer.Bytes()

	// Verify tree object header
	if !bytes.HasPrefix(data, []byte("tree ")) {
		return fmt.Errorf("object is not a tree")
	}
	data = data[bytes.IndexByte(data, 0)+1:] // Skip header

	// Parse tree entries
	for len(data) > 0 {
		// Extract mode
		modeEnd := bytes.IndexByte(data, ' ')
		if modeEnd == -1 {
			return fmt.Errorf("invalid tree object format")
		}
		// mode := string(data[:modeEnd])
		data = data[modeEnd+1:]

		// Extract name
		nameEnd := bytes.IndexByte(data, 0)
		if nameEnd == -1 {
			return fmt.Errorf("invalid tree object format")
		}
		name := string(data[:nameEnd])
		data = data[nameEnd+1:]

		// Extract SHA-1 hash
		if len(data) < 20 {
			return fmt.Errorf("invalid tree object format")
		}
		// sha := data[:20]
		data = data[20:]

		// Print the name (for --name-only flag)
		fmt.Println(name)

		// Convert sha to hex (if needed for other purposes)
		// shaHex := hex.EncodeToString(sha)

		// Debug output
		// fmt.Printf("Mode: %s, Name: %s, SHA: %s\n", mode, name, shaHex)
	}

	return nil
}

func hashFile(writeObject bool, filename string) []byte {
	file, err := os.Open(filename)
	if err != nil {
		return nil
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil
	}
	if info.IsDir() {
		return nil
	}
	fileSize := int(info.Size())

	content := make([]byte, fileSize)
	_, err = file.Read(content)
	if err != nil {
		return nil
	}

	return hashObject(writeObject, "blob", fileSize, content)
}

func writeTree(path string) ([]byte, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open directory: %w", err)
	}
	defer dir.Close()

	if info, _ := dir.Stat(); !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", path)
	}

	entries, err := dir.ReadDir(0)
	if err != nil {
		return nil, fmt.Errorf("could not read directory entries: %w", err)
	}

	treeEntries := []*treeEntry{}

	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		te := new(treeEntry)
		te.name = entry.Name()
		fullPath := filepath.Join(path, te.name)
		if entry.IsDir() {
			te.mode = "040000" // directory
			te.hash, err = writeTree(fullPath)
			if err != nil {
				return nil, err
			}
		} else {
			te.mode = "100644" // regular file
			te.hash = hashFile(true, fullPath)
		}
		treeEntries = append(treeEntries, te)
	}

	slices.SortFunc(treeEntries, func(a, b *treeEntry) int {
		return cmp.Compare(a.name, b.name)
	})

	content := []byte{}
	for _, entry := range treeEntries {
		content = append(content, []byte(entry.mode)...)
		content = append(content, ' ')
		content = append(content, []byte(entry.name)...)
		content = append(content, '\000')
		content = append(content, entry.hash...)
	}

	return hashObject(true, "tree", len(content), content), nil
}

func hashObject(writeObject bool, objectType string, size int, content []byte) []byte {
	header := fmt.Sprintf("%s %d\000", objectType, size)
	fullContent := append([]byte(header), content...)

	h := sha1.New()
	h.Write(fullContent)
	hash := h.Sum(nil)

	if writeObject {
		objectDir := filepath.Join(".git", "objects", fmt.Sprintf("%02x", hash[0]))
		objectFile := filepath.Join(objectDir, fmt.Sprintf("%x", hash[1:]))

		if err := os.MkdirAll(objectDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating object directory: %s\n", err)
			os.Exit(1)
		}

		var compressedContent bytes.Buffer
		zlibWriter := zlib.NewWriter(&compressedContent)
		_, err := zlibWriter.Write(fullContent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error compressing data: %s\n", err)
			os.Exit(1)
		}
		zlibWriter.Close()

		if err := os.WriteFile(objectFile, compressedContent.Bytes(), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing object file: %s\n", err)
			os.Exit(1)
		}
	}

	return hash
}

type treeEntry struct {
	mode string
	name string
	hash []byte
}
