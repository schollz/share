import React, { useCallback, useEffect, useRef, useState } from "react";

/* ---------- Crypto helpers (ECDH + AES-GCM) ---------- */

// Generate ephemeral ECDH key pair
async function generateECDHKeyPair() {
    return await window.crypto.subtle.generateKey(
        { name: "ECDH", namedCurve: "P-256" },
        true,
        ["deriveKey"]
    );
}

// Export our public key for sharing
async function exportPubKey(pubKey) {
    const raw = await window.crypto.subtle.exportKey("raw", pubKey);
    return new Uint8Array(raw);
}

// Import peer's public key
async function importPeerPubKey(rawBytes) {
    return await window.crypto.subtle.importKey(
        "raw",
        rawBytes,
        { name: "ECDH", namedCurve: "P-256" },
        false,
        []
    );
}

// Derive AES key from shared secret
async function deriveSharedKey(privKey, peerPubKey) {
    return await window.crypto.subtle.deriveKey(
        { name: "ECDH", public: peerPubKey },
        privKey,
        { name: "AES-GCM", length: 256 },
        false,
        ["encrypt", "decrypt"]
    );
}

// AES-GCM encrypt/decrypt
async function encryptBytes(aesKey, plainBuffer) {
    const iv = window.crypto.getRandomValues(new Uint8Array(12));
    const ciphertext = await window.crypto.subtle.encrypt(
        { name: "AES-GCM", iv },
        aesKey,
        plainBuffer
    );
    return { iv, ciphertext: new Uint8Array(ciphertext) };
}

async function decryptBytes(aesKey, ivBytes, cipherBytes) {
    const plain = await window.crypto.subtle.decrypt(
        { name: "AES-GCM", iv: ivBytes },
        aesKey,
        cipherBytes
    );
    return new Uint8Array(plain);
}

// Helpers: base64 encode/decode
function uint8ToBase64(u8) {
    let binary = '';
    for (let i = 0; i < u8.length; i++) {
        binary += String.fromCharCode(u8[i]);
    }
    return btoa(binary);
}
function base64ToUint8(b64) {
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
}

/* ------------------- Helper Functions ------------------- */

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i];
}

function formatSpeed(bytesPerSecond) {
    return formatBytes(bytesPerSecond) + '/s';
}

function formatTime(seconds) {
    if (seconds < 1) return '< 1s';
    if (seconds < 60) return Math.round(seconds) + 's';
    if (seconds < 3600) {
        const mins = Math.floor(seconds / 60);
        const secs = Math.round(seconds % 60);
        return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`;
    }
    const hours = Math.floor(seconds / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`;
}

/* ------------------- React Component ------------------- */

function ProgressBar({ progress, label }) {
    if (!progress) return null;

    // Remove emoji from label
    const cleanLabel = label.replace(/[\u{1F300}-\u{1F9FF}]/gu, '').trim();

    return (
        <div className="bg-white border-2 sm:border-4 border-black p-3 sm:p-4 mb-3 sm:mb-4">
            <div className="text-sm sm:text-base font-black mb-2 uppercase">{cleanLabel}</div>
            <div className="relative w-full h-6 sm:h-8 bg-gray-300 border-2 sm:border-4 border-black">
                <div
                    className="absolute top-0 left-0 h-full bg-black transition-all duration-300"
                    style={{ width: `${progress.percent}%` }}
                />
                <div className="absolute inset-0 flex items-center justify-center text-xs sm:text-sm font-bold">
                    {progress.percent}%
                </div>
            </div>
            {(progress.speed > 0 || progress.eta > 0) && (
                <div className="mt-2 text-xs sm:text-sm font-bold flex flex-wrap gap-x-4 gap-y-1">
                    {progress.speed > 0 && (
                        <span>Speed: {formatSpeed(progress.speed)}</span>
                    )}
                    {progress.eta > 0 && progress.percent < 100 && (
                        <span>ETA: {formatTime(progress.eta)}</span>
                    )}
                </div>
            )}
        </div>
    );
}

export default function App() {
    // Parse room from URL path (e.g., /myroom -> "myroom")
    const pathRoom = window.location.pathname.slice(1).toLowerCase();
    const [roomId, setRoomId] = useState(pathRoom);
    const [connected, setConnected] = useState(false);
    const [peerCount, setPeerCount] = useState(1);
    const [status, setStatus] = useState("Not connected");
    const [downloadUrl, setDownloadUrl] = useState(null);
    const [downloadName, setDownloadName] = useState(null);
    const [myMnemonic, setMyMnemonic] = useState(null);
    const [peerMnemonic, setPeerMnemonic] = useState(null);
    const [hasAesKey, setHasAesKey] = useState(false);
    const [uploadProgress, setUploadProgress] = useState(null);
    const [downloadProgress, setDownloadProgress] = useState(null);
    const [showErrorModal, setShowErrorModal] = useState(false);

    const myKeyPairRef = useRef(null);
    const aesKeyRef = useRef(null);
    const havePeerPubRef = useRef(false);

    const wsRef = useRef(null);
    const selfIdRef = useRef(null);
    const myMnemonicRef = useRef(null);
    const clientIdRef = useRef(crypto.randomUUID());
    const roomInputRef = useRef(null);

    // For chunked file reception
    const fileChunksRef = useRef([]);
    const fileNameRef = useRef(null);
    const fileIVRef = useRef(null);
    const fileTotalSizeRef = useRef(0);
    const receivedBytesRef = useRef(0);

    function log(msg) {
        console.log(msg);
    }

    function sendMsg(obj) {
        if (!wsRef.current || wsRef.current.readyState !== 1) return;
        wsRef.current.send(JSON.stringify(obj));
    }

    async function initKeys() {
        myKeyPairRef.current = await generateECDHKeyPair();
        havePeerPubRef.current = false;
        aesKeyRef.current = null;
        setHasAesKey(false);
        log("Generated ECDH key pair");
    }

    async function announcePublicKey() {
        const raw = await exportPubKey(myKeyPairRef.current.publicKey);
        sendMsg({ type: "pubkey", pub: uint8ToBase64(raw) });
        log("Sent my public key");
    }

    async function handlePeerPubKey(b64) {
        const rawPeer = base64ToUint8(b64);
        const peerPub = await importPeerPubKey(rawPeer);
        const sharedAes = await deriveSharedKey(
            myKeyPairRef.current.privateKey,
            peerPub
        );
        aesKeyRef.current = sharedAes;
        havePeerPubRef.current = true;
        setHasAesKey(true);
        log("Derived shared AES key (E2EE ready)");
    }

    const connectToRoom = useCallback(async () => {
        const room = roomId.trim().toLowerCase();
        if (!room) {
            return;
        }
        setRoomId(room);
        await initKeys();

        // Dynamically choose ws:// or wss:// based on current page
        const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
        const host = window.location.hostname;
        // Only add port if running on localhost
        const isLocalhost = host === "localhost" || host === "127.0.0.1";
        const wsUrl = isLocalhost
            ? `${protocol}//${host}:3001/ws`
            : `${protocol}//${host}/ws`;
        const ws = new WebSocket(wsUrl);
        wsRef.current = ws;

        ws.onopen = () => {
            setConnected(true);
            setStatus("Connected. Waiting for peer...");
            log("WebSocket open");
            sendMsg({ type: "join", roomId: room, clientId: clientIdRef.current });
        };

        ws.onmessage = async (event) => {
            let msg;
            try {
                msg = JSON.parse(event.data);
            } catch {
                return;
            }

            if (msg.type === "error") {
                setShowErrorModal(true);
                setConnected(false);
                ws.close();
                return;
            }

            if (msg.type === "joined") {
                selfIdRef.current = msg.selfId;
                const mnemonic = msg.mnemonic || msg.selfId;
                myMnemonicRef.current = mnemonic;
                setMyMnemonic(mnemonic);
                log(`Joined room ${msg.roomId} as ${mnemonic}`);
                await announcePublicKey();
                return;
            }

            if (msg.type === "peers") {
                setPeerCount(msg.count);
                setStatus(
                    msg.count === 2
                        ? "Peer connected. Secure channel ready."
                        : `Connected as ${myMnemonicRef.current || "waiting..."}`
                );
                return;
            }

            if (msg.type === "pubkey") {
                const peerName = msg.mnemonic || msg.from;
                setPeerMnemonic(peerName);
                log(`Received peer public key from ${peerName}`);
                const hadPeerPub = havePeerPubRef.current;
                await handlePeerPubKey(msg.pub);
                if (!hadPeerPub) await announcePublicKey();
                return;
            }

            if (msg.type === "file_start") {
                if (!aesKeyRef.current) {
                    log("Can't decrypt yet (no shared key)");
                    return;
                }

                fileNameRef.current = msg.name;
                fileTotalSizeRef.current = msg.total_size;
                fileChunksRef.current = [];
                receivedBytesRef.current = 0;

                log(`Incoming encrypted file: ${msg.name} (${formatBytes(msg.total_size)})`);
                setDownloadProgress({ percent: 0, speed: 0, eta: 0, startTime: Date.now(), fileName: msg.name });
                return;
            }

            if (msg.type === "file_chunk") {
                try {
                    // Decrypt chunk immediately with its own IV
                    const chunkIV = base64ToUint8(msg.iv_b64);
                    const cipherChunk = base64ToUint8(msg.chunk_data);

                    const plainChunk = await decryptBytes(aesKeyRef.current, chunkIV, cipherChunk);

                    // Store decrypted chunk
                    fileChunksRef.current.push(plainChunk);
                    receivedBytesRef.current += plainChunk.length;

                    const elapsed = (Date.now() - (downloadProgress?.startTime || Date.now())) / 1000;
                    const speed = elapsed > 0 ? receivedBytesRef.current / elapsed : 0;
                    const percent = fileTotalSizeRef.current > 0
                        ? Math.round((receivedBytesRef.current / fileTotalSizeRef.current) * 100)
                        : 0;

                    const remainingBytes = fileTotalSizeRef.current - receivedBytesRef.current;
                    const eta = speed > 0 ? remainingBytes / speed : 0;

                    setDownloadProgress({
                        percent,
                        speed,
                        eta,
                        startTime: downloadProgress?.startTime || Date.now(),
                        fileName: fileNameRef.current
                    });
                } catch (err) {
                    console.error("Chunk decryption failed:", err);
                    log("Chunk decryption failed");
                }
                return;
            }

            if (msg.type === "file_end") {
                if (!aesKeyRef.current || fileChunksRef.current.length === 0) {
                    log("No file data received");
                    setDownloadProgress(null);
                    return;
                }

                try {
                    // Reassemble plaintext from decrypted chunks
                    const totalLen = fileChunksRef.current.reduce((sum, chunk) => sum + chunk.length, 0);
                    const plainBytes = new Uint8Array(totalLen);
                    let offset = 0;
                    for (const chunk of fileChunksRef.current) {
                        plainBytes.set(chunk, offset);
                        offset += chunk.length;
                    }

                    const elapsed = (Date.now() - (downloadProgress?.startTime || Date.now())) / 1000;
                    const speed = elapsed > 0 ? totalLen / elapsed : 0;

                    setDownloadProgress({ percent: 100, speed, eta: 0, fileName: fileNameRef.current });

                    const blob = new Blob([plainBytes], { type: "application/octet-stream" });
                    const url = URL.createObjectURL(blob);
                    setDownloadUrl(url);
                    setDownloadName(fileNameRef.current);

                    // auto trigger browser download
                    const a = document.createElement("a");
                    a.href = url;
                    a.download = fileNameRef.current || "download.bin";
                    document.body.appendChild(a);
                    a.click();
                    document.body.removeChild(a);

                    log(`Decrypted and prepared download "${fileNameRef.current}"`);
                } catch (err) {
                    console.error(err);
                    log("Failed to assemble file");
                    setDownloadProgress(null);
                }
                return;
            }

            if (msg.type === "file") {
                // Backward compatibility: handle old-style single-message transfers
                const { name, size, iv_b64, data_b64 } = msg;
                log(`Incoming encrypted file: ${name}`);

                if (!aesKeyRef.current) {
                    log("Can't decrypt yet (no shared key)");
                    return;
                }

                try {
                    const startTime = Date.now();
                    setDownloadProgress({ percent: 0, speed: 0, eta: 0, fileName: name });

                    const iv = base64ToUint8(iv_b64);
                    const ciphertext = base64ToUint8(data_b64);

                    setDownloadProgress({ percent: 50, speed: 0, eta: 0, fileName: name });

                    const plainBytes = await decryptBytes(aesKeyRef.current, iv, ciphertext);

                    const elapsed = (Date.now() - startTime) / 1000;
                    const speed = size / elapsed;

                    setDownloadProgress({ percent: 100, speed, eta: 0, fileName: name });

                    const blob = new Blob([plainBytes], { type: "application/octet-stream" });
                    const url = URL.createObjectURL(blob);
                    setDownloadUrl(url);
                    setDownloadName(name);

                    // auto trigger browser download
                    const a = document.createElement("a");
                    a.href = url;
                    a.download = name || "download.bin";
                    document.body.appendChild(a);
                    a.click();
                    document.body.removeChild(a);

                    log(`Decrypted and prepared download "${name}"`);
                } catch (err) {
                    console.error(err);
                    log("Decryption failed");
                    setDownloadProgress(null);
                }
                return;
            }
        };

        ws.onclose = () => {
            log("WebSocket closed");
            setConnected(false);
            setPeerCount(1);
            setStatus("Not connected");
        };

        ws.onerror = (err) => {
            console.error("WS error", err);
            log("WebSocket error");
        };
    }, [roomId]);

    async function handleFileSelect(e) {
        const file = e.target.files?.[0];
        if (!file || !aesKeyRef.current) {
            log("No file or key not ready");
            return;
        }
        try {
            const startTime = Date.now();
            setUploadProgress({ percent: 0, speed: 0, eta: 0, startTime, fileName: file.name });

            log(`Streaming file "${file.name}" (${formatBytes(file.size)})`);

            // Send file_start message (with original file size, not encrypted size)
            sendMsg({
                type: "file_start",
                name: file.name,
                total_size: file.size
            });

            // Stream file in chunks, encrypting each chunk individually
            const chunkSize = 256 * 1024;
            let sentBytes = 0;
            let chunkNum = 0;

            // Use File stream API for memory-efficient reading
            const stream = file.stream();
            const reader = stream.getReader();

            try {
                while (true) {
                    const { done, value } = await reader.read();
                    if (done) break;

                    // Encrypt this chunk with its own IV
                    const { iv, ciphertext } = await encryptBytes(aesKeyRef.current, value);

                    // Send chunk with its IV
                    sendMsg({
                        type: "file_chunk",
                        chunk_num: chunkNum,
                        chunk_data: uint8ToBase64(ciphertext),
                        iv_b64: uint8ToBase64(iv)
                    });

                    sentBytes += value.length;
                    const elapsed = (Date.now() - startTime) / 1000;
                    const speed = elapsed > 0 ? sentBytes / elapsed : 0;
                    const percent = Math.round((sentBytes / file.size) * 100);

                    const remainingBytes = file.size - sentBytes;
                    const eta = speed > 0 ? remainingBytes / speed : 0;

                    setUploadProgress({ percent, speed, eta, startTime, fileName: file.name });

                    chunkNum++;

                    // Small delay to allow UI updates
                    await new Promise(resolve => setTimeout(resolve, 10));
                }
            } finally {
                reader.releaseLock();
            }

            // Send file_end message
            sendMsg({
                type: "file_end"
            });

            const elapsed = (Date.now() - startTime) / 1000;
            const speed = file.size / elapsed;
            setUploadProgress({ percent: 100, speed, eta: 0, fileName: file.name });

            log(`Sent encrypted file "${file.name}"`);
        } catch (err) {
            console.error(err);
            log("Failed to send file");
            setUploadProgress(null);
        }
    }

    useEffect(() => {
        return () => {
            if (wsRef.current && wsRef.current.readyState === 1) {
                wsRef.current.close();
            }
        };
    }, []);

    // Auto-connect if room is in URL
    useEffect(() => {
        if (pathRoom && !connected) {
            connectToRoom();
        }
    }, [pathRoom, connected, connectToRoom]);

    // Update page title based on room
    useEffect(() => {
        document.title = roomId ? `SHARE · ${roomId.toUpperCase()}` : "SHARE";
    }, [roomId]);

    // Auto-focus room input on page load if no room in URL
    useEffect(() => {
        if (!pathRoom && !connected && roomInputRef.current) {
            roomInputRef.current.focus();
        }
    }, []);

    return (
        <div className="min-h-screen bg-white p-2 sm:p-4 md:p-8 font-mono flex items-center justify-center">
            <div className="max-w-4xl w-full">
                {/* Header */}
                <div className="bg-black text-white border-4 sm:border-8 border-black p-4 sm:p-6 mb-3 sm:mb-6" style={{ clipPath: 'polygon(0 0, calc(100% - 20px) 0, 100% 20px, 100% 100%, 0 100%)', boxShadow: '4px 4px 0px 0px rgb(229, 231, 235), 0 0 0 4px black' }}>
                    <h1 className="text-3xl sm:text-5xl md:text-6xl font-black mb-2 sm:mb-4 uppercase tracking-tight">
                        <a href="/" className="text-white no-underline cursor-pointer hover:text-white">SHARE</a>
                    </h1>
                    <p className="text-sm sm:text-lg md:text-xl font-bold leading-tight">
                        <a href="https://github.com/schollz/share" target="_blank" rel="noopener noreferrer" className="text-white no-underline cursor-pointer hover:text-white">E2EE FILE TRANSFER</a>
                    </p>
                    {myMnemonic && (
                        <div className="mt-3 sm:mt-4 flex flex-wrap items-center gap-2">
                            <div className="bg-white text-black px-2 py-1 sm:px-3 sm:py-1 inline-block border-2 sm:border-4 border-black font-black text-sm sm:text-lg uppercase">
                                {myMnemonic}
                            </div>
                            <i className="fas fa-arrows-left-right text-white text-lg sm:text-xl"></i>
                            <a href={`/${roomId}`} className="bg-white text-black px-2 py-1 sm:px-3 sm:py-1 inline-block border-2 sm:border-4 border-black font-black text-sm sm:text-lg uppercase no-underline cursor-pointer hover:bg-white">
                                {window.location.host}/{roomId}
                            </a>
                            {peerMnemonic && (
                                <>
                                    <i className="fas fa-arrows-left-right text-white text-lg sm:text-xl"></i>
                                    <div className="bg-white text-black px-2 py-1 sm:px-3 sm:py-1 inline-block border-2 sm:border-4 border-black font-black text-sm sm:text-lg uppercase">
                                        {peerMnemonic}
                                    </div>
                                </>
                            )}
                        </div>
                    )}
                </div>

                {/* Connection Panel */}
                <div className="bg-gray-200 border-4 sm:border-8 border-black p-4 sm:p-6 mb-3 sm:mb-6 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] sm:shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]">
                    {/* <h2 className="text-2xl sm:text-3xl font-black mb-3 sm:mb-4 uppercase">ROOM</h2> */}
                    <div className="flex flex-col sm:flex-row gap-3 sm:gap-4 mb-3 sm:mb-4">
                        <input
                            ref={roomInputRef}
                            type="text"
                            placeholder="ENTER ROOM ID"
                            value={roomId}
                            disabled={connected}
                            onChange={(e) => setRoomId(e.target.value)}
                            onKeyDown={(e) => e.key === "Enter" && !connected && connectToRoom()}
                            className="flex-1 border-2 sm:border-4 border-black p-3 sm:p-4 text-base sm:text-xl font-bold uppercase bg-white disabled:bg-gray-300 disabled:cursor-not-allowed focus:outline-hidden focus:ring-4 focus:ring-black"
                        />
                        <button
                            onClick={connectToRoom}
                            disabled={connected}
                            className={`border-2 sm:border-4 border-black px-6 py-3 sm:px-8 sm:py-4 text-base sm:text-xl font-black uppercase transition-all whitespace-nowrap ${connected
                                ? "bg-gray-400 cursor-not-allowed"
                                : "bg-white hover:translate-x-1 hover:translate-y-1 hover:shadow-none active:translate-x-2 active:translate-y-2"
                                } shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]`}
                        >
                            {connected ? "CONNECTED" : "CONNECT"}
                        </button>
                    </div>
                    <div className="bg-black text-white border-2 sm:border-4 border-black p-2 sm:p-3 font-bold text-sm sm:text-base md:text-lg break-words">
                        {peerMnemonic ? (
                            <>PEER: {peerMnemonic.toUpperCase()}</>
                        ) : (
                            <>STATUS: {status.toUpperCase()}</>
                        )}
                    </div>
                </div>

                {/* File Transfer Panel */}
                {connected && (
                    <div className="bg-gray-300 border-4 sm:border-8 border-black p-4 sm:p-6 mb-3 sm:mb-6 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] sm:shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]">
                        {uploadProgress && (
                            <ProgressBar progress={uploadProgress} label={`Sending ${uploadProgress.fileName}`} />
                        )}

                        {downloadProgress && (
                            <ProgressBar progress={downloadProgress} label={`Receiving ${downloadProgress.fileName}`} />
                        )}

                        <label
                            className={`block border-2 sm:border-4 border-black p-6 sm:p-8 text-center font-black uppercase ${hasAesKey
                                ? "bg-white cursor-pointer hover:translate-x-1 hover:translate-y-1 hover:shadow-none transition-all shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] text-xl sm:text-2xl"
                                : "bg-gray-400 cursor-not-allowed text-sm sm:text-base"
                                }`}
                        >
                            {hasAesKey ? "SHARE" : `WAITING FOR PEER TO JOIN ${window.location.host}/${roomId}`.toUpperCase()}
                            <input
                                type="file"
                                className="hidden"
                                onChange={handleFileSelect}
                                disabled={!hasAesKey}
                            />
                        </label>

                        {downloadUrl && (
                            <div className="mt-3 sm:mt-4 bg-white border-2 sm:border-4 border-black p-3 sm:p-4">
                                <div className="text-base sm:text-xl font-black mb-2">FILE READY:</div>
                                <a
                                    href={downloadUrl}
                                    download={downloadName}
                                    className="text-lg sm:text-2xl font-black underline hover:no-underline text-black break-all"
                                >
                                    {downloadName}
                                </a>
                            </div>
                        )}
                    </div>
                )}

            </div>

            {/* Error Modal */}
            {showErrorModal && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
                    <div className="bg-white border-4 sm:border-8 border-black p-6 sm:p-8 max-w-md w-full shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]">
                        <h2 className="text-2xl sm:text-4xl font-black mb-4 uppercase text-center">MAXIMUM ROOMS</h2>
                        <p className="text-lg sm:text-xl font-bold mb-6 text-center">TRY AGAIN LATER</p>
                        <button
                            onClick={() => {
                                setShowErrorModal(false);
                                setRoomId('');
                            }}
                            className="w-full border-2 sm:border-4 border-black bg-black text-white px-6 py-3 sm:py-4 text-lg sm:text-xl font-black uppercase hover:bg-gray-900 transition-colors"
                        >
                            OK
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}
