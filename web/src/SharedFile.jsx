import React, { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { decryptFile, decryptString, deriveKey } from "./encryption";
import toast from "react-hot-toast";

export default function SharedFile() {
    const { token } = useParams();
    const navigate = useNavigate();
    const [loading, setLoading] = useState(true);
    const [fileInfo, setFileInfo] = useState(null);
    const [decryptedFilename, setDecryptedFilename] = useState(null);
    const [downloading, setDownloading] = useState(false);

    useEffect(() => {
        const initializeFile = async () => {
            // Get the decryption key and encrypted filename from URL fragment
            // Format: #fileKey|encryptedFilename
            const hashContent = window.location.hash.slice(1);

            if (!hashContent) {
                toast.error("Invalid share link - missing decryption key");
                setLoading(false);
                return;
            }

            const [keyHex, encryptedFilename] = hashContent.split("|");

            if (!keyHex) {
                toast.error("Invalid share link - missing decryption key");
                setLoading(false);
                return;
            }

            setFileInfo({
                keyHex,
                encryptedFilename: encryptedFilename ? decodeURIComponent(encryptedFilename) : null
            });

            // Decrypt filename for display
            if (encryptedFilename && keyHex) {
                try {
                    // Convert hex key to CryptoKey
                    const keyBytes = new Uint8Array(
                        keyHex.match(/.{1,2}/g).map((byte) => parseInt(byte, 16))
                    );
                    const fileKey = await window.crypto.subtle.importKey(
                        "raw",
                        keyBytes,
                        { name: "AES-GCM", length: 256 },
                        false,
                        ["decrypt"]
                    );

                    // Decrypt the filename
                    const filename = await decryptString(decodeURIComponent(encryptedFilename), fileKey);
                    setDecryptedFilename(filename);
                } catch (error) {
                    console.error("Failed to decrypt filename:", error);
                    // Don't show error, just proceed without filename
                }
            }

            setLoading(false);
        };

        initializeFile();
    }, [token]);

    const handleDownload = async () => {
        if (!fileInfo?.keyHex) {
            toast.error("Decryption key not found");
            return;
        }

        setDownloading(true);

        try {
            // Download encrypted file
            toast.loading("Downloading encrypted file...", { id: "download" });
            const response = await fetch(`/api/files/share/${token}`);

            if (!response.ok) {
                throw new Error("Failed to download file");
            }

            const encryptedBlob = await response.blob();

            // Convert hex key to CryptoKey
            toast.loading("Decrypting file...", { id: "download" });
            const keyBytes = new Uint8Array(
                fileInfo.keyHex.match(/.{1,2}/g).map((byte) => parseInt(byte, 16))
            );
            const fileKey = await window.crypto.subtle.importKey(
                "raw",
                keyBytes,
                { name: "AES-GCM", length: 256 },
                false,
                ["decrypt"]
            );

            // Use already decrypted filename or fall back to default
            const filename = decryptedFilename || "download";

            // Decrypt the file
            const decryptedBlob = await decryptFile(encryptedBlob, fileKey);

            // Trigger download
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
        } finally {
            setDownloading(false);
        }
    };

    if (loading) {
        return (
            <div className="min-h-screen bg-white dark:bg-black text-black dark:text-white flex items-center justify-center">
                <div className="text-2xl font-bold">Loading...</div>
            </div>
        );
    }

    if (!fileInfo?.keyHex) {
        return (
            <div className="min-h-screen bg-white dark:bg-black text-black dark:text-white flex items-center justify-center p-4">
                <div className="max-w-2xl w-full">
                    <div className="border-4 border-black dark:border-white p-8 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                        <h1 className="text-4xl font-black uppercase mb-4 text-red-600">
                            Invalid Link
                        </h1>
                        <p className="text-lg mb-6">
                            This share link is invalid or missing the decryption key.
                        </p>
                        <button
                            onClick={() => navigate("/")}
                            className="border-2 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-6 py-3 font-bold uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors"
                        >
                            Go Home
                        </button>
                    </div>
                </div>
            </div>
        );
    }

    return (
        <div className="min-h-screen bg-white dark:bg-black text-black dark:text-white p-4 sm:p-8">
            <div className="max-w-4xl mx-auto">
                {/* Header */}
                <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center mb-8 gap-4">
                    <h1 className="text-4xl sm:text-5xl font-black uppercase">
                        Shared File
                    </h1>
                    <button
                        onClick={() => navigate("/")}
                        className="border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-4 py-2 font-bold uppercase hover:bg-gray-100 dark:hover:bg-gray-900 transition-colors"
                    >
                        Home
                    </button>
                </div>

                {/* Download Section */}
                <div className="border-4 border-black dark:border-white p-8 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                    <div className="flex items-center justify-center mb-6">
                        <div className="text-6xl">📁</div>
                    </div>
                    <h2 className="text-2xl font-black uppercase mb-4 text-center">
                        {decryptedFilename ? decryptedFilename : "Someone shared a file with you"}
                    </h2>
                    <p className="text-lg mb-8 text-center">
                        This file is end-to-end encrypted. Only you can decrypt it
                        with the key included in this link.
                    </p>
                    <button
                        onClick={handleDownload}
                        disabled={downloading}
                        className="w-full border-2 sm:border-4 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-6 py-4 text-lg font-black uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors disabled:opacity-50 disabled:cursor-not-allowed shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,1)] hover:translate-x-1 hover:translate-y-1 hover:shadow-none active:translate-x-2 active:translate-y-2"
                    >
                        {downloading ? "Downloading & Decrypting..." : "Download File"}
                    </button>
                    <div className="mt-6 p-4 border-2 border-black dark:border-white bg-gray-100 dark:bg-gray-900">
                        <p className="text-sm">
                            <strong>Privacy Notice:</strong> The server cannot
                            access this file's contents. The decryption happens
                            entirely in your browser using the key embedded in this
                            URL.
                        </p>
                    </div>
                </div>
            </div>
        </div>
    );
}
