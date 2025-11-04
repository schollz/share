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
	Hash               string `json:"hash,omitempty"` // SHA256 hash of the file
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

// LocalRelayInfo contains local relay connection information that should be encrypted
// before transmission through the relay server
type LocalRelayInfo struct {
	IPs  []string `json:"ips"`
	Port int      `json:"port"`
}

// MarshalLocalRelayInfo converts LocalRelayInfo to JSON bytes for encryption
func MarshalLocalRelayInfo(info LocalRelayInfo) ([]byte, error) {
	return json.Marshal(info)
}

// UnmarshalLocalRelayInfo converts JSON bytes to LocalRelayInfo after decryption
func UnmarshalLocalRelayInfo(data []byte) (LocalRelayInfo, error) {
	var info LocalRelayInfo
	err := json.Unmarshal(data, &info)
	return info, err
}
