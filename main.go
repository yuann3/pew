package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var textExtensions = map[string]bool{
	".txt": true, ".md": true, ".go": true, ".py": true, ".js": true,
	".html": true, ".css": true, ".json": true, ".xml": true, ".yaml": true,
	".yml": true, ".sh": true, ".bash": true, ".c": true, ".cpp": true,
	".h": true, ".hpp": true, ".rs": true, ".ts": true, ".java": true,
	".rb": true, ".php": true, ".pl": true, ".swift": true, ".kt": true,
	".sql": true, ".r": true, ".conf": true, ".ini": true, ".csv": true,
	".tsv": true, ".bat": true, ".ps1": true, ".lua": true,
}

type flags struct {
	outputFile       string
	dumpDir          string
	noDefaultIgnores bool
}

var binarySignatures = [][]byte{
	{0x7F, 0x45, 0x4C, 0x46}, // ELF
	{0x4D, 0x5A},             // Windows EXE
	{0x50, 0x4B, 0x03, 0x04}, // ZIP
	{0xFF, 0xD8, 0xFF},       // JPEG
	{0x89, 0x50, 0x4E, 0x47}, // PNG
	{0x25, 0x50, 0x44, 0x46}, // PDF
	{0x1F, 0x8B},             // GZIP
}

var defaultIgnorePatterns = []string{
	".*", "node_modules/", "target/", "dist/", "build/", "bin/", "pkg/", ".pewc", ".git/",
}

func main() {
	f := parseFlags()

	var markdown string
	var err error

	if f.dumpDir != "" {
		markdown, err = processDirectory(f.dumpDir, f.noDefaultIgnores)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing directory: %v\n", err)
			os.Exit(1)
		}
	} else {
		inputFiles := flag.Args()
		if len(inputFiles) == 0 {
			fmt.Fprintln(os.Stderr, "Error: no input files provided")
			os.Exit(1)
		}

		markdown, err = processFiles(inputFiles)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing files: %v\n", err)
			os.Exit(1)
		}
	}

	markdown = sanitizeContent(markdown)

	// Write to file
	err = writeMarkdownFile(f.outputFile, markdown)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to %s: %v\n", f.outputFile, err)
		os.Exit(1)
	}

	fmt.Printf("Successfully wrote Markdown to %s\n", f.outputFile)
}

func parseFlags() flags {
	var f flags

	flag.StringVar(&f.outputFile, "o", "source.md", "output markdown file")
	flag.StringVar(&f.dumpDir, "d", "", "directory to dump")
	flag.BoolVar(&f.noDefaultIgnores, "no-default-ignores", false, "disable default ignore patterns for directories like .git, node_modules, etc.")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `pew: A tool to dump source code files or directories into a Markdown file.

Usage:
pew [flags] [files...]

Flags:
-d <directory>       Directory to dump (mutually exclusive with specifying files)
-o <filename>        Output Markdown file (default: "source.md")
-h                   Show this help message
--no-default-ignores Disable default ignore patterns for directories like .git, node_modules, etc.

Examples:
Dump specific files

pew file1.go file2.go -o output.md
Dump a directory with default ignores

pew -d /path/to/project -o project.md
Dump a directory without default ignores

pew -d /path/to/project --no-default-ignores -o project.md
`)
	}

	flag.Parse()
	return f
}

func sanitizeContent(content string) string {
	content = strings.ReplaceAll(content, "├", "|")
	content = strings.ReplaceAll(content, "─", "-")
	content = strings.ReplaceAll(content, "└", "`")

	return content
}

func writeMarkdownFile(filename string, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	return nil
}

func processDirectory(dumpDir string, noDefaultIgnores bool) (string, error) {
	absDir, err := filepath.Abs(dumpDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path of %s: %w", dumpDir, err)
	}

	if _, err := os.Stat(absDir); os.IsNotExist(err) {
		return "", fmt.Errorf("directory does not exist: %s", dumpDir)
	}

	ignorePatterns, err := getIgnorePatterns(absDir, noDefaultIgnores)
	if err != nil {
		return "", fmt.Errorf("failed to get ignore patterns: %w", err)
	}

	fileList, err := collectTextFiles(absDir, ignorePatterns)
	if err != nil {
		return "", fmt.Errorf("failed to collect files: %w", err)
	}

	tree, err := generateTree(absDir, ignorePatterns)
	if err != nil {
		return "", fmt.Errorf("failed to generate tree: %w", err)
	}

	return generateMarkdown(absDir, fileList, tree), nil
}

// returns patterns from .pewc file and default patterns
func getIgnorePatterns(rootDir string, noDefaultIgnores bool) ([]string, error) {
	var patterns []string

	if !noDefaultIgnores {
		patterns = append(patterns, defaultIgnorePatterns...)
	}

	pewcPatterns, err := readPewcFile(rootDir)
	if err != nil {
		return nil, err
	}

	patterns = append(patterns, pewcPatterns...)
	return patterns, nil
}

// reads and parses the .pewc file for ignore patterns
func readPewcFile(rootDir string) ([]string, error) {
	pewcPath := filepath.Join(rootDir, ".pewc")

	content, err := os.ReadFile(pewcPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No .pewc file, that's ok
			return nil, nil
		}
		return nil, fmt.Errorf("error reading .pewc file: %w", err)
	}

	var patterns []string
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Add the pattern
		patterns = append(patterns, line)
	}

	return patterns, nil
}

func isPathIgnored(path string, rootDir string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	relPath, err := filepath.Rel(rootDir, path)
	if err != nil {
		// If we can't get relative path, use the full path
		relPath = path
	}

	relPath = filepath.ToSlash(relPath)

	for _, pattern := range patterns {
		if matchesGitIgnorePattern(relPath, pattern) {
			return true
		}
	}

	return false
}

// gitignore-style pattern matching
func matchesGitIgnorePattern(path, pattern string) bool {
	pattern = filepath.ToSlash(pattern)

	isDirOnly := strings.HasSuffix(pattern, "/")
	if isDirOnly {
		pattern = strings.TrimSuffix(pattern, "/")
	}

	isDir := strings.HasSuffix(path, "/") || isDirectory(path)
	if isDirOnly && !isDir {
		return false
	}

	// Handle wildcards
	if strings.Contains(pattern, "*") {
		regexPattern := strings.ReplaceAll(pattern, ".", "\\.")
		regexPattern = strings.ReplaceAll(regexPattern, "*", ".*")
		regexPattern = strings.ReplaceAll(regexPattern, "?", ".")

		if !strings.HasPrefix(pattern, "*") {
			regexPattern = "^" + regexPattern
		}

		if !strings.HasSuffix(pattern, "*") {
			regexPattern = regexPattern + "$"
		}

		matched, _ := filepath.Match(regexPattern, path)
		if matched {
			return true
		}

		pathParts := strings.Split(path, "/")
		for _, part := range pathParts {
			matched, _ := filepath.Match(regexPattern, part)
			if matched {
				return true
			}
		}

		return false
	}

	if pattern == filepath.Base(path) {
		return true
	}

	if pattern == path {
		return true
	}

	matched, _ := filepath.Match(pattern, path)
	return matched
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// walks the directory and collects all text files
func collectTextFiles(rootDir string, ignorePatterns []string) ([]string, error) {
	var fileList []string

	err := filepath.Walk(rootDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if isPathIgnored(path, rootDir, ignorePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		isText, err := isTextFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not check file type of %s: %v\n", path, err)
			return nil
		}

		if isText {
			fileList = append(fileList, path)
		} else {
			fmt.Printf("Skipping binary file: %s\n", path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	sort.Strings(fileList)
	return fileList, nil
}

// checks if a file is a text file by examining its content
func isTextFile(path string) (bool, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if textExtensions[ext] {
		return verifyTextContent(path)
	}

	return verifyTextContent(path)
}

// reads file content to determine if it's text
func verifyTextContent(path string) (bool, error) {
	// Read first 512 bytes to check file type
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// Read up to 512 bytes to determine the content type
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}

	return !isBinaryContent(buf[:n]), nil
}

func isBinaryContent(content []byte) bool {
	if len(content) >= 2 {
		for _, sig := range binarySignatures {
			if len(sig) <= len(content) {
				match := true
				for i := range sig {
					if content[i] != sig[i] {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
	}

	// Check for null bytes (common in binary files)
	nullCount := 0
	nonAsciiCount := 0

	for i := 0; i < len(content); i++ {
		if content[i] == 0 {
			nullCount++
			if nullCount > 1 { // Allow at most one null byte in text files
				return true
			}
		} else if content[i] > 127 {
			nonAsciiCount++
		}
	}

	// If more than 30% of characters are non-ASCII, likely binary
	if len(content) > 0 {
		return float64(nonAsciiCount)/float64(len(content)) > 0.3
	}

	return false
}

// removes control characters that aren't whitespace
func sanitizeFileContent(content []byte) string {
	var buf bytes.Buffer
	contentStr := string(content)

	for i := 0; i < len(contentStr); i++ {
		if contentStr[i] < 32 && contentStr[i] != '\n' && contentStr[i] != '\t' && contentStr[i] != '\r' {
			continue
		}
		buf.WriteByte(contentStr[i])
	}

	return buf.String()
}

// processes individual files and returns markdown content
func processFiles(files []string) (string, error) {
	var buf bytes.Buffer

	buf.WriteString("# Source Code Files\n\n")

	for _, file := range files {
		isText, err := isTextFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not check file type of %s: %v\n", file, err)
			continue
		}

		if !isText {
			fmt.Printf("Skipping binary file: %s\n", file)
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", file, err)
			continue
		}

		ext := strings.TrimPrefix(filepath.Ext(file), ".")
		if ext == "" {
			ext = "text"
		}

		buf.WriteString("## " + file + "\n\n")
		buf.WriteString("```" + ext + "\n")

		// Sanitize and write the content
		buf.WriteString(sanitizeFileContent(content))

		if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
			buf.WriteString("\n")
		}
		buf.WriteString("```\n\n")
	}

	return buf.String(), nil
}

func generateTree(rootDir string, ignorePatterns []string) (string, error) {
	var buf bytes.Buffer
	buf.WriteString(filepath.Base(rootDir) + "/\n")

	err := printDir(rootDir, "", &buf, rootDir, ignorePatterns)
	if err != nil {
		return buf.String(), fmt.Errorf("error generating tree: %w", err)
	}

	return buf.String(), nil
}

// recursively prints the directory tree
func printDir(dir, prefix string, buf *bytes.Buffer, rootDir string, ignorePatterns []string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var visibleFiles []fs.DirEntry
	for _, file := range files {
		path := filepath.Join(dir, file.Name())
		if !isPathIgnored(path, rootDir, ignorePatterns) {
			visibleFiles = append(visibleFiles, file)
		}
	}

	if len(visibleFiles) == 0 {
		return nil
	}

	sort.Slice(visibleFiles, func(i, j int) bool {
		return visibleFiles[i].Name() < visibleFiles[j].Name()
	})

	for i, file := range visibleFiles {
		path := filepath.Join(dir, file.Name())

		isLast := i == len(visibleFiles)-1
		branch := "|-- "
		newPrefix := prefix + "|   "
		if isLast {
			branch = "`-- "
			newPrefix = prefix + "    "
		}

		buf.WriteString(prefix + branch + file.Name())

		if file.IsDir() {
			buf.WriteString("/\n")
			if err := printDir(path, newPrefix, buf, rootDir, ignorePatterns); err != nil {
				return err
			}
		} else {
			buf.WriteString("\n")
		}
	}

	return nil
}

func generateMarkdown(dumpDir string, files []string, tree string) string {
	var buf bytes.Buffer

	// Create a simple plain text header
	buf.WriteString("# Directory Structure\n\n")
	buf.WriteString("```\n")
	buf.WriteString(tree)
	buf.WriteString("```\n\n")

	buf.WriteString("# File Contents\n\n")

	textFiles := 0
	for _, file := range files {
		relPath, err := filepath.Rel(dumpDir, file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting relative path of %s: %v\n", file, err)
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", file, err)
			continue
		}

		// Get file extension for proper code fence language
		ext := strings.TrimPrefix(filepath.Ext(file), ".")
		if ext == "" {
			ext = "text"
		}

		buf.WriteString("## " + relPath + "\n\n")
		buf.WriteString("```" + ext + "\n")

		buf.WriteString(sanitizeFileContent(content))

		if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
			buf.WriteString("\n")
		}
		buf.WriteString("```\n\n")

		textFiles++
	}

	if textFiles == 0 {
		buf.WriteString("No text files found in the directory.\n")
	}

	return buf.String()
}
