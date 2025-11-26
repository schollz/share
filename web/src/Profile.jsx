import React, { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "./AuthContext";
import toast from "react-hot-toast";

export default function Profile() {
    const { user, token, logout } = useAuth();
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
            setFiles(data.files || []);
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

        // Check if adding this file would exceed storage limit
        if (totalStorage + file.size > storageLimit) {
            const remaining = storageLimit - totalStorage;
            toast.error(
                `Storage limit exceeded. You have ${formatBytes(remaining)} remaining`,
            );
            return;
        }

        setUploading(true);
        const formData = new FormData();
        formData.append("file", file);

        try {
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

            toast.success("File uploaded successfully!");
            fetchFiles();
        } catch (error) {
            toast.error(error.message);
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

    const handleGenerateShareLink = async (fileId) => {
        try {
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
                throw new Error("Failed to generate share link");
            }

            const data = await response.json();
            const shareURL = `${window.location.origin}${data.share_url}`;

            // Copy to clipboard
            await navigator.clipboard.writeText(shareURL);
            toast.success("Share link copied to clipboard!");
            fetchFiles();
        } catch (error) {
            toast.error("Failed to generate share link");
        }
    };

    const handleDownload = async (fileId, filename) => {
        try {
            const response = await fetch(`/api/files/download/${fileId}`, {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            });

            if (!response.ok) {
                throw new Error("Download failed");
            }

            const blob = await response.blob();
            const url = window.URL.createObjectURL(blob);
            const link = document.createElement("a");
            link.href = url;
            link.download = filename;
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            window.URL.revokeObjectURL(url);
        } catch (error) {
            toast.error("Failed to download file");
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
                                            <p className="text-sm">
                                                {formatBytes(file.file_size)} •{" "}
                                                {new Date(
                                                    file.created_at,
                                                ).toLocaleDateString()}
                                            </p>
                                            {file.share_token && (
                                                <p className="text-sm text-green-600 dark:text-green-400 mt-1">
                                                    ✓ Share link generated
                                                </p>
                                            )}
                                        </div>
                                        <div className="flex flex-wrap gap-2">
                                            <button
                                                onClick={() =>
                                                    handleDownload(
                                                        file.id,
                                                        file.filename,
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
