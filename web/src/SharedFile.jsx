import React, { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { decryptFile, decryptString, deriveKey } from "./encryption";
import toast from "react-hot-toast";
import { Light as SyntaxHighlighter } from 'react-syntax-highlighter';
import { atomOneDark, atomOneLight } from 'react-syntax-highlighter/dist/esm/styles/hljs';

// Import language support
import javascript from 'react-syntax-highlighter/dist/esm/languages/hljs/javascript';
import typescript from 'react-syntax-highlighter/dist/esm/languages/hljs/typescript';
import python from 'react-syntax-highlighter/dist/esm/languages/hljs/python';
import java from 'react-syntax-highlighter/dist/esm/languages/hljs/java';
import cpp from 'react-syntax-highlighter/dist/esm/languages/hljs/cpp';
import c from 'react-syntax-highlighter/dist/esm/languages/hljs/c';
import csharp from 'react-syntax-highlighter/dist/esm/languages/hljs/csharp';
import php from 'react-syntax-highlighter/dist/esm/languages/hljs/php';
import ruby from 'react-syntax-highlighter/dist/esm/languages/hljs/ruby';
import go from 'react-syntax-highlighter/dist/esm/languages/hljs/go';
import rust from 'react-syntax-highlighter/dist/esm/languages/hljs/rust';
import swift from 'react-syntax-highlighter/dist/esm/languages/hljs/swift';
import kotlin from 'react-syntax-highlighter/dist/esm/languages/hljs/kotlin';
import sql from 'react-syntax-highlighter/dist/esm/languages/hljs/sql';
import bash from 'react-syntax-highlighter/dist/esm/languages/hljs/bash';
import json from 'react-syntax-highlighter/dist/esm/languages/hljs/json';
import xml from 'react-syntax-highlighter/dist/esm/languages/hljs/xml';
import yaml from 'react-syntax-highlighter/dist/esm/languages/hljs/yaml';
import markdown from 'react-syntax-highlighter/dist/esm/languages/hljs/markdown';
import css from 'react-syntax-highlighter/dist/esm/languages/hljs/css';
import scss from 'react-syntax-highlighter/dist/esm/languages/hljs/scss';
import dockerfile from 'react-syntax-highlighter/dist/esm/languages/hljs/dockerfile';
import nginx from 'react-syntax-highlighter/dist/esm/languages/hljs/nginx';

// Register languages
SyntaxHighlighter.registerLanguage('javascript', javascript);
SyntaxHighlighter.registerLanguage('typescript', typescript);
SyntaxHighlighter.registerLanguage('python', python);
SyntaxHighlighter.registerLanguage('java', java);
SyntaxHighlighter.registerLanguage('cpp', cpp);
SyntaxHighlighter.registerLanguage('c', c);
SyntaxHighlighter.registerLanguage('csharp', csharp);
SyntaxHighlighter.registerLanguage('php', php);
SyntaxHighlighter.registerLanguage('ruby', ruby);
SyntaxHighlighter.registerLanguage('go', go);
SyntaxHighlighter.registerLanguage('rust', rust);
SyntaxHighlighter.registerLanguage('swift', swift);
SyntaxHighlighter.registerLanguage('kotlin', kotlin);
SyntaxHighlighter.registerLanguage('sql', sql);
SyntaxHighlighter.registerLanguage('bash', bash);
SyntaxHighlighter.registerLanguage('json', json);
SyntaxHighlighter.registerLanguage('xml', xml);
SyntaxHighlighter.registerLanguage('yaml', yaml);
SyntaxHighlighter.registerLanguage('markdown', markdown);
SyntaxHighlighter.registerLanguage('css', css);
SyntaxHighlighter.registerLanguage('scss', scss);
SyntaxHighlighter.registerLanguage('dockerfile', dockerfile);
SyntaxHighlighter.registerLanguage('nginx', nginx);

// File type detection
const getFileType = (filename) => {
    if (!filename) return { category: 'unknown', language: 'text' };

    const ext = filename.split('.').pop().toLowerCase();

    // Image files
    const imageExts = ['jpg', 'jpeg', 'png', 'gif', 'bmp', 'webp', 'svg', 'ico'];
    if (imageExts.includes(ext)) {
        return { category: 'image', mimeType: `image/${ext === 'jpg' ? 'jpeg' : ext}` };
    }

    // Audio files
    const audioExts = ['mp3', 'wav', 'ogg', 'flac', 'm4a', 'aac', 'wma'];
    if (audioExts.includes(ext)) {
        return { category: 'audio', mimeType: `audio/${ext}` };
    }

    // Video files
    const videoExts = ['mp4', 'webm', 'ogg', 'mov', 'avi', 'wmv', 'flv', 'mkv'];
    if (videoExts.includes(ext)) {
        return { category: 'video', mimeType: `video/${ext}` };
    }

    // Code/text files with language mapping
    const languageMap = {
        'js': 'javascript',
        'jsx': 'javascript',
        'ts': 'typescript',
        'tsx': 'typescript',
        'py': 'python',
        'java': 'java',
        'cpp': 'cpp',
        'cc': 'cpp',
        'cxx': 'cpp',
        'c': 'c',
        'h': 'c',
        'hpp': 'cpp',
        'cs': 'csharp',
        'php': 'php',
        'rb': 'ruby',
        'go': 'go',
        'rs': 'rust',
        'swift': 'swift',
        'kt': 'kotlin',
        'sql': 'sql',
        'sh': 'bash',
        'bash': 'bash',
        'zsh': 'bash',
        'json': 'json',
        'xml': 'xml',
        'html': 'xml',
        'yml': 'yaml',
        'yaml': 'yaml',
        'md': 'markdown',
        'css': 'css',
        'scss': 'scss',
        'sass': 'scss',
        'dockerfile': 'dockerfile',
        'txt': 'text',
        'log': 'text',
        'conf': 'nginx',
        'config': 'text',
        'ini': 'text',
        'env': 'bash',
        'gitignore': 'text',
        'gitattributes': 'text',
        'editorconfig': 'text',
    };

    // Special case for Dockerfile
    if (filename.toLowerCase() === 'dockerfile' || filename.toLowerCase().startsWith('dockerfile.')) {
        return { category: 'code', language: 'dockerfile' };
    }

    const language = languageMap[ext] || 'text';
    return { category: 'code', language };
};

export default function SharedFile() {
    const { token } = useParams();
    const navigate = useNavigate();
    const [loading, setLoading] = useState(true);
    const [fileInfo, setFileInfo] = useState(null);
    const [decryptedFilename, setDecryptedFilename] = useState(null);
    const [downloading, setDownloading] = useState(false);
    const [preview, setPreview] = useState(null);
    const [previewing, setPreviewing] = useState(false);
    const [darkMode, setDarkMode] = useState(
        window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
    );

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

    const handlePreview = async () => {
        if (!fileInfo?.keyHex || !decryptedFilename) {
            toast.error("Cannot preview file");
            return;
        }

        setPreviewing(true);

        try {
            toast.loading("Loading preview...", { id: "preview" });
            const response = await fetch(`/api/files/share/${token}`);

            if (!response.ok) {
                throw new Error("Failed to load file");
            }

            const encryptedBlob = await response.blob();

            // Convert hex key to CryptoKey
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

            // Decrypt the file
            const decryptedBlob = await decryptFile(encryptedBlob, fileKey);
            const fileType = getFileType(decryptedFilename);

            if (fileType.category === 'code') {
                // Read as text for code files
                const text = await decryptedBlob.text();
                setPreview({
                    type: 'code',
                    content: text,
                    language: fileType.language,
                    filename: decryptedFilename
                });
            } else if (fileType.category === 'image') {
                // Create object URL for image
                const url = URL.createObjectURL(decryptedBlob);
                setPreview({
                    type: 'image',
                    url: url,
                    filename: decryptedFilename
                });
            } else if (fileType.category === 'audio') {
                // Create object URL for audio
                const url = URL.createObjectURL(decryptedBlob);
                setPreview({
                    type: 'audio',
                    url: url,
                    mimeType: fileType.mimeType,
                    filename: decryptedFilename
                });
            } else if (fileType.category === 'video') {
                // Create object URL for video
                const url = URL.createObjectURL(decryptedBlob);
                setPreview({
                    type: 'video',
                    url: url,
                    mimeType: fileType.mimeType,
                    filename: decryptedFilename
                });
            } else {
                toast.error("Preview not available for this file type", { id: "preview" });
                return;
            }

            toast.success("Preview loaded!", { id: "preview" });
        } catch (error) {
            console.error("Preview error:", error);
            toast.error("Failed to load preview", { id: "preview" });
        } finally {
            setPreviewing(false);
        }
    };

    // Auto-preview on load if it's a previewable file
    useEffect(() => {
        if (decryptedFilename && !preview && !previewing) {
            const fileType = getFileType(decryptedFilename);
            if (['code', 'image', 'audio', 'video'].includes(fileType.category)) {
                handlePreview();
            }
        }
    }, [decryptedFilename]);

    // Cleanup object URLs on unmount
    useEffect(() => {
        return () => {
            if (preview?.url) {
                URL.revokeObjectURL(preview.url);
            }
        };
    }, [preview?.url]);

    // Listen for dark mode changes
    useEffect(() => {
        const darkModeQuery = window.matchMedia('(prefers-color-scheme: dark)');
        const handleChange = (e) => setDarkMode(e.matches);
        darkModeQuery.addEventListener('change', handleChange);
        return () => darkModeQuery.removeEventListener('change', handleChange);
    }, []);

    // Animated loading dots
    const [loadingDots, setLoadingDots] = useState('.');
    useEffect(() => {
        if (loading) {
            const interval = setInterval(() => {
                setLoadingDots(prev => prev.length >= 3 ? '.' : prev + '.');
            }, 300);
            return () => clearInterval(interval);
        }
    }, [loading]);

    if (loading) {
        return (
            <div className="min-h-screen bg-white dark:bg-black text-black dark:text-white flex items-center justify-center">
                <div className="text-2xl font-bold">Loading{loadingDots}</div>
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
                            <i className="fas fa-home"></i>
                            <span className="hidden sm:inline sm:ml-2">Go Home</span>
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
                        <i className="fas fa-home"></i>
                        <span className="hidden sm:inline sm:ml-2">Home</span>
                    </button>
                </div>

                {/* Download Section */}
                <div className="border-4 border-black dark:border-white p-8 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                    <h2 className="text-2xl font-black uppercase mb-6 text-center">
                        {decryptedFilename ? decryptedFilename : "Someone shared a file with you"}
                    </h2>

                    {/* Preview */}
                    {preview && (
                        <div className="mb-6">
                            {preview.type === 'code' && (
                                <div className="overflow-auto max-h-[600px] border-2 border-black dark:border-white">
                                    <SyntaxHighlighter
                                        language={preview.language}
                                        style={darkMode ? atomOneDark : atomOneLight}
                                        showLineNumbers={true}
                                        wrapLines={true}
                                        customStyle={{
                                            margin: 0,
                                            padding: '1rem',
                                            fontSize: '0.875rem',
                                            lineHeight: '1.5'
                                        }}
                                    >
                                        {preview.content}
                                    </SyntaxHighlighter>
                                </div>
                            )}

                            {preview.type === 'image' && (
                                <div className="flex justify-center items-center border-2 border-black dark:border-white p-4 bg-gray-100 dark:bg-gray-900">
                                    <img
                                        src={preview.url}
                                        alt={preview.filename}
                                        className="max-w-full h-auto max-h-[600px] object-contain"
                                    />
                                </div>
                            )}

                            {preview.type === 'audio' && (
                                <div className="border-2 border-black dark:border-white p-4 bg-gray-100 dark:bg-gray-900">
                                    <audio
                                        controls
                                        className="w-full"
                                        style={{ maxWidth: '100%' }}
                                    >
                                        <source src={preview.url} type={preview.mimeType} />
                                        Your browser does not support audio playback.
                                    </audio>
                                </div>
                            )}

                            {preview.type === 'video' && (
                                <div className="border-2 border-black dark:border-white p-4 bg-gray-100 dark:bg-gray-900">
                                    <video
                                        controls
                                        className="w-full max-h-[600px]"
                                        style={{ maxWidth: '100%' }}
                                    >
                                        <source src={preview.url} type={preview.mimeType} />
                                        Your browser does not support video playback.
                                    </video>
                                </div>
                            )}
                        </div>
                    )}

                    <button
                        onClick={handleDownload}
                        disabled={downloading}
                        className="w-full border-2 sm:border-4 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-6 py-4 text-lg font-black uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors disabled:opacity-50 disabled:cursor-not-allowed shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,1)] hover:translate-x-1 hover:translate-y-1 hover:shadow-none active:translate-x-2 active:translate-y-2"
                    >
                        {downloading ? "Downloading & Decrypting..." : "Download File"}
                    </button>
                    <div className="mt-6 p-4 border-2 border-black dark:border-white bg-gray-100 dark:bg-gray-900">
                        <p className="text-sm">
                            <strong>Privacy Notice:</strong> This file is end-to-end encrypted. Decryption happens in your browser using the key in this URL. The server cannot access the file contents.
                        </p>
                    </div>
                </div>
            </div>
        </div>
    );
}
