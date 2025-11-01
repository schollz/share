package client

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// Maximum uncompressed size: 10 GB
	maxUncompressedSize = 10 * 1024 * 1024 * 1024
	// Maximum compression ratio to prevent zip bombs
	maxCompressionRatio = 100
)

// CreateZipFromDirectory creates a zip archive from a directory
func CreateZipFromDirectory(sourceDir, targetZip string) error {
	// Create the zip file
	zipFile, err := os.Create(targetZip)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Get the base directory name to preserve structure
	baseDir := filepath.Base(sourceDir)

	// Create the base directory entry first (handles empty directories)
	baseDirEntry := filepath.ToSlash(baseDir) + "/"
	_, err = zipWriter.Create(baseDirEntry)
	if err != nil {
		return fmt.Errorf("failed to create base directory entry: %w", err)
	}

	// Walk the directory tree
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == sourceDir {
			return nil
		}

		// Get the relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Create zip path with base directory
		zipPath := filepath.Join(baseDir, relPath)
		// Convert to forward slashes for cross-platform compatibility
		zipPath = filepath.ToSlash(zipPath)

		// Handle directories
		if info.IsDir() {
			// Add trailing slash for directories
			zipPath = zipPath + "/"
			_, err := zipWriter.Create(zipPath)
			return err
		}

		// Skip symbolic links
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		// Add file to zip
		writer, err := zipWriter.Create(zipPath)
		if err != nil {
			return err
		}

		// Open and copy file content
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to zip directory: %w", err)
	}

	return nil
}

// ExtractZipToDirectory extracts a zip archive to a target directory and returns the list of extracted files
func ExtractZipToDirectory(zipPath, targetDir string) ([]string, error) {
	return ExtractZipToDirectoryWithOptions(zipPath, targetDir, false)
}

// ExtractZipToDirectoryWithOptions extracts a zip archive with options to strip root folder
func ExtractZipToDirectoryWithOptions(zipPath, targetDir string, stripRootFolder bool) ([]string, error) {
	// Open the zip file
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	// Check for zip bomb: calculate total uncompressed size
	var totalUncompressed int64
	var totalCompressed int64
	for _, file := range reader.File {
		totalUncompressed += int64(file.UncompressedSize64)
		totalCompressed += int64(file.CompressedSize64)
	}

	// Validate total size
	if totalUncompressed > maxUncompressedSize {
		return nil, fmt.Errorf("zip file too large: %d bytes (max %d bytes)", totalUncompressed, maxUncompressedSize)
	}

	// Validate compression ratio
	if totalCompressed > 0 {
		ratio := totalUncompressed / totalCompressed
		if ratio > maxCompressionRatio {
			return nil, fmt.Errorf("suspicious compression ratio: %d:1 (max %d:1)", ratio, maxCompressionRatio)
		}
	}

	// Create target directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err)
	}

	// Determine root folder to strip if requested
	var rootFolderToStrip string
	if stripRootFolder {
		// Find the common root folder
		for _, file := range reader.File {
			if file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/") {
				continue
			}
			parts := strings.Split(filepath.ToSlash(file.Name), "/")
			if len(parts) > 1 {
				rootFolderToStrip = parts[0] + "/"
				break
			}
		}
	}

	// Extract each file and track extracted paths
	var extractedFiles []string
	for _, file := range reader.File {
		// Strip root folder if needed
		fileName := file.Name
		if stripRootFolder && rootFolderToStrip != "" && strings.HasPrefix(fileName, rootFolderToStrip) {
			fileName = strings.TrimPrefix(fileName, rootFolderToStrip)
			if fileName == "" {
				continue // Skip the root directory itself
			}
		}

		filePath, err := extractFileWithName(file, targetDir, fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to extract %s: %w", file.Name, err)
		}
		if filePath != "" {
			extractedFiles = append(extractedFiles, filePath)
		}
	}

	return extractedFiles, nil
}

// extractFile extracts a single file from the zip archive and returns the extracted file path
func extractFile(file *zip.File, targetDir string) (string, error) {
	return extractFileWithName(file, targetDir, file.Name)
}

// extractFileWithName extracts a single file with a custom filename and returns the extracted file path
func extractFileWithName(file *zip.File, targetDir, fileName string) (string, error) {
	// Sanitize the file path to prevent zip slip
	filePath, err := sanitizeExtractPath(targetDir, fileName)
	if err != nil {
		return "", err
	}

	// Handle directories (check for trailing slash or IsDir flag)
	if file.FileInfo().IsDir() || strings.HasSuffix(fileName, "/") {
		return "", os.MkdirAll(filePath, 0755)
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return "", err
	}

	// Open the file from zip
	srcFile, err := file.Open()
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	// Create the destination file
	dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return "", err
	}
	defer dstFile.Close()

	// Copy the content
	_, err = io.Copy(dstFile, srcFile)
	return filePath, err
}

// sanitizeExtractPath validates and sanitizes file paths to prevent zip slip attacks
func sanitizeExtractPath(baseDir, filePath string) (string, error) {
	// Reject null bytes early (avoid issues with subsequent processing)
	if strings.Contains(filePath, "\x00") {
		return "", fmt.Errorf("illegal null byte in path: %s", filePath)
	}

	// Reject explicit absolute zip entry names (leading slash/backslash)
	// and Windows drive-letter rooted names like "C:\..."
	if strings.HasPrefix(filePath, "/") || strings.HasPrefix(filePath, `\`) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", filePath)
	}
	if len(filePath) >= 2 {
		// Windows drive-letter check (e.g. "C:\")
		c := filePath[0]
		if ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) && filePath[1] == ':' {
			return "", fmt.Errorf("absolute windows drive paths are not allowed: %s", filePath)
		}
	}

	// Convert zipped path slashes to OS separators, then clean
	filePath = filepath.FromSlash(filePath)
	cleanPath := filepath.Clean(filePath)

	// Clean may turn an empty name into "."; treat that as error
	if cleanPath == "." || cleanPath == "" {
		return "", fmt.Errorf("invalid path: %s", filePath)
	}

	// Get absolute base directory
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to compute absolute base directory: %w", err)
	}

	// Join (if cleanPath is absolute Join will return cleanPath)
	joined := filepath.Join(absBase, cleanPath)

	// Resolve absolute path for the joined destination
	absDest, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("failed to compute absolute destination path: %w", err)
	}

	// Ensure dest is within base by using Rel between absBase and absDest
	rel, err := filepath.Rel(absBase, absDest)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("illegal path escape: %s", filePath)
	}

	// Return normalized path (use OS path format)
	return absDest, nil
}

// GetDirectorySize calculates the total size of a directory
func GetDirectorySize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// CountFilesInDirectory counts the number of files in a directory
func CountFilesInDirectory(path string) (int, error) {
	var count int
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

// ListFilesInDirectory returns a list of all file paths in a directory
func ListFilesInDirectory(path string) ([]string, error) {
	var files []string
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, filePath)
		}
		return nil
	})
	return files, err
}
