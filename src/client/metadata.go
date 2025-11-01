package client

import (
	"encoding/json"
)

// FileMetadata contains all sensitive file information that should be encrypted
// before transmission through the relay server
type FileMetadata struct {
	Name               string `json:"name"`
	TotalSize          int64  `json:"total_size"`
	IsFolder           bool   `json:"is_folder,omitempty"`
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	IsMultipleFiles    bool   `json:"is_multiple_files,omitempty"`
}

// MarshalMetadata converts FileMetadata to JSON bytes for encryption
func MarshalMetadata(meta FileMetadata) ([]byte, error) {
	return json.Marshal(meta)
}

// UnmarshalMetadata converts JSON bytes to FileMetadata after decryption
func UnmarshalMetadata(data []byte) (FileMetadata, error) {
	var meta FileMetadata
	err := json.Unmarshal(data, &meta)
	return meta, err
}
