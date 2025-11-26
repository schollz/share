// Client-side file encryption utilities using Web Crypto API
// Each file gets its own random encryption key (DEK - Data Encryption Key)
// The file key is encrypted with the user's master key (KEK - Key Encryption Key)

const PBKDF2_ITERATIONS = 100000;
const KEY_LENGTH = 256;

/**
 * Derive an encryption key from password and salt using PBKDF2
 */
export async function deriveKey(password, saltHex) {
    const encoder = new TextEncoder();
    const passwordBuffer = encoder.encode(password);
    const saltBuffer = hexToBytes(saltHex);

    // Import password as key material
    const keyMaterial = await window.crypto.subtle.importKey(
        "raw",
        passwordBuffer,
        "PBKDF2",
        false,
        ["deriveBits", "deriveKey"]
    );

    // Derive AES-GCM key
    const key = await window.crypto.subtle.deriveKey(
        {
            name: "PBKDF2",
            salt: saltBuffer,
            iterations: PBKDF2_ITERATIONS,
            hash: "SHA-256",
        },
        keyMaterial,
        { name: "AES-GCM", length: KEY_LENGTH },
        false,
        ["encrypt", "decrypt"]
    );

    return key;
}

/**
 * Generate a random file-specific encryption key
 */
export async function generateFileKey() {
    return await window.crypto.subtle.generateKey(
        { name: "AES-GCM", length: KEY_LENGTH },
        true,
        ["encrypt", "decrypt"]
    );
}

/**
 * Encrypt a file key with the user's master key
 * Returns base64-encoded encrypted key with IV
 */
export async function encryptFileKey(fileKey, masterKey) {
    const iv = window.crypto.getRandomValues(new Uint8Array(12));
    const exportedKey = await window.crypto.subtle.exportKey("raw", fileKey);

    const encryptedKey = await window.crypto.subtle.encrypt(
        { name: "AES-GCM", iv: iv },
        masterKey,
        exportedKey
    );

    // Prepend IV to encrypted key
    const result = new Uint8Array(iv.length + encryptedKey.byteLength);
    result.set(iv, 0);
    result.set(new Uint8Array(encryptedKey), iv.length);

    return bytesToBase64(result);
}

/**
 * Decrypt a file key with the user's master key
 */
export async function decryptFileKey(encryptedKeyBase64, masterKey) {
    const encryptedKeyBytes = base64ToBytes(encryptedKeyBase64);
    const iv = encryptedKeyBytes.slice(0, 12);
    const encryptedKey = encryptedKeyBytes.slice(12);

    const decryptedKey = await window.crypto.subtle.decrypt(
        { name: "AES-GCM", iv: iv },
        masterKey,
        encryptedKey
    );

    return await window.crypto.subtle.importKey(
        "raw",
        decryptedKey,
        { name: "AES-GCM", length: KEY_LENGTH },
        true,  // extractable: must be true to allow exporting for share links
        ["encrypt", "decrypt"]
    );
}

/**
 * Encrypt a file using a file-specific key
 * Returns encrypted data with IV prepended
 */
export async function encryptFile(file, fileKey) {
    const arrayBuffer = await file.arrayBuffer();
    const iv = window.crypto.getRandomValues(new Uint8Array(12)); // 96-bit IV for GCM

    const encryptedData = await window.crypto.subtle.encrypt(
        {
            name: "AES-GCM",
            iv: iv,
        },
        fileKey,
        arrayBuffer
    );

    // Prepend IV to encrypted data
    const result = new Uint8Array(iv.length + encryptedData.byteLength);
    result.set(iv, 0);
    result.set(new Uint8Array(encryptedData), iv.length);

    return new Blob([result], { type: "application/octet-stream" });
}

/**
 * Decrypt a file using a file-specific key
 * Expects IV to be prepended to the encrypted data
 */
export async function decryptFile(encryptedBlob, fileKey) {
    const arrayBuffer = await encryptedBlob.arrayBuffer();
    const dataView = new Uint8Array(arrayBuffer);

    // Extract IV (first 12 bytes)
    const iv = dataView.slice(0, 12);
    const encryptedData = dataView.slice(12);

    const decryptedData = await window.crypto.subtle.decrypt(
        {
            name: "AES-GCM",
            iv: iv,
        },
        fileKey,
        encryptedData
    );

    return new Blob([decryptedData], { type: "application/octet-stream" });
}

/**
 * Convert hex string to Uint8Array
 */
function hexToBytes(hex) {
    const bytes = new Uint8Array(hex.length / 2);
    for (let i = 0; i < hex.length; i += 2) {
        bytes[i / 2] = parseInt(hex.substr(i, 2), 16);
    }
    return bytes;
}

/**
 * Convert base64 string to Uint8Array
 */
function base64ToBytes(base64) {
    const binary = atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
        bytes[i] = binary.charCodeAt(i);
    }
    return bytes;
}

/**
 * Convert Uint8Array to base64 string
 */
function bytesToBase64(bytes) {
    let binary = "";
    for (let i = 0; i < bytes.length; i++) {
        binary += String.fromCharCode(bytes[i]);
    }
    return btoa(binary);
}

/**
 * Encrypt a string (like filename) with the user's master key
 * Returns base64-encoded encrypted string with IV
 */
export async function encryptString(plaintext, masterKey) {
    const encoder = new TextEncoder();
    const plaintextBytes = encoder.encode(plaintext);
    const iv = window.crypto.getRandomValues(new Uint8Array(12));

    const encryptedData = await window.crypto.subtle.encrypt(
        { name: "AES-GCM", iv: iv },
        masterKey,
        plaintextBytes
    );

    // Prepend IV to encrypted data
    const result = new Uint8Array(iv.length + encryptedData.byteLength);
    result.set(iv, 0);
    result.set(new Uint8Array(encryptedData), iv.length);

    return bytesToBase64(result);
}

/**
 * Decrypt a string (like filename) with the user's master key
 */
export async function decryptString(encryptedBase64, masterKey) {
    const encryptedBytes = base64ToBytes(encryptedBase64);
    const iv = encryptedBytes.slice(0, 12);
    const encryptedData = encryptedBytes.slice(12);

    const decryptedData = await window.crypto.subtle.decrypt(
        { name: "AES-GCM", iv: iv },
        masterKey,
        encryptedData
    );

    const decoder = new TextDecoder();
    return decoder.decode(decryptedData);
}
