# Client-Side File Encryption

## Overview

All files are encrypted **client-side** before being uploaded to the server. The server only stores encrypted blobs and cannot access the plaintext files.

## How It Works

### 1. Key Derivation
- When a user registers, a random 32-byte salt is generated and stored in the database
- When logging in, the client derives an AES-256 encryption key from:
  - User's password
  - User's unique salt
  - PBKDF2 with 100,000 iterations and SHA-256

### 2. File Upload (Encryption)
1. User selects a file
2. **Client-side**: File is encrypted using AES-GCM with the derived key
3. A random 12-byte IV is generated for each file
4. IV is prepended to the encrypted data
5. Encrypted blob is uploaded to server
6. Server stores the encrypted blob (cannot decrypt it)

### 3. File Download (Decryption)
1. User requests to download a file
2. Server sends the encrypted blob
3. **Client-side**: IV is extracted from the first 12 bytes
4. File is decrypted using AES-GCM with the derived key
5. Decrypted file is saved to user's device

## Security Features

✅ **Zero-Knowledge**: Server never has access to plaintext files
✅ **Password-Based**: Encryption key is derived from user's password
✅ **Unique Keys**: Each user has a unique salt for key derivation
✅ **Strong Encryption**: AES-256-GCM with authenticated encryption
✅ **Random IVs**: Each file gets a unique IV for encryption
✅ **Ephemeral Keys**: Encryption key only exists in memory during session

## Important Notes

### Storage Tracking
- Storage usage reflects the **encrypted** file size
- Encrypted files are slightly larger than originals (~12 bytes for IV + GCM tag)

### Password Reset
⚠️ **If a user forgets their password, their files are permanently lost**
- The encryption key can only be derived from the password
- There is no password recovery mechanism
- This is by design for zero-knowledge encryption

### Session Management
- Encryption key is only available during the active session
- After page refresh, user must log in again to re-derive the key
- This ensures the key is never persisted

### Share Links
⚠️ **Current limitation**: Share links provide encrypted files
- Recipients would need the decryption key to access files
- Consider implementing a separate sharing mechanism with re-encryption

## Technical Implementation

### Backend
- `users.encryption_salt`: Stores the user's unique salt (hex-encoded)
- Files are stored as-is (encrypted blobs)
- No decryption logic on server

### Frontend
- `encryption.js`: Encryption utilities using Web Crypto API
- `AuthContext`: Derives and manages encryption key
- `Profile`: Encrypts before upload, decrypts after download

### Crypto Primitives
- **KDF**: PBKDF2 with 100,000 iterations
- **Hash**: SHA-256
- **Cipher**: AES-GCM
- **Key Size**: 256 bits
- **IV Size**: 96 bits (12 bytes)

## Future Enhancements

1. **Secure Sharing**: Implement file sharing with recipient's public key
2. **Password Change**: Allow password change with file re-encryption
3. **Key Backup**: Optional encrypted key backup with recovery phrase
4. **Metadata Encryption**: Encrypt filenames as well
