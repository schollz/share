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

// ExtractZipToDirectory extracts a zip archive to a target directory
func ExtractZipToDirectory(zipPath, targetDir string) error {
	// Open the zip file
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
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
		return fmt.Errorf("zip file too large: %d bytes (max %d bytes)", totalUncompressed, maxUncompressedSize)
	}

	// Validate compression ratio
	if totalCompressed > 0 {
		ratio := totalUncompressed / totalCompressed
		if ratio > maxCompressionRatio {
			return fmt.Errorf("suspicious compression ratio: %d:1 (max %d:1)", ratio, maxCompressionRatio)
		}
	}

	// Create target directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Extract each file
	for _, file := range reader.File {
		err := extractFile(file, targetDir)
		if err != nil {
			return fmt.Errorf("failed to extract %s: %w", file.Name, err)
		}
	}

	return nil
}

// extractFile extracts a single file from the zip archive
func extractFile(file *zip.File, targetDir string) error {
	// Sanitize the file path to prevent zip slip
	filePath, err := sanitizeExtractPath(targetDir, file.Name)
	if err != nil {
		return err
	}

	// Handle directories (check for trailing slash or IsDir flag)
	if file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/") {
		return os.MkdirAll(filePath, 0755)
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	// Open the file from zip
	srcFile, err := file.Open()
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create the destination file
	dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy the content
	_, err = io.Copy(dstFile, srcFile)
	return err
}

// sanitizeExtractPath validates and sanitizes file paths to prevent zip slip attacks
func sanitizeExtractPath(baseDir, filePath string) (string, error) {
	// Convert to platform-specific path
	filePath = filepath.FromSlash(filePath)

	// Clean the path to remove .. and other tricks
	cleanPath := filepath.Clean(filePath)

	// Check for absolute paths
	if filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("illegal absolute path: %s", filePath)
	}

	// Check for null bytes
	if strings.Contains(cleanPath, "\x00") {
		return "", fmt.Errorf("illegal null byte in path: %s", filePath)
	}

	// Join with base directory
	fullPath := filepath.Join(baseDir, cleanPath)

	// Resolve to absolute path to check for escapes
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Ensure the path is still within the base directory
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return "", fmt.Errorf("illegal path escape: %s", filePath)
	}

	return fullPath, nil
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
