package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"io/fs"
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
		if err := writeTree(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
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

func writeTree() error {
	treeEntries, err := getTreeEntries(".")
	if err != nil {
		return err
	}

	treeObject, err := createTreeObject(treeEntries)
	if err != nil {
		return err
	}

	treeSHA := sha1.New()
	treeSHA.Write(treeObject)
	hash := fmt.Sprintf("%x", treeSHA.Sum(nil))

	objectPath := filepath.Join(".git", "objects", hash[:2], hash[2:])
	if err := os.MkdirAll(filepath.Dir(objectPath), 0755); err != nil {
		return err
	}
	if err := writeCompressedObject(objectPath, treeObject); err != nil {
		return err
	}

	fmt.Println(hash)
	return nil
}

func getTreeEntries(dir string) ([]treeEntry, error) {
	var entries []treeEntry

	err := filepath.WalkDir(dir, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(dir, path)
		if relPath == "." {
			return nil
		}

		if info.IsDir() {
			if relPath != "." {
				entries = append(entries, treeEntry{
					mode:  "040000",
					name:  relPath,
					sha:   getTreeSHA(path),
				})
			}
		} else {
			entries = append(entries, treeEntry{
				mode:  "100644",
				name:  relPath,
				sha:   getBlobSHA(path),
			})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

func createTreeObject(entries []treeEntry) ([]byte, error) {
	var buffer bytes.Buffer
	for _, entry := range entries {
		entryStr := fmt.Sprintf("%s %s\x00%s", entry.mode, entry.name, entry.sha)
		buffer.WriteString(entryStr)
	}

	// Debug: Print raw tree entries
	fmt.Printf("Raw tree entries: %x\n", buffer.Bytes())

	treeHeader := fmt.Sprintf("tree %d\x00", buffer.Len())
	treeObject := append([]byte(treeHeader), buffer.Bytes()...)

	// Debug: Print the final tree object before compression
	fmt.Printf("Tree object (header + entries): %x\n", treeObject)

	return treeObject, nil
}

func getBlobSHA(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	blobData := fmt.Sprintf("blob %d\x00%s", len(content), string(content))
	hash := sha1.Sum([]byte(blobData))
	return fmt.Sprintf("%x", hash[:])
}

func getTreeSHA(path string) string {
	entries, err := getTreeEntries(path)
	if err != nil {
		panic(err)
	}
	treeObject, err := createTreeObject(entries)
	if err != nil {
		panic(err)
	}
	hash := sha1.Sum(treeObject)
	return fmt.Sprintf("%x", hash[:])
}

func writeCompressedObject(objectPath string, data []byte) error {
	file, err := os.Create(objectPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zlib.NewWriter(file)
	defer writer.Close()

	_, err = writer.Write(data)
	if err != nil {
		return err
	}
	return nil
}

type treeEntry struct {
	mode string
	name string
	sha  string
}