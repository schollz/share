import React, { useState, useEffect, useRef } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from './AuthContext';
import toast from 'react-hot-toast';

const API_BASE = '';

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round((bytes / Math.pow(k, i)) * 10) / 10 + ' ' + sizes[i];
}

function formatDate(dateString) {
    const date = new Date(dateString);
    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
}

export default function ProfilePage() {
    const { user, token, logout, refreshProfile, isAuthenticated } = useAuth();
    const navigate = useNavigate();
    const [files, setFiles] = useState([]);
    const [loading, setLoading] = useState(true);
    const [uploading, setUploading] = useState(false);
    const [uploadProgress, setUploadProgress] = useState(null);
    const fileInputRef = useRef(null);

    useEffect(() => {
        if (!isAuthenticated) {
            navigate('/auth');
            return;
        }
        loadFiles();
    }, [isAuthenticated, navigate]);

    const loadFiles = async () => {
        try {
            const response = await fetch(`${API_BASE}/api/files`, {
                headers: {
                    'Authorization': `Bearer ${token}`
                }
            });
            
            if (response.ok) {
                const data = await response.json();
                setFiles(data || []);
            } else if (response.status === 401) {
                logout();
                navigate('/auth');
            }
        } catch (error) {
            console.error('Failed to load files:', error);
            toast.error('Failed to load files');
        } finally {
            setLoading(false);
        }
    };

    const handleFileUpload = async (e) => {
        const file = e.target.files?.[0];
        if (!file) return;

        // Check file size (100MB limit per upload)
        if (file.size > 100 * 1024 * 1024) {
            toast.error('File size must be less than 100MB');
            return;
        }

        setUploading(true);
        setUploadProgress({ percent: 0, filename: file.name });

        try {
            const formData = new FormData();
            formData.append('file', file);

            const response = await fetch(`${API_BASE}/api/files`, {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`
                },
                body: formData
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || 'Upload failed');
            }

            toast.success('File uploaded!');
            await loadFiles();
            await refreshProfile();
        } catch (error) {
            toast.error(error.message);
        } finally {
            setUploading(false);
            setUploadProgress(null);
            if (fileInputRef.current) {
                fileInputRef.current.value = '';
            }
        }
    };

    const handleDelete = async (fileId, filename) => {
        if (!confirm(`Delete "${filename}"?`)) return;

        try {
            const response = await fetch(`${API_BASE}/api/files/${fileId}`, {
                method: 'DELETE',
                headers: {
                    'Authorization': `Bearer ${token}`
                }
            });

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || 'Delete failed');
            }

            toast.success('File deleted');
            await loadFiles();
            await refreshProfile();
        } catch (error) {
            toast.error(error.message);
        }
    };

    const handleShare = async (fileId) => {
        try {
            const response = await fetch(`${API_BASE}/api/files/${fileId}/share`, {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`
                }
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || 'Failed to generate share link');
            }

            await navigator.clipboard.writeText(data.share_url);
            toast.success('Share link copied to clipboard!');
            await loadFiles();
        } catch (error) {
            toast.error(error.message);
        }
    };

    const handleDownload = (fileId, filename) => {
        // Create a hidden link and click it
        const link = document.createElement('a');
        link.href = `${API_BASE}/api/files/${fileId}`;
        link.download = filename;
        // Add auth header via fetch and blob
        fetch(`${API_BASE}/api/files/${fileId}`, {
            headers: {
                'Authorization': `Bearer ${token}`
            }
        })
        .then(response => response.blob())
        .then(blob => {
            const url = window.URL.createObjectURL(blob);
            link.href = url;
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            window.URL.revokeObjectURL(url);
        })
        .catch(error => {
            toast.error('Download failed');
            console.error('Download error:', error);
        });
    };

    const handleLogout = () => {
        logout();
        navigate('/');
    };

    const storagePercent = user ? Math.round((user.storage_used / user.storage_limit) * 100) : 0;

    if (loading) {
        return (
            <div className="min-h-screen bg-white dark:bg-black p-4 sm:p-8 font-mono flex items-center justify-center transition-colors duration-200">
                <div className="text-2xl font-black text-black dark:text-white">LOADING...</div>
            </div>
        );
    }

    return (
        <div className="min-h-screen bg-white dark:bg-black p-4 sm:p-8 font-mono transition-colors duration-200">
            <div className="max-w-4xl mx-auto">
                {/* Header */}
                <div className="bg-black dark:bg-black text-white border-4 sm:border-8 border-black dark:border-white p-4 sm:p-6 mb-4 sm:mb-6 header-shadow transition-colors duration-200"
                    style={{
                        clipPath: "polygon(0 0, calc(100% - 20px) 0, 100% 20px, 100% 100%, 0 100%)"
                    }}>
                    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                        <div>
                            <h1 className="text-3xl sm:text-4xl font-black uppercase tracking-tight mb-2">
                                <Link to="/" className="text-white no-underline hover:underline">e2ecp</Link>
                            </h1>
                            <p className="text-sm sm:text-base font-bold">
                                {user?.email?.toUpperCase() || 'YOUR PROFILE'}
                            </p>
                        </div>
                        <button
                            onClick={handleLogout}
                            className="border-2 border-white px-4 py-2 font-black uppercase text-sm hover:bg-white hover:text-black transition-colors cursor-pointer"
                        >
                            SIGN OUT
                        </button>
                    </div>
                </div>

                {/* Storage Info */}
                <div className="bg-gray-200 dark:bg-black border-4 sm:border-8 border-black dark:border-white p-4 sm:p-6 mb-4 sm:mb-6 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,1)] sm:shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:sm:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)] transition-colors duration-200">
                    <h2 className="text-lg sm:text-xl font-black uppercase mb-3 text-black dark:text-white">
                        Storage
                    </h2>
                    <div className="relative w-full h-6 sm:h-8 bg-gray-300 dark:bg-white border-2 sm:border-4 border-black dark:border-white mb-2">
                        <div
                            className="absolute top-0 left-0 h-full bg-black dark:bg-black transition-all duration-300"
                            style={{ width: `${storagePercent}%` }}
                        />
                        <div
                            className="absolute inset-0 flex items-center justify-center text-xs sm:text-sm font-bold"
                            style={{ mixBlendMode: 'difference', color: 'white' }}
                        >
                            {storagePercent}%
                        </div>
                    </div>
                    <p className="text-sm font-bold text-black dark:text-white">
                        {formatBytes(user?.storage_used || 0)} / {formatBytes(user?.storage_limit || 0)} used
                    </p>
                </div>

                {/* Upload Section */}
                <div className="bg-gray-300 dark:bg-black border-4 sm:border-8 border-black dark:border-white p-4 sm:p-6 mb-4 sm:mb-6 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,1)] sm:shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:sm:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)] transition-colors duration-200">
                    <h2 className="text-lg sm:text-xl font-black uppercase mb-3 text-black dark:text-white">
                        Upload File
                    </h2>
                    
                    {uploadProgress && (
                        <div className="mb-4">
                            <p className="text-sm font-bold text-black dark:text-white mb-2">
                                Uploading: {uploadProgress.filename}
                            </p>
                        </div>
                    )}

                    <input
                        ref={fileInputRef}
                        type="file"
                        className="hidden"
                        onChange={handleFileUpload}
                        disabled={uploading}
                    />
                    <button
                        onClick={() => fileInputRef.current?.click()}
                        disabled={uploading}
                        className={`w-full border-2 sm:border-4 border-black dark:border-white p-4 font-black uppercase text-sm sm:text-base transition-colors ${
                            uploading
                                ? 'bg-gray-400 dark:bg-gray-500 cursor-not-allowed text-white'
                                : 'bg-white dark:bg-black dark:text-white hover:bg-gray-100 dark:hover:bg-white dark:hover:text-black cursor-pointer'
                        }`}
                    >
                        {uploading ? 'UPLOADING...' : 'CLICK TO UPLOAD FILE (MAX 100MB)'}
                    </button>
                </div>

                {/* Files List */}
                <div className="bg-white dark:bg-black border-4 sm:border-8 border-black dark:border-white p-4 sm:p-6 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,1)] sm:shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:sm:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)] transition-colors duration-200">
                    <h2 className="text-lg sm:text-xl font-black uppercase mb-3 text-black dark:text-white">
                        Your Files ({files.length})
                    </h2>

                    {files.length === 0 ? (
                        <p className="text-gray-500 dark:text-gray-400 font-bold text-center py-8">
                            NO FILES YET. UPLOAD YOUR FIRST FILE!
                        </p>
                    ) : (
                        <div className="space-y-3">
                            {files.map(file => (
                                <div
                                    key={file.id}
                                    className="border-2 sm:border-4 border-black dark:border-white p-3 sm:p-4 bg-gray-100 dark:bg-black transition-colors duration-200"
                                >
                                    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                                        <div className="flex-1 min-w-0">
                                            <p className="font-black text-black dark:text-white truncate">
                                                {file.filename}
                                            </p>
                                            <p className="text-sm text-gray-600 dark:text-gray-400 font-bold">
                                                {formatBytes(file.size)} • {formatDate(file.created_at)}
                                            </p>
                                            {file.share_url && (
                                                <p className="text-xs text-green-600 dark:text-green-400 font-bold mt-1 break-all">
                                                    <i className="fas fa-link mr-1"></i>
                                                    {file.share_url}
                                                </p>
                                            )}
                                        </div>
                                        <div className="flex gap-2 flex-shrink-0">
                                            <button
                                                onClick={() => handleDownload(file.id, file.filename)}
                                                className="border-2 border-black dark:border-white px-3 py-2 font-black text-sm uppercase bg-white dark:bg-black dark:text-white hover:bg-gray-100 dark:hover:bg-white dark:hover:text-black transition-colors cursor-pointer"
                                                title="Download"
                                            >
                                                <i className="fas fa-download"></i>
                                            </button>
                                            <button
                                                onClick={() => handleShare(file.id)}
                                                className="border-2 border-black dark:border-white px-3 py-2 font-black text-sm uppercase bg-white dark:bg-black dark:text-white hover:bg-gray-100 dark:hover:bg-white dark:hover:text-black transition-colors cursor-pointer"
                                                title="Generate Share Link"
                                            >
                                                <i className="fas fa-share-nodes"></i>
                                            </button>
                                            <button
                                                onClick={() => handleDelete(file.id, file.filename)}
                                                className="border-2 border-red-600 px-3 py-2 font-black text-sm uppercase bg-red-600 text-white hover:bg-red-700 transition-colors cursor-pointer"
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

                {/* Back to home link */}
                <div className="mt-4 text-center">
                    <Link
                        to="/"
                        className="text-black dark:text-white font-bold underline hover:no-underline"
                    >
                        ← BACK TO FILE TRANSFER
                    </Link>
                </div>
            </div>
        </div>
    );
}
