import React, { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "./AuthContext";
import {
    generateFileKey,
    encryptFileKey,
    decryptFileKey,
    encryptFile,
    decryptFile,
    encryptString,
    decryptString,
} from "./encryption";
import toast from "react-hot-toast";

export default function Profile() {
    const { user, token, encryptionKey, logout } = useAuth();
    const navigate = useNavigate();
    const [files, setFiles] = useState([]);
    const [totalStorage, setTotalStorage] = useState(0);
    const [storageLimit, setStorageLimit] = useState(0);
    const [uploading, setUploading] = useState(false);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        if (!user) {
            navigate("/login");
            return;
        }
        fetchFiles();
    }, [user, navigate]);

    const fetchFiles = async () => {
        try {
            const response = await fetch("/api/files/list", {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            });

            if (!response.ok) {
                throw new Error("Failed to fetch files");
            }

            const data = await response.json();

            // Decrypt filenames for display
            const filesWithDecryptedNames = await Promise.all(
                (data.files || []).map(async (file) => {
                    try {
                        const decryptedFilename = await decryptString(
                            file.encrypted_filename,
                            encryptionKey
                        );
                        return { ...file, filename: decryptedFilename };
                    } catch (error) {
                        console.error("Failed to decrypt filename:", error);
                        return { ...file, filename: "[Encrypted]" };
                    }
                })
            );

            setFiles(filesWithDecryptedNames);
            setTotalStorage(data.total_storage);
            setStorageLimit(data.storage_limit);
        } catch (error) {
            toast.error("Failed to load files");
            console.error(error);
        } finally {
            setLoading(false);
        }
    };

    const handleFileUpload = async (e) => {
        const file = e.target.files[0];
        if (!file) return;

        if (!encryptionKey) {
            toast.error("Encryption key not available. Please log in again.");
            return;
        }

        // Check if adding this file would exceed storage limit
        if (totalStorage + file.size > storageLimit) {
            const remaining = storageLimit - totalStorage;
            toast.error(
                `Storage limit exceeded. You have ${formatBytes(remaining)} remaining`,
            );
            return;
        }

        setUploading(true);

        try {
            // Generate a unique encryption key for this file
            toast.loading("Generating encryption key...", { id: "encrypt" });
            const fileKey = await generateFileKey();

            // Encrypt the file with the file-specific key
            toast.loading("Encrypting file...", { id: "encrypt" });
            const encryptedBlob = await encryptFile(file, fileKey);

            // Encrypt the file key with the user's master key
            const encryptedFileKey = await encryptFileKey(
                fileKey,
                encryptionKey,
            );

            // Encrypt the filename with the user's master key
            const encryptedFilename = await encryptString(file.name, encryptionKey);
            toast.success("File encrypted", { id: "encrypt" });

            const formData = new FormData();
            // Send encrypted blob (server never sees the real filename)
            formData.append("file", encryptedBlob);
            // Send the encrypted file key
            formData.append("encrypted_key", encryptedFileKey);
            // Send the encrypted filename
            formData.append("encrypted_filename", encryptedFilename);

            toast.loading("Uploading encrypted file...", { id: "upload" });
            const response = await fetch("/api/files/upload", {
                method: "POST",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
                body: formData,
            });

            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || "Upload failed");
            }

            toast.success("File uploaded successfully!", { id: "upload" });
            fetchFiles();
        } catch (error) {
            toast.error(error.message, { id: "upload" });
        } finally {
            setUploading(false);
            e.target.value = "";
        }
    };

    const handleDelete = async (fileId, filename) => {
        if (
            !confirm(`Are you sure you want to delete "${filename}"?`)
        ) {
            return;
        }

        try {
            const response = await fetch(`/api/files/${fileId}`, {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            });

            if (!response.ok) {
                throw new Error("Delete failed");
            }

            toast.success("File deleted successfully!");
            fetchFiles();
        } catch (error) {
            toast.error("Failed to delete file");
        }
    };

    const handleGenerateShareLink = async (fileId, encryptedFilename) => {
        console.log("Share button clicked, fileId:", fileId);

        if (!encryptionKey) {
            toast.error("Encryption key not available. Please log in again.");
            return;
        }

        try {
            console.log("Starting share link generation...");
            toast.loading("Generating share link...", { id: "share" });

            const response = await fetch(
                `/api/files/share/generate/${fileId}`,
                {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${token}`,
                    },
                },
            );

            if (!response.ok) {
                const errorData = await response.json();
                throw new Error(errorData.error || "Failed to generate share link");
            }

            const data = await response.json();
            console.log("Backend response:", data);

            // Check if file has an encrypted key
            if (!data.file_key || data.file_key.trim() === "") {
                toast.error("This file was uploaded without encryption. Please re-upload it to enable sharing.", { id: "share" });
                return;
            }

            // Decrypt the file key with user's master key
            console.log("Decrypting file key...");
            const fileKey = await decryptFileKey(
                data.file_key,
                encryptionKey,
            );
            console.log("File key decrypted successfully");

            // Decrypt the filename with master key, then re-encrypt with file key for sharing
            const decryptedFilename = await decryptString(encryptedFilename, encryptionKey);
            const filenameForShare = await encryptString(decryptedFilename, fileKey);

            // Export the file key as raw bytes for sharing
            const exportedKey = await window.crypto.subtle.exportKey(
                "raw",
                fileKey,
            );
            const keyArray = new Uint8Array(exportedKey);
            let keyHex = "";
            for (let i = 0; i < keyArray.length; i++) {
                keyHex += keyArray[i].toString(16).padStart(2, "0");
            }

            // Create share URL with the decryption key and filename encrypted with file key
            // Extract token from API URL: /api/files/share/token -> /share/token
            const shareToken = data.share_url.split("/").pop();
            // Format: #fileKey|filenameEncryptedWithFileKey
            const shareURL = `${window.location.origin}/share/${shareToken}#${keyHex}|${encodeURIComponent(filenameForShare)}`;

            // Copy to clipboard
            await navigator.clipboard.writeText(shareURL);
            toast.success("Link copied to clipboard!", { id: "share" });
            fetchFiles();
        } catch (error) {
            console.error("Share link error:", error);
            toast.error(error.message || "Failed to generate share link", { id: "share" });
        }
    };

    const handleDownload = async (fileId, filename, encryptedFileKey) => {
        if (!encryptionKey) {
            toast.error("Encryption key not available. Please log in again.");
            return;
        }

        try {
            toast.loading("Downloading encrypted file...", { id: "download" });
            const response = await fetch(`/api/files/download/${fileId}`, {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            });

            if (!response.ok) {
                throw new Error("Download failed");
            }

            const encryptedBlob = await response.blob();

            toast.loading("Decrypting file key...", { id: "download" });
            // Decrypt the file-specific key with user's master key
            const fileKey = await decryptFileKey(
                encryptedFileKey,
                encryptionKey,
            );

            toast.loading("Decrypting file...", { id: "download" });
            const decryptedBlob = await decryptFile(encryptedBlob, fileKey);

            const url = window.URL.createObjectURL(decryptedBlob);
            const link = document.createElement("a");
            link.href = url;
            link.download = filename;
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            window.URL.revokeObjectURL(url);

            toast.success("File downloaded and decrypted!", { id: "download" });
        } catch (error) {
            console.error("Download error:", error);
            toast.error("Failed to download or decrypt file", {
                id: "download",
            });
        }
    };

    const formatBytes = (bytes) => {
        if (bytes === 0) return "0 Bytes";
        const k = 1024;
        const sizes = ["Bytes", "KB", "MB", "GB"];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + " " + sizes[i];
    };

    const storagePercentage = (totalStorage / storageLimit) * 100;

    if (loading) {
        return (
            <div className="min-h-screen bg-white dark:bg-black text-black dark:text-white flex items-center justify-center">
                <div className="text-2xl font-bold">Loading...</div>
            </div>
        );
    }

    return (
        <div className="min-h-screen bg-white dark:bg-black text-black dark:text-white p-4 sm:p-8">
            <div className="max-w-6xl mx-auto">
                {/* Header */}
                <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center mb-8 gap-4">
                    <div>
                        <h1 className="text-4xl sm:text-5xl font-black uppercase mb-2">
                            My Files
                        </h1>
                        <p className="text-lg">{user?.email}</p>
                    </div>
                    <div className="flex gap-4">
                        <button
                            onClick={() => navigate("/")}
                            className="border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-4 py-2 font-bold uppercase hover:bg-gray-100 dark:hover:bg-gray-900 transition-colors"
                        >
                            Home
                        </button>
                        <button
                            onClick={() => {
                                logout();
                                navigate("/");
                            }}
                            className="border-2 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-4 py-2 font-bold uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors"
                        >
                            Logout
                        </button>
                    </div>
                </div>

                {/* Storage Info */}
                <div className="border-4 border-black dark:border-white p-6 mb-8 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                    <div className="mb-4">
                        <div className="flex justify-between text-lg font-bold mb-2">
                            <span>Storage Used</span>
                            <span>
                                {formatBytes(totalStorage)} /{" "}
                                {formatBytes(storageLimit)}
                            </span>
                        </div>
                        <div className="w-full h-4 border-2 border-black dark:border-white bg-white dark:bg-black">
                            <div
                                className="h-full bg-black dark:bg-white transition-all"
                                style={{ width: `${storagePercentage}%` }}
                            ></div>
                        </div>
                    </div>
                </div>

                {/* Upload Section */}
                <div className="border-4 border-black dark:border-white p-6 mb-8 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                    <h2 className="text-2xl font-black uppercase mb-4">
                        Upload File
                    </h2>
                    <label className="block w-full border-2 sm:border-4 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-4 py-3 sm:py-4 text-base sm:text-lg font-black uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors cursor-pointer shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,1)] hover:translate-x-1 hover:translate-y-1 hover:shadow-none active:translate-x-2 active:translate-y-2 text-center">
                        {uploading ? "Uploading..." : "Choose File"}
                        <input
                            type="file"
                            onChange={handleFileUpload}
                            disabled={uploading}
                            className="hidden"
                        />
                    </label>
                </div>

                {/* Files List */}
                <div className="border-4 border-black dark:border-white p-6 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                    <h2 className="text-2xl font-black uppercase mb-4">
                        Your Files ({files.length})
                    </h2>

                    {files.length === 0 ? (
                        <p className="text-lg text-center py-8">
                            No files uploaded yet
                        </p>
                    ) : (
                        <div className="space-y-4">
                            {files.map((file) => (
                                <div
                                    key={file.id}
                                    className="border-2 border-black dark:border-white p-4 bg-white dark:bg-black"
                                >
                                    <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
                                        <div className="flex-1 min-w-0">
                                            <h3 className="text-lg font-bold break-all">
                                                {file.filename}
                                            </h3>
                                            <p className="text-sm flex flex-wrap items-center gap-2">
                                                <span>
                                                    {formatBytes(file.file_size)}
                                                </span>
                                                <span>•</span>
                                                <span>
                                                    {new Date(
                                                        file.created_at,
                                                    ).toLocaleDateString()}
                                                </span>
                                                {file.share_token && (
                                                    <>
                                                        <span>•</span>
                                                        <span className="text-green-600 dark:text-green-400 font-semibold">
                                                            Share link generated
                                                        </span>
                                                        <span>•</span>
                                                        <span className="text-blue-600 dark:text-blue-300">
                                                            {file.download_count || 0} downloads
                                                        </span>
                                                    </>
                                                )}
                                            </p>
                                        </div>
                                        <div className="flex flex-wrap gap-2">
                                            <button
                                                onClick={() =>
                                                    handleDownload(
                                                        file.id,
                                                        file.filename,
                                                        file.encrypted_key,
                                                    )
                                                }
                                                className="border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-1 font-bold text-sm uppercase hover:bg-gray-100 dark:hover:bg-gray-900 transition-colors"
                                                title="Download"
                                            >
                                                <i className="fas fa-download"></i>
                                            </button>
                                            <button
                                                onClick={() =>
                                                    handleGenerateShareLink(
                                                        file.id,
                                                        file.encrypted_filename,
                                                    )
                                                }
                                                className="border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-1 font-bold text-sm uppercase hover:bg-gray-100 dark:hover:bg-gray-900 transition-colors"
                                                title="Generate share link"
                                            >
                                                <i className="fas fa-share"></i>
                                            </button>
                                            <button
                                                onClick={() =>
                                                    handleDelete(
                                                        file.id,
                                                        file.filename,
                                                    )
                                                }
                                                className="border-2 border-black dark:border-white bg-red-600 text-white px-3 py-1 font-bold text-sm uppercase hover:bg-red-700 transition-colors"
                                                title="Delete"
                                            >
                                                <i className="fas fa-trash"></i>
                                            </button>
                                        </div>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
