import React from "react";
import { useNavigate } from "react-router-dom";
import { useConfig } from "./ConfigContext";
import Navbar from "./Navbar";

export default function About() {
    const navigate = useNavigate();
    const { storageEnabled } = useConfig();

    return (
        <div className="min-h-screen bg-white dark:bg-black text-black dark:text-white">
            <Navbar title="About" />

            {/* Content */}
            <div className="max-w-4xl mx-auto px-4 py-8 space-y-6">
                {/* What is E2ECP */}
                <div className="border-4 border-black dark:border-white p-6 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                    <h2 className="text-3xl font-black uppercase mb-4">What is E2ECP?</h2>
                    <p className="text-base sm:text-lg leading-relaxed mb-3">
                        <strong>E2ECP (End-to-End Copy)</strong> is a secure, peer-to-peer file transfer tool that enables you to send files directly between devices.
                    </p>
                    <p className="text-base sm:text-lg leading-relaxed">
                        Files are transferred securely between peers, where the server only facilitates the connection handshake, ensuring your data remains private and transfers happen at maximum speed.
                    </p>
                </div>

                {/* Browser Transfer */}
                <div className="border-4 border-black dark:border-white p-6 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                    <h2 className="text-3xl font-black uppercase mb-4">Browser to Browser</h2>
                    <p className="text-base sm:text-lg leading-relaxed mb-3">
                        Transfer files between browsers instantly using a simple room code:
                    </p>
                    <ol className="list-decimal list-inside space-y-2 text-base sm:text-lg ml-4">
                        <li>Open the home page and enter a room name (or use a random one)</li>
                        <li>Share the room code with another person</li>
                        <li>Both devices connect to the same room</li>
                        <li>Drag and drop files or click to upload</li>
                        <li>Files transfer directly with end-to-end encryption</li>
                    </ol>
                    <p className="text-base sm:text-lg leading-relaxed mt-3">
                        <strong>No server storage</strong> - files go directly from one browser to another.
                    </p>
                </div>

                {/* CLI Usage */}
                <div className="border-4 border-black dark:border-white p-6 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                    <h2 className="text-3xl font-black uppercase mb-4">Command Line Interface</h2>
                    <p className="text-base sm:text-lg leading-relaxed mb-3">
                        E2ECP includes a powerful CLI for terminal-based file transfers:
                    </p>
                    <div className="bg-black dark:bg-white text-white dark:text-black p-4 font-mono text-sm border-2 border-black dark:border-white mb-3 nocase">
                        <div># send a file</div>
                        <div>e2ecp send myfile.txt</div>
                        <div className="mt-2"># receive a file with room code</div>
                        <div>e2ecp receive room-code</div>
                    </div>
                    <p className="text-base sm:text-lg leading-relaxed">
                        The CLI supports the same peer-to-peer transfer mechanism as the browser, making it perfect for server-to-server transfers or scripting automated file transfers.
                    </p>
                </div>

                {/* CLI + Browser */}
                <div className="border-4 border-black dark:border-white p-6 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                    <h2 className="text-3xl font-black uppercase mb-4">CLI to Browser Transfer</h2>
                    <p className="text-base sm:text-lg leading-relaxed mb-3">
                        Seamlessly transfer files between the command line and browser:
                    </p>
                    <ul className="list-disc list-inside space-y-2 text-base sm:text-lg ml-4">
                        <li><strong>CLI to Browser:</strong> Run <code className="bg-gray-200 dark:bg-gray-800 px-2 py-1 nocase">e2ecp send file.zip</code>, get a room code, then open that room in a browser to download</li>
                        <li><strong>Browser to CLI:</strong> Create a room in the browser, upload files, then run <code className="nocase bg-gray-200 dark:bg-gray-800 px-2 py-1">e2ecp receive room-code</code> to download</li>
                        <li>Perfect for transferring files between your laptop and server</li>
                        <li>Works across different networks and NATs</li>
                    </ul>
                </div>

                {/* File Storage */}
                <div className="border-4 border-black dark:border-white p-6 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                    <h2 className="text-3xl font-black uppercase mb-4">File Storage</h2>
                    <p className="text-base sm:text-lg leading-relaxed mb-3">
                        In addition to peer-to-peer transfers, E2ECP offers optional <strong>end-to-end encrypted storage</strong>:
                    </p>
                    <ul className="list-disc list-inside space-y-2 text-base sm:text-lg ml-4">
                        <li>Create an account to store files temporarily</li>
                        <li>All files are encrypted on your device before upload</li>
                        <li>The server stores only encrypted data and cannot decrypt your files</li>
                        <li>Filenames are also encrypted for maximum privacy</li>
                        <li>Generate shareable links with encryption keys in the URL fragment</li>
                        <li>Perfect for temporary file hosting or transferring files asynchronously</li>
                    </ul>
                    {storageEnabled ? (
                        <button
                            onClick={() => navigate("/login")}
                            className="mt-4 border-2 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-6 py-3 font-black uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors cursor-pointer shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,1)] hover:translate-x-1 hover:translate-y-1 hover:shadow-none active:translate-x-2 active:translate-y-2"
                        >
                            Get Started with Storage
                        </button>
                    ) : (
                        <p className="mt-4 text-sm text-gray-600 dark:text-gray-400">
                            Storage is currently disabled on this server.
                        </p>
                    )}
                </div>

                {/* How to Upload */}
                <div className="border-4 border-black dark:border-white p-6 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                    <h2 className="text-3xl font-black uppercase mb-4">How to Upload Files</h2>
                    <div className="space-y-4">
                        <div>
                            <h3 className="text-xl font-bold mb-2">For Peer-to-Peer Transfer:</h3>
                            <ol className="list-decimal list-inside space-y-1 text-base sm:text-lg ml-4">
                                <li>Enter or create a room on the home page</li>
                                <li>Wait for both devices to connect</li>
                                <li>Drag and drop files or click the upload area</li>
                                <li>Files transfer instantly to connected peers</li>
                            </ol>
                        </div>
                        <div>
                            <h3 className="text-xl font-bold mb-2">For Encrypted Storage:</h3>
                            <ol className="list-decimal list-inside space-y-1 text-base sm:text-lg ml-4">
                                <li>Sign up or log in to your account</li>
                                <li>Navigate to the Storage page</li>
                                <li>Click "Choose File" to select files</li>
                                <li>Files are encrypted locally and uploaded</li>
                                <li>Generate share links or download later</li>
                            </ol>
                        </div>
                    </div>
                </div>

                {/* Source Code */}
                <div className="border-4 border-black dark:border-white p-6 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                    <h2 className="text-3xl font-black uppercase mb-4">Open Source</h2>
                    <p className="text-base sm:text-lg leading-relaxed mb-3">
                        E2ECP is free and open source software. You can view the code, contribute, or self-host your own instance.
                    </p>
                    <a
                        href="https://github.com/schollz/e2ecp"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-block border-2 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-6 py-3 font-black uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors cursor-pointer shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,1)] hover:translate-x-1 hover:translate-y-1 hover:shadow-none active:translate-x-2 active:translate-y-2"
                    >
                        <i className="fab fa-github mr-2"></i>
                        View on GitHub
                    </a>
                </div>

                {/* Footer */}
                <div className="text-center text-sm text-gray-600 dark:text-gray-400 py-4">
                    Built with WebRTC, Go, and React
                </div>
            </div>
        </div>
    );
}
