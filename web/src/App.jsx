import React, { useCallback, useEffect, useRef, useState } from "react";
import protobuf from "protobufjs";
import { QRCodeSVG } from "qrcode.react";
import JSZip from "jszip";

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

// Calculate SHA256 hash of data
async function calculateSHA256(data) {
    const hashBuffer = await window.crypto.subtle.digest('SHA-256', data);
    const hashArray = Array.from(new Uint8Array(hashBuffer));
    const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
    return hashHex;
}

/* ------------------- Protobuf Message Handling ------------------- */

// Protobuf schema definition
// NOTE: This schema is duplicated from src/relay/messages.proto for the web client.
// Keep in sync with the proto file or consider using a build step to import it.
const protoSchema = `
syntax = "proto3";

package relay;

message PBIncomingMessage {
  string type = 1;
  string room_id = 2;
  string client_id = 3;
  string pub = 4;
  string iv_b64 = 7;
  string data_b64 = 8;
  string chunk_data = 9;
  int32 chunk_num = 10;
  string encrypted_metadata = 20;
  string metadata_iv = 21;
}

message PBOutgoingMessage {
  string type = 1;
  string from = 2;
  string mnemonic = 3;
  string room_id = 4;
  string pub = 5;
  string iv_b64 = 8;
  string data_b64 = 9;
  string chunk_data = 10;
  int32 chunk_num = 11;
  string self_id = 13;
  repeated string peers = 14;
  int32 count = 15;
  string error = 16;
  string encrypted_metadata = 20;
  string metadata_iv = 21;
}
`;

let pbIncomingMessage, pbOutgoingMessage;

// Load protobuf schema once
const root = protobuf.parse(protoSchema).root;
pbIncomingMessage = root.lookupType("relay.PBIncomingMessage");
pbOutgoingMessage = root.lookupType("relay.PBOutgoingMessage");

// Encode message to protobuf
function encodeProtobuf(obj) {
    // Create message using protobufjs - use camelCase field names
    const pbMessage = {
        type: obj.type || ""
    };

    // protobufjs expects camelCase field names that map to snake_case in proto
    if (obj.roomId !== undefined && obj.roomId !== null && obj.roomId !== "") {
        pbMessage.roomId = obj.roomId;
    }
    if (obj.clientId !== undefined && obj.clientId !== null && obj.clientId !== "") {
        pbMessage.clientId = obj.clientId;
    }
    if (obj.pub !== undefined && obj.pub !== null && obj.pub !== "") {
        pbMessage.pub = obj.pub;
    }
    if (obj.iv_b64 !== undefined && obj.iv_b64 !== null && obj.iv_b64 !== "") {
        pbMessage.ivB64 = obj.iv_b64;
    }
    if (obj.data_b64 !== undefined && obj.data_b64 !== null && obj.data_b64 !== "") {
        pbMessage.dataB64 = obj.data_b64;
    }
    if (obj.chunk_data !== undefined && obj.chunk_data !== null && obj.chunk_data !== "") {
        pbMessage.chunkData = obj.chunk_data;
    }
    if (obj.chunk_num !== undefined && obj.chunk_num !== null) {
        pbMessage.chunkNum = obj.chunk_num;
    }
    if (obj.encrypted_metadata !== undefined && obj.encrypted_metadata !== null && obj.encrypted_metadata !== "") {
        pbMessage.encryptedMetadata = obj.encrypted_metadata;
    }
    if (obj.metadata_iv !== undefined && obj.metadata_iv !== null && obj.metadata_iv !== "") {
        pbMessage.metadataIv = obj.metadata_iv;
    }

    const message = pbIncomingMessage.create(pbMessage);
    return pbIncomingMessage.encode(message).finish();
}

// Decode protobuf message
function decodeProtobuf(buffer) {
    const message = pbOutgoingMessage.decode(buffer);
    // protobufjs provides camelCase properties for snake_case proto fields
    return {
        type: message.type,
        from: message.from,
        mnemonic: message.mnemonic,
        roomId: message.roomId,
        pub: message.pub,
        iv_b64: message.ivB64,
        data_b64: message.dataB64,
        chunk_data: message.chunkData,
        chunk_num: message.chunkNum,
        selfId: message.selfId,
        peers: message.peers || [],
        count: message.count,
        error: message.error,
        encrypted_metadata: message.encryptedMetadata || null,
        metadata_iv: message.metadataIv || null
    };
}

/* ------------------- Helper Functions ------------------- */

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round((bytes / Math.pow(k, i)) * 10) / 10 + ' ' + sizes[i];
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

const ICON_CLASSES = [
    "fa-anchor",
    "fa-ankh",
    "fa-apple-whole",
    "fa-atom",
    "fa-award",
    "fa-basketball",
    "fa-bell",
    "fa-bicycle",
    "fa-bolt",
    "fa-bomb",
    "fa-book",
    "fa-box",
    "fa-brain",
    "fa-briefcase",
    "fa-bug",
    "fa-cactus",
    "fa-cake-candles",
    "fa-calculator",
    "fa-camera",
    "fa-cannabis",
    "fa-car",
    "fa-carrot",
    "fa-cat",
    "fa-chess-knight",
    "fa-chess-rook",
    "fa-cloud",
    "fa-code",
    "fa-cog",
    "fa-compass",
    "fa-cookie",
    "fa-couch",
    "fa-crow",
    "fa-cubes",
    "fa-diamond",
    "fa-dog",
    "fa-dove",
    "fa-dragon",
    "fa-droplet",
    "fa-drum",
    "fa-earth-americas",
    "fa-egg",
    "fa-envelope",
    "fa-fan",
    "fa-feather",
    "fa-fire",
    "fa-fish",
    "fa-flag",
    "fa-flask",
    "fa-floppy-disk",
    "fa-folder",
    "fa-football",
    "fa-frog",
    "fa-gamepad",
    "fa-gavel",
    "fa-gem",
    "fa-ghost",
    "fa-gift",
    "fa-guitar",
    "fa-hammer",
    "fa-hat-cowboy",
    "fa-hat-wizard",
    "fa-heart",
    "fa-helicopter",
    "fa-helmet-safety",
    "fa-hippo",
    "fa-horse",
    "fa-hourglass-half",
    "fa-ice-cream",
    "fa-key",
    "fa-leaf",
    "fa-lightbulb",
    "fa-magnet",
    "fa-map",
    "fa-microphone",
    "fa-moon",
    "fa-mountain",
    "fa-mug-hot",
    "fa-music",
    "fa-paintbrush",
    "fa-paper-plane",
    "fa-paw",
    "fa-pen",
    "fa-pepper-hot",
    "fa-rocket",
    "fa-route",
    "fa-school",
    "fa-screwdriver-wrench",
    "fa-scroll",
    "fa-seedling",
    "fa-shield-heart",
    "fa-ship",
    "fa-skull",
    "fa-sliders",
    "fa-snowflake",
    "fa-spider",
    "fa-splotch",
    "fa-star",
    "fa-sun",
    "fa-toolbox",
    "fa-tornado",
    "fa-tree",
    "fa-trophy",
    "fa-truck",
    "fa-user-astronaut",
    "fa-water",
    "fa-wand-magic-sparkles",
    "fa-wrench",
];

function mnemonicToIcon(mnemonic) {
    if (!mnemonic) {
        return "fa-circle-question";
    }
    const words = mnemonic.toLowerCase().split(/[^a-z0-9]+/).filter(Boolean);
    for (const word of words) {
        const directMatch = ICON_CLASSES.find((icon) => icon.includes(word));
        if (directMatch) {
            return directMatch;
        }
    }
    let hash = 0;
    for (let i = 0; i < mnemonic.length; i++) {
        hash = (hash * 31 + mnemonic.charCodeAt(i)) >>> 0;
    }
    return ICON_CLASSES[hash % ICON_CLASSES.length];
}

function IconBadge({ mnemonic, label, className = "" }) {
    if (!mnemonic) {
        return null;
    }
    const iconClass = mnemonicToIcon(mnemonic);
    return (
        <div className={`relative group ${className}`}>
            <div
                tabIndex={0}
                className="bg-white text-black px-3 py-2 sm:px-4 sm:py-3 inline-flex items-center justify-center gap-2 border-2 sm:border-4 border-black font-black focus:outline-hidden"
                aria-label={`${label}: ${mnemonic}`}
            >
                <i className={`fas ${iconClass} text-xl sm:text-2xl md:text-3xl`} aria-hidden="true"></i>
                {label === "You" && <span className="text-sm sm:text-base">(YOU)</span>}
            </div>
            <div className="pointer-events-none absolute -top-2 left-1/2 -translate-x-1/2 -translate-y-full opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity duration-150 bg-black text-white border-2 border-white px-2 py-1 text-xs font-black uppercase whitespace-nowrap shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]">
                {mnemonic.toUpperCase()}
            </div>
        </div>
    );
}

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
                <div className="absolute inset-0 flex items-center justify-center text-xs sm:text-sm font-bold" style={{ mixBlendMode: 'difference', color: 'white' }}>
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
    const [showAboutModal, setShowAboutModal] = useState(false);
    const [isDragging, setIsDragging] = useState(false);

    const myKeyPairRef = useRef(null);
    const aesKeyRef = useRef(null);
    const havePeerPubRef = useRef(false);

    const wsRef = useRef(null);
    const selfIdRef = useRef(null);
    const myMnemonicRef = useRef(null);
    const clientIdRef = useRef(crypto.randomUUID());
    const roomInputRef = useRef(null);
    const fileInputRef = useRef(null);

    // For chunked file reception
    const fileChunksRef = useRef([]);
    const fileNameRef = useRef(null);
    const fileIVRef = useRef(null);
    const fileTotalSizeRef = useRef(0);
    const receivedBytesRef = useRef(0);
    const downloadStartTimeRef = useRef(null);
    const isFolderRef = useRef(false);
    const originalFolderNameRef = useRef(null);
    const expectedHashRef = useRef(null);
    
    // For chunk ordering and ACK tracking
    const receivedChunksRef = useRef(new Set());
    const chunkBufferRef = useRef(new Map());
    const nextExpectedChunkRef = useRef(0);
    const lastActivityTimeRef = useRef(Date.now());
    
    // For sending with ACK/retransmission
    const pendingChunksRef = useRef(new Map());
    const ackReceivedRef = useRef(new Set());
    const retransmitTimerRef = useRef(null);

    const myIconClass = mnemonicToIcon(myMnemonic);
    const peerIconClass = mnemonicToIcon(peerMnemonic);

    function log(msg) {
        console.log(msg);
    }

    function sendMsg(obj) {
        if (!wsRef.current || wsRef.current.readyState !== 1) return;
        // Send as protobuf binary
        const buffer = encodeProtobuf(obj);
        wsRef.current.send(buffer);
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
                // Handle both protobuf binary and JSON messages
                if (event.data instanceof Blob) {
                    // Binary protobuf message
                    const arrayBuffer = await event.data.arrayBuffer();
                    const buffer = new Uint8Array(arrayBuffer);
                    msg = decodeProtobuf(buffer);
                } else if (typeof event.data === 'string') {
                    // JSON message (fallback for compatibility)
                    msg = JSON.parse(event.data);
                } else {
                    return;
                }
            } catch (e) {
                console.error("Failed to parse message:", e);
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
            
            if (msg.type === "chunk_ack") {
                // Sender: mark chunk as acknowledged
                const chunkNum = msg.chunk_num;
                ackReceivedRef.current.add(chunkNum);
                pendingChunksRef.current.delete(chunkNum);
                lastActivityTimeRef.current = Date.now();
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

                // Decrypt metadata
                if (!msg.encrypted_metadata || !msg.metadata_iv) {
                    console.error("Missing encrypted metadata");
                    log("Missing encrypted metadata");
                    return;
                }

                let fileName, totalSize, isFolder, originalFolderName, isMultipleFiles, expectedHash;
                
                try {
                    // Decrypt metadata
                    const metadataIV = base64ToUint8(msg.metadata_iv);
                    const encryptedMetadata = base64ToUint8(msg.encrypted_metadata);
                    const metadataBytes = await decryptBytes(aesKeyRef.current, metadataIV, encryptedMetadata);
                    const metadataJSON = new TextDecoder().decode(metadataBytes);
                    const metadata = JSON.parse(metadataJSON);

                    // Use decrypted metadata
                    fileName = metadata.name;
                    totalSize = metadata.total_size;
                    isFolder = metadata.is_folder || false;
                    originalFolderName = metadata.original_folder_name || null;
                    isMultipleFiles = metadata.is_multiple_files || false;
                    expectedHash = metadata.hash || null;
                } catch (err) {
                    console.error("Failed to decrypt metadata:", err);
                    log("Failed to decrypt metadata");
                    return;
                }

                fileNameRef.current = fileName;
                fileTotalSizeRef.current = totalSize;
                fileChunksRef.current = [];
                receivedBytesRef.current = 0;
                downloadStartTimeRef.current = Date.now();
                isFolderRef.current = isFolder;
                originalFolderNameRef.current = originalFolderName;
                expectedHashRef.current = expectedHash;
                
                // Reset chunk tracking
                receivedChunksRef.current = new Set();
                chunkBufferRef.current = new Map();
                nextExpectedChunkRef.current = 0;
                lastActivityTimeRef.current = Date.now();

                const displayName = isFolderRef.current ? originalFolderNameRef.current : fileName;
                const typeLabel = isFolderRef.current ? "folder" : "file";
                log(`Incoming encrypted ${typeLabel}: ${displayName} (${formatBytes(totalSize)})`);
                setDownloadProgress({ percent: 0, speed: 0, eta: 0, startTime: downloadStartTimeRef.current, fileName: displayName });
                return;
            }

            if (msg.type === "file_chunk") {
                try {
                    const chunkNum = msg.chunk_num;
                    
                    // Check for duplicate chunk
                    if (receivedChunksRef.current.has(chunkNum)) {
                        // Send ACK again for idempotency
                        sendMsg({ type: "chunk_ack", chunk_num: chunkNum });
                        return;
                    }
                    
                    // Decrypt chunk immediately with its own IV
                    const chunkIV = base64ToUint8(msg.iv_b64);
                    const cipherChunk = base64ToUint8(msg.chunk_data);

                    const plainChunk = await decryptBytes(aesKeyRef.current, chunkIV, cipherChunk);
                    
                    // Mark as received
                    receivedChunksRef.current.add(chunkNum);
                    lastActivityTimeRef.current = Date.now();
                    
                    // Handle chunk ordering
                    if (chunkNum === nextExpectedChunkRef.current) {
                        // This is the next expected chunk - add it
                        fileChunksRef.current.push(plainChunk);
                        receivedBytesRef.current += plainChunk.length;
                        nextExpectedChunkRef.current++;
                        
                        // Check if we have buffered chunks that can now be added
                        while (chunkBufferRef.current.has(nextExpectedChunkRef.current)) {
                            const bufferedChunk = chunkBufferRef.current.get(nextExpectedChunkRef.current);
                            fileChunksRef.current.push(bufferedChunk);
                            receivedBytesRef.current += bufferedChunk.length;
                            chunkBufferRef.current.delete(nextExpectedChunkRef.current);
                            nextExpectedChunkRef.current++;
                        }
                    } else if (chunkNum > nextExpectedChunkRef.current) {
                        // Out-of-order chunk - buffer it
                        chunkBufferRef.current.set(chunkNum, plainChunk);
                    }
                    // If chunkNum < nextExpectedChunkRef.current, it's a duplicate

                    const elapsed = (Date.now() - downloadStartTimeRef.current) / 1000;
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
                        startTime: downloadStartTimeRef.current,
                        fileName: fileNameRef.current
                    });
                    
                    // Send ACK for this chunk
                    sendMsg({ type: "chunk_ack", chunk_num: chunkNum });
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

                    // Verify file hash if provided
                    if (expectedHashRef.current) {
                        const actualHash = await calculateSHA256(plainBytes);
                        if (actualHash !== expectedHashRef.current) {
                            log(`⚠️  WARNING: File hash mismatch!`);
                            log(`   Expected: ${expectedHashRef.current}`);
                            log(`   Received: ${actualHash}`);
                            log(`   The file may be corrupted or tampered with.`);
                        } else {
                            const hashPrefix = expectedHashRef.current.substring(0, 8);
                            const hashSuffix = expectedHashRef.current.substring(expectedHashRef.current.length - 8);
                            log(`✓ File integrity verified (hash: ${hashPrefix}...${hashSuffix})`);
                        }
                    }

                    const elapsed = (Date.now() - downloadStartTimeRef.current) / 1000;
                    const speed = elapsed > 0 ? totalLen / elapsed : 0;

                    // Determine download name based on whether it's a folder
                    let downloadFileName;
                    if (isFolderRef.current && originalFolderNameRef.current) {
                        downloadFileName = originalFolderNameRef.current + ".zip";
                    } else {
                        downloadFileName = fileNameRef.current || "download.bin";
                    }

                    setDownloadProgress({ percent: 100, speed, eta: 0, fileName: downloadFileName });

                    const blob = new Blob([plainBytes], { type: isFolderRef.current ? "application/zip" : "application/octet-stream" });
                    const url = URL.createObjectURL(blob);
                    setDownloadUrl(url);
                    setDownloadName(downloadFileName);

                    // auto trigger browser download
                    const a = document.createElement("a");
                    a.href = url;
                    a.download = downloadFileName;
                    document.body.appendChild(a);
                    a.click();
                    document.body.removeChild(a);

                    const typeLabel = isFolderRef.current ? "folder" : "file";
                    log(`Decrypted and prepared download "${downloadFileName}" (${typeLabel})`);
                } catch (err) {
                    console.error(err);
                    log("Failed to assemble file");
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

    // Helper function to zip a folder
    async function zipFolder(files, folderName) {
        const zip = new JSZip();
        const folder = zip.folder(folderName);

        // Add all files to the zip
        for (let i = 0; i < files.length; i++) {
            const file = files[i];
            // Get relative path (remove the folder name prefix if present)
            let relativePath = file.webkitRelativePath || file.name;

            // Remove the first folder component to get relative path within the folder
            const parts = relativePath.split('/');
            if (parts.length > 1) {
                relativePath = parts.slice(1).join('/');
            }

            folder.file(relativePath, file);
        }

        // Generate the zip blob
        const zipBlob = await zip.generateAsync({
            type: 'blob',
            compression: 'DEFLATE',
            compressionOptions: { level: 6 }
        });

        return zipBlob;
    }

    async function handleFileSelect(e) {
        const files = e.target.files;
        if (!files || files.length === 0 || !aesKeyRef.current) {
            log("No file or key not ready");
            return;
        }

        log(`Selected ${files.length} file(s)`);
        for (let i = 0; i < files.length; i++) {
            log(`  File ${i + 1}: ${files[i].name}, webkitRelativePath: ${files[i].webkitRelativePath || '(none)'}`);
        }

        try {
            let fileToSend;
            let isFolder = false;
            let isMultipleFiles = false;
            let originalFolderName = null;

            // Check if this is a folder (files with webkitRelativePath) or multiple individual files
            if (files.length > 1 || (files.length === 1 && files[0].webkitRelativePath)) {
                // Get folder name from the first file's path, or detect multiple files
                if (files[0].webkitRelativePath) {
                    // Real folder with directory structure
                    isFolder = true;
                    originalFolderName = files[0].webkitRelativePath.split('/')[0];
                } else {
                    // Multiple individual files selected (not a folder)
                    isMultipleFiles = true;
                    originalFolderName = 'files';
                }

                log(`Zipping ${isFolder ? 'folder' : 'files'} "${originalFolderName}" (${files.length} files)...`);
                const zipBlob = await zipFolder(files, originalFolderName);

                if (!zipBlob || zipBlob.size === 0) {
                    log("Error: zip file is empty");
                    setUploadProgress(null);
                    return;
                }

                fileToSend = new File([zipBlob], originalFolderName + '.zip', { type: 'application/zip' });
                log(`${isFolder ? 'Folder' : 'Files'} zipped successfully (${formatBytes(zipBlob.size)})`);
            } else {
                // Single file
                fileToSend = files[0];
            }

            const startTime = Date.now();
            const displayName = (isFolder || isMultipleFiles) ? originalFolderName : fileToSend.name;
            setUploadProgress({ percent: 0, speed: 0, eta: 0, startTime, fileName: displayName });

            const typeLabel = isFolder ? "folder" : isMultipleFiles ? "files" : "file";
            log(`Streaming ${typeLabel} "${displayName}" (${formatBytes(fileToSend.size)})`);

            // Calculate SHA256 hash of the file
            const fileBuffer = await fileToSend.arrayBuffer();
            const fileHash = await calculateSHA256(fileBuffer);
            log(`Calculated hash: ${fileHash.substring(0, 8)}...${fileHash.substring(fileHash.length - 8)}`);

            // Create metadata object
            const metadata = {
                name: fileToSend.name,
                total_size: fileToSend.size,
                hash: fileHash
            };

            if (isFolder) {
                metadata.is_folder = true;
                metadata.original_folder_name = originalFolderName;
            } else if (isMultipleFiles) {
                metadata.is_multiple_files = true;
            }

            // Encrypt metadata
            const metadataJSON = JSON.stringify(metadata);
            const metadataBytes = new TextEncoder().encode(metadataJSON);
            const { iv: metadataIV, ciphertext: encryptedMetadataBytes } = await encryptBytes(aesKeyRef.current, metadataBytes);

            // Send file_start message with encrypted metadata only
            const fileStartMsg = {
                type: "file_start",
                encrypted_metadata: uint8ToBase64(encryptedMetadataBytes),
                metadata_iv: uint8ToBase64(metadataIV)
            };

            sendMsg(fileStartMsg);
            
            // Reset ACK tracking
            pendingChunksRef.current = new Map();
            ackReceivedRef.current = new Set();
            lastActivityTimeRef.current = Date.now();
            
            // Setup retransmission logic
            const maxRetries = 3;
            const ackTimeout = 5000; // 5 seconds
            const transferTimeout = 30000; // 30 seconds
            let sendingComplete = false;
            
            // Start retransmission checker
            retransmitTimerRef.current = setInterval(() => {
                const now = Date.now();
                
                // Check for transfer timeout
                if (now - lastActivityTimeRef.current > transferTimeout) {
                    clearInterval(retransmitTimerRef.current);
                    retransmitTimerRef.current = null;
                    log("Transfer timeout: no activity for 30 seconds");
                    setUploadProgress(null);
                    return;
                }
                
                // Check pending chunks for retransmission
                for (const [chunkNum, chunkInfo] of pendingChunksRef.current.entries()) {
                    if (now - chunkInfo.sentTime > ackTimeout) {
                        if (chunkInfo.retries >= maxRetries) {
                            clearInterval(retransmitTimerRef.current);
                            retransmitTimerRef.current = null;
                            log(`Failed to send chunk ${chunkNum} after ${maxRetries} retries`);
                            setUploadProgress(null);
                            return;
                        }
                        
                        // Resend chunk
                        sendMsg({
                            type: "file_chunk",
                            chunk_num: chunkNum,
                            chunk_data: chunkInfo.chunkData,
                            iv_b64: chunkInfo.ivB64
                        });
                        chunkInfo.sentTime = now;
                        chunkInfo.retries++;
                        lastActivityTimeRef.current = now;
                    }
                }
            }, 500);

            // Stream file in chunks, encrypting each chunk individually
            const chunkSize = 256 * 1024;
            let sentBytes = 0;
            let chunkNum = 0;

            // Use File stream API for memory-efficient reading
            const stream = fileToSend.stream();
            const reader = stream.getReader();

            try {
                while (true) {
                    const { done, value } = await reader.read();
                    if (done) break;

                    // Encrypt this chunk with its own IV
                    const { iv, ciphertext } = await encryptBytes(aesKeyRef.current, value);
                    
                    const chunkData = uint8ToBase64(ciphertext);
                    const ivB64 = uint8ToBase64(iv);
                    
                    // Track this chunk for potential retransmission
                    pendingChunksRef.current.set(chunkNum, {
                        chunkData: chunkData,
                        ivB64: ivB64,
                        sentTime: Date.now(),
                        retries: 0
                    });

                    // Send chunk with its IV
                    sendMsg({
                        type: "file_chunk",
                        chunk_num: chunkNum,
                        chunk_data: chunkData,
                        iv_b64: ivB64
                    });

                    sentBytes += value.length;
                    const elapsed = (Date.now() - startTime) / 1000;
                    const speed = elapsed > 0 ? sentBytes / elapsed : 0;
                    const percent = Math.round((sentBytes / fileToSend.size) * 100);

                    const remainingBytes = fileToSend.size - sentBytes;
                    const eta = speed > 0 ? remainingBytes / speed : 0;

                    setUploadProgress({ percent, speed, eta, startTime, fileName: displayName });

                    chunkNum++;
                    lastActivityTimeRef.current = Date.now();

                    // Small delay to allow UI updates
                    await new Promise(resolve => setTimeout(resolve, 10));
                }
            } finally {
                reader.releaseLock();
            }
            
            sendingComplete = true;
            
            // Wait for all chunks to be acknowledged
            const waitStart = Date.now();
            while (pendingChunksRef.current.size > 0) {
                if (Date.now() - waitStart > 30000) {
                    clearInterval(retransmitTimerRef.current);
                    retransmitTimerRef.current = null;
                    log("Timeout waiting for chunk acknowledgments");
                    setUploadProgress(null);
                    return;
                }
                await new Promise(resolve => setTimeout(resolve, 100));
            }
            
            // Stop retransmission checker
            clearInterval(retransmitTimerRef.current);
            retransmitTimerRef.current = null;

            // Send file_end message
            sendMsg({
                type: "file_end"
            });

            const elapsed = (Date.now() - startTime) / 1000;
            const speed = fileToSend.size / elapsed;
            setUploadProgress({ percent: 100, speed, eta: 0, fileName: displayName });

            log(`Sent encrypted ${typeLabel} "${displayName}"`);
        } catch (err) {
            console.error(err);
            log("Failed to send " + (err.message || "file"));
            setUploadProgress(null);
        }
    }

    // Drag and drop handlers
    function handleDragOver(e) {
        e.preventDefault();
        e.stopPropagation();
        if (hasAesKey) {
            setIsDragging(true);
        }
    }

    function handleDragEnter(e) {
        e.preventDefault();
        e.stopPropagation();
        if (hasAesKey) {
            setIsDragging(true);
        }
    }

    function handleDragLeave(e) {
        e.preventDefault();
        e.stopPropagation();
        // Only set to false if leaving the label element itself
        if (e.currentTarget === e.target) {
            setIsDragging(false);
        }
    }

    async function handleDrop(e) {
        e.preventDefault();
        e.stopPropagation();
        setIsDragging(false);

        if (!hasAesKey) return;

        const items = e.dataTransfer?.items;
        if (!items || items.length === 0) return;

        try {
            const allFiles = [];
            let folderName = null;
            let isFolder = false;

            // Process each dropped item
            for (let i = 0; i < items.length; i++) {
                const item = items[i];
                if (item.kind === 'file') {
                    const entry = item.webkitGetAsEntry();
                    if (entry) {
                        if (entry.isDirectory) {
                            isFolder = true;
                            folderName = entry.name;
                            // Read all files from the directory
                            const dirFiles = await readDirectory(entry, entry.name);
                            allFiles.push(...dirFiles);
                        } else if (entry.isFile) {
                            const file = item.getAsFile();
                            if (file) {
                                allFiles.push(file);
                            }
                        }
                    }
                }
            }

            if (allFiles.length > 0) {
                // Create a FileList-like object with webkitRelativePath set for folder files
                const fileList = {
                    length: allFiles.length,
                    item: (index) => allFiles[index],
                    [Symbol.iterator]: function* () {
                        for (let i = 0; i < allFiles.length; i++) {
                            yield allFiles[i];
                        }
                    }
                };

                // Add indexed properties
                for (let i = 0; i < allFiles.length; i++) {
                    fileList[i] = allFiles[i];
                }

                const syntheticEvent = {
                    target: {
                        files: fileList
                    }
                };
                handleFileSelect(syntheticEvent);
            }
        } catch (err) {
            console.error("Error processing dropped items:", err);
            log("Failed to process dropped items");
        }
    }

    // Helper function to recursively read directory contents
    async function readDirectory(dirEntry, basePath = '') {
        const files = [];
        const reader = dirEntry.createReader();

        return new Promise((resolve, reject) => {
            const readEntries = () => {
                reader.readEntries(async (entries) => {
                    if (entries.length === 0) {
                        resolve(files);
                        return;
                    }

                    for (const entry of entries) {
                        if (entry.isFile) {
                            const file = await new Promise((res, rej) => {
                                entry.file((f) => {
                                    // Create a new File object with webkitRelativePath set
                                    const path = basePath ? `${basePath}/${f.name}` : f.name;
                                    const newFile = new File([f], f.name, { type: f.type, lastModified: f.lastModified });
                                    Object.defineProperty(newFile, 'webkitRelativePath', {
                                        value: path,
                                        writable: false
                                    });
                                    res(newFile);
                                }, rej);
                            });
                            files.push(file);
                        } else if (entry.isDirectory) {
                            const subPath = basePath ? `${basePath}/${entry.name}` : entry.name;
                            const subFiles = await readDirectory(entry, subPath);
                            files.push(...subFiles);
                        }
                    }

                    // Continue reading (directories may have many entries)
                    readEntries();
                }, reject);
            };

            readEntries();
        });
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
        <div className="min-h-screen bg-white p-2 sm:p-4 md:p-8 font-mono flex flex-col items-center justify-center">
            <div className="max-w-4xl w-full flex-grow flex flex-col justify-center">
                {/* Header */}
                <div className="bg-black text-white border-4 sm:border-8 border-black p-4 sm:p-6 mb-3 sm:mb-6 flex items-start justify-between gap-4" style={{ clipPath: 'polygon(0 0, calc(100% - 20px) 0, 100% 20px, 100% 100%, 0 100%)', boxShadow: '4px 4px 0px 0px rgb(229, 231, 235), 0 0 0 4px black' }}>
                    <div className="flex-1">
                        <h1 className="text-3xl sm:text-5xl md:text-6xl font-black mb-2 sm:mb-4 uppercase tracking-tight">
                            <a href="/" className="text-white no-underline cursor-pointer hover:text-white hover:underline">SHARE</a>
                        </h1>
                        <p className="flex items-center gap-2 text-sm sm:text-lg md:text-xl font-bold leading-tight">
                            <a href="https://github.com/schollz/share" target="_blank" rel="noopener noreferrer" className="text-white no-underline cursor-pointer hover:text-white hover:underline">E2EE FILE TRANSFER</a>
                            <button
                                type="button"
                                onClick={() => setShowAboutModal(true)}
                                className="inline-flex h-7 w-7 sm:h-8 sm:w-8 items-center justify-center rounded-full border-2 border-white text-white hover:bg-white hover:text-black transition-colors cursor-pointer"
                                aria-label="About Share"
                            >
                                ?
                            </button>
                        </p>
                        {myMnemonic && (
                            <div className="mt-3 sm:mt-4 flex flex-wrap items-center gap-2">
                                <IconBadge mnemonic={myMnemonic} label="You" className="shrink-0" />
                                <i className="fas fa-arrows-left-right text-white text-lg sm:text-xl"></i>
                                <a
                                    href={`/${roomId}`}
                                    className="bg-white text-black px-2 py-1 sm:px-3 sm:py-1 inline-flex items-center justify-center border-2 sm:border-4 border-black font-black text-sm sm:text-lg uppercase no-underline cursor-pointer hover:bg-white"
                                >
                                    {roomId ? roomId.toUpperCase() : "ROOM"}
                                    <span className="sr-only">Link to {window.location.host}/{roomId}</span>
                                </a>
                                {peerMnemonic && (
                                    <>
                                        <i className="fas fa-arrows-left-right text-white text-lg sm:text-xl"></i>
                                        <IconBadge mnemonic={peerMnemonic} label="Peer" className="shrink-0" />
                                    </>
                                )}
                            </div>
                        )}
                    </div>
                    {myMnemonic && !peerMnemonic && roomId && (
                        <div className="flex-shrink-0 ml-auto">
                            <QRCodeSVG
                                value={`${window.location.origin}/${roomId}`}
                                size={140}
                                level="M"
                                fgColor="#ffffff"
                                bgColor="#000000"
                            />
                        </div>
                    )}
                </div>

                {/* Connection Panel */}
                {!connected && (
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
                                    : "bg-white hover:translate-x-1 hover:translate-y-1 hover:shadow-none active:translate-x-2 active:translate-y-2 cursor-pointer"
                                    } shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]`}
                            >
                                {connected ? "CONNECTED" : "CONNECT"}
                            </button>
                        </div>
                        <div className="bg-black text-white border-2 sm:border-4 border-black p-2 sm:p-3 font-bold text-sm sm:text-base md:text-lg break-words">
                            {connected && myMnemonic ? (
                                peerMnemonic ? (
                                    <>
                                        CONNECTED AS {myMnemonic.toUpperCase()} (
                                        <span className="inline-flex items-center ml-1" title={`Your icon for ${myMnemonic}`}>
                                            <i className={`fas ${myIconClass}`} aria-hidden="true"></i>
                                        </span>
                                        ) TO {peerMnemonic.toUpperCase()} (
                                        <span className="inline-flex items-center ml-1" title={`Peer icon for ${peerMnemonic}`}>
                                            <i className={`fas ${peerIconClass}`} aria-hidden="true"></i>
                                        </span>
                                        )
                                    </>
                                ) : (
                                    <>
                                        CONNECTED AS {myMnemonic.toUpperCase()} (
                                        <span className="inline-flex items-center ml-1" title={`Your icon for ${myMnemonic}`}>
                                            <i className={`fas ${myIconClass}`} aria-hidden="true"></i>
                                        </span>
                                        )
                                    </>
                                )
                            ) : (
                                <>STATUS: {status.toUpperCase()}</>
                            )}
                        </div>
                    </div>
                )}

                {/* File Transfer Panel */}
                {connected && (
                    <div className="bg-gray-300 border-4 sm:border-8 border-black p-4 sm:p-6 mb-3 sm:mb-6 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] sm:shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]">
                        {uploadProgress && (
                            <ProgressBar progress={uploadProgress} label={`Sending ${uploadProgress.fileName}`} />
                        )}

                        {downloadProgress && (
                            <ProgressBar progress={downloadProgress} label={`Receiving ${downloadProgress.fileName}`} />
                        )}

                        <div
                            className={`border-2 sm:border-4 border-black p-6 sm:p-8 text-center transition-all ${hasAesKey
                                ? isDragging
                                    ? "bg-yellow-300 shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] scale-105"
                                    : "bg-white shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]"
                                : "bg-gray-400"
                                }`}
                            onDragOver={handleDragOver}
                            onDragEnter={handleDragEnter}
                            onDragLeave={handleDragLeave}
                            onDrop={handleDrop}
                        >
                            {hasAesKey ? (
                                isDragging ? (
                                    <div className="font-black uppercase text-xl sm:text-2xl">
                                        📁 DROP FILES OR FOLDER HERE
                                    </div>
                                ) : (
                                    <>
                                        <input
                                            ref={fileInputRef}
                                            type="file"
                                            className="hidden"
                                            onChange={handleFileSelect}
                                            disabled={!hasAesKey}
                                            multiple
                                        />
                                        <button
                                            onClick={() => fileInputRef.current?.click()}
                                            disabled={!hasAesKey}
                                            className="block w-full border-2 border-black p-4 font-black uppercase cursor-pointer hover:bg-gray-100 transition-colors text-base sm:text-lg disabled:cursor-not-allowed disabled:bg-gray-400"
                                        >
                                            SHARE
                                        </button>
                                    </>
                                )
                            ) : (
                                <div className="font-black uppercase text-sm sm:text-base text-gray-600">
                                    {`WAITING FOR PEER TO JOIN ${window.location.host}/${roomId}`.toUpperCase()}
                                </div>
                            )}
                        </div>

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
                            className="w-full border-2 sm:border-4 border-black bg-black text-white px-6 py-3 sm:py-4 text-lg sm:text-xl font-black uppercase hover:bg-gray-900 transition-colors cursor-pointer"
                        >
                            OK
                        </button>
                    </div>
                </div>
            )}

            {/* About Modal */}
            {showAboutModal && (
                <div
                    className="fixed inset-0 bg-[rgba(15,15,15,0.7)] flex items-center justify-center z-50 p-4"
                    onClick={() => setShowAboutModal(false)}
                >
                    <div
                        className="bg-white border-4 sm:border-8 border-black p-6 sm:p-8 max-w-md sm:max-w-lg w-full text-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]"
                        onClick={(e) => e.stopPropagation()}
                    >
                        <h2 className="text-2xl sm:text-3xl font-black uppercase mb-3 text-center">WHAT IS SHARE?</h2>
                        <p className="text-sm sm:text-base font-bold mb-3 text-center">
                            Share allows two computers to share files with end-to-end encryption via a zero-knowledge relay.
                        </p>
                        <p className="text-sm sm:text-base font-bold mb-4 text-center">
                            Use the CLI to swap files between web or terminals:
                            <br />
                            <code>curl https://share.schollz.com | bash</code>
                        </p>
                        <button
                            type="button"
                            onClick={() => setShowAboutModal(false)}
                            className="w-full border-2 sm:border-4 border-black bg-black text-white px-4 py-2 sm:py-3 text-sm sm:text-lg font-black uppercase hover:bg-gray-900 transition-colors cursor-pointer"
                        >
                            Close
                        </button>
                    </div>
                </div>
            )}

        </div>
    );
}
