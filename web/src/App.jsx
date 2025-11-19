import React, { useCallback, useEffect, useRef, useState } from "react";
import protobuf from "protobufjs";
import { QRCodeSVG } from "qrcode.react";
import JSZip from "jszip";
import toast, { Toaster } from "react-hot-toast";

/* ---------- Crypto helpers (ECDH + AES-GCM) ---------- */

// Generate ephemeral ECDH key pair
async function generateECDHKeyPair() {
    return await window.crypto.subtle.generateKey(
        { name: "ECDH", namedCurve: "P-256" },
        true,
        ["deriveKey"],
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
        [],
    );
}

// Derive AES key from shared secret
async function deriveSharedKey(privKey, peerPubKey) {
    return await window.crypto.subtle.deriveKey(
        { name: "ECDH", public: peerPubKey },
        privKey,
        { name: "AES-GCM", length: 256 },
        false,
        ["encrypt", "decrypt"],
    );
}

// AES-GCM encrypt/decrypt
async function encryptBytes(aesKey, plainBuffer) {
    const iv = window.crypto.getRandomValues(new Uint8Array(12));
    const ciphertext = await window.crypto.subtle.encrypt(
        { name: "AES-GCM", iv },
        aesKey,
        plainBuffer,
    );
    return { iv, ciphertext: new Uint8Array(ciphertext) };
}

async function decryptBytes(aesKey, ivBytes, cipherBytes) {
    const plain = await window.crypto.subtle.decrypt(
        { name: "AES-GCM", iv: ivBytes },
        aesKey,
        cipherBytes,
    );
    return new Uint8Array(plain);
}

// Helpers: base64 encode/decode
function uint8ToBase64(u8) {
    let binary = "";
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
    const hashBuffer = await window.crypto.subtle.digest("SHA-256", data);
    const hashArray = Array.from(new Uint8Array(hashBuffer));
    const hashHex = hashArray
        .map((b) => b.toString(16).padStart(2, "0"))
        .join("");
    return hashHex;
}

/* ------------------- Protobuf Message Handling ------------------- */

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
  string peer_id = 22;
}
`;

let pbIncomingMessage, pbOutgoingMessage;

const root = protobuf.parse(protoSchema).root;
pbIncomingMessage = root.lookupType("relay.PBIncomingMessage");
pbOutgoingMessage = root.lookupType("relay.PBOutgoingMessage");

function encodeProtobuf(obj) {
    const pbMessage = { type: obj.type || "" };

    if (obj.roomId !== undefined && obj.roomId !== null && obj.roomId !== "") pbMessage.roomId = obj.roomId;
    if (obj.clientId !== undefined && obj.clientId !== null && obj.clientId !== "") pbMessage.clientId = obj.clientId;
    if (obj.pub !== undefined && obj.pub !== null && obj.pub !== "") pbMessage.pub = obj.pub;
    if (obj.iv_b64 !== undefined && obj.iv_b64 !== null && obj.iv_b64 !== "") pbMessage.ivB64 = obj.iv_b64;
    if (obj.data_b64 !== undefined && obj.data_b64 !== null && obj.data_b64 !== "") pbMessage.dataB64 = obj.data_b64;
    if (obj.chunk_data !== undefined && obj.chunk_data !== null && obj.chunk_data !== "") pbMessage.chunkData = obj.chunk_data;
    if (obj.chunk_num !== undefined && obj.chunk_num !== null) pbMessage.chunkNum = obj.chunk_num;
    if (obj.encrypted_metadata !== undefined && obj.encrypted_metadata !== null && obj.encrypted_metadata !== "") pbMessage.encryptedMetadata = obj.encrypted_metadata;
    if (obj.metadata_iv !== undefined && obj.metadata_iv !== null && obj.metadata_iv !== "") pbMessage.metadataIv = obj.metadata_iv;

    const message = pbIncomingMessage.create(pbMessage);
    return pbIncomingMessage.encode(message).finish();
}

function decodeProtobuf(buffer) {
    const message = pbOutgoingMessage.decode(buffer);
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
        metadata_iv: message.metadataIv || null,
        peerId: message.peerId || null,
    };
}

/* ------------------- Helper Functions ------------------- */

function formatBytes(bytes) {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round((bytes / Math.pow(k, i)) * 10) / 10 + " " + sizes[i];
}

function formatSpeed(bytesPerSecond) {
    return formatBytes(bytesPerSecond) + "/s";
}

function formatTime(seconds) {
    if (seconds < 1) return "< 1s";
    if (seconds < 60) return Math.round(seconds) + "s";
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
    "fa-cake-candles",
    "fa-calculator",
    "fa-camera",
    "fa-campground",
    "fa-car",
    "fa-carrot",
    "fa-cat",
    "fa-chess-knight",
    "fa-chess-rook",
    "fa-cloud",
    "fa-code",
    "fa-gear",
    "fa-compass",
    "fa-cookie",
    "fa-crow",
    "fa-cube",
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
    "fa-snowflake",
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
    "fa-road",
    "fa-school",
    "fa-screwdriver-wrench",
    "fa-scroll",
    "fa-seedling",
    "fa-shield-heart",
    "fa-ship",
    "fa-skull",
    "fa-sliders",
    "fa-splotch",
    "fa-spider",
    "fa-star",
    "fa-sun",
    "fa-toolbox",
    "fa-tornado",
    "fa-tree",
    "fa-trophy",
    "fa-truck",
    "fa-user-astronaut",
    "fa-wand-magic-sparkles",
    "fa-wrench",
    "fa-pizza-slice",
    "fa-burger",
    "fa-lemon",
];

function mnemonicToIcons(mnemonic) {
    if (!mnemonic) {
        return ["fa-circle-question", "fa-circle-question", "fa-circle-question"];
    }

    const words = mnemonic.toLowerCase().split(/[^a-z0-9]+/).filter(Boolean);
    const icons = [];

    for (const word of words) {
        const directMatch = ICON_CLASSES.find((icon) => icon.includes(word));
        if (directMatch) {
            icons.push(directMatch);
        } else {
            let hash = 0;
            for (let i = 0; i < word.length; i++) {
                hash = (hash * 31 + word.charCodeAt(i)) >>> 0;
            }
            icons.push(ICON_CLASSES[hash % ICON_CLASSES.length]);
        }
    }

    while (icons.length < 3) {
        icons.push("fa-circle-question");
    }

    return icons.slice(0, 3);
}

function IconBadge({ mnemonic, label, className = "" }) {
    if (!mnemonic) return null;
    const iconClasses = mnemonicToIcons(mnemonic);

    return (
        <div className={`tooltip tooltip-bottom ${className}`} data-tip={mnemonic.toUpperCase()}>
            <div className="badge badge-lg gap-2 px-4 py-3 shadow-md">
                {iconClasses.map((iconClass, index) => (
                    <i key={index} className={`fas ${iconClass} text-lg`} aria-hidden="true"></i>
                ))}
                {label === "You" && <span className="text-xs ml-1 font-bold">(YOU)</span>}
            </div>
        </div>
    );
}

function ProgressBar({ progress, label }) {
    if (!progress) return null;

    const cleanLabel = label.replace(/[\u{1F300}-\u{1F9FF}]/gu, "").trim();

    return (
        <div className="card bg-base-200 shadow-md mb-4">
            <div className="card-body p-4">
                <h3 className="card-title text-sm">{cleanLabel}</h3>
                <progress
                    className="progress progress-primary w-full"
                    value={progress.percent}
                    max="100"
                ></progress>
                <div className="flex justify-between text-xs opacity-70">
                    <span>{progress.percent}%</span>
                    {progress.speed > 0 && <span>{formatSpeed(progress.speed)}</span>}
                    {progress.eta > 0 && progress.percent < 100 && <span>ETA: {formatTime(progress.eta)}</span>}
                </div>
            </div>
        </div>
    );
}

export default function App() {
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
    const [showDownloadConfirmModal, setShowDownloadConfirmModal] = useState(false);
    const [pendingDownload, setPendingDownload] = useState(null);
    const [isDragging, setIsDragging] = useState(false);
    const [roomIdError, setRoomIdError] = useState(null);
    const [darkMode, setDarkMode] = useState(false);
    const [textInput, setTextInput] = useState("");
    const [receivedText, setReceivedText] = useState(null);
    const [showTextModal, setShowTextModal] = useState(false);

    const myKeyPairRef = useRef(null);
    const aesKeyRef = useRef(null);
    const havePeerPubRef = useRef(false);
    const wsRef = useRef(null);
    const selfIdRef = useRef(null);
    const myMnemonicRef = useRef(null);
    const clientIdRef = useRef(crypto.randomUUID());
    const roomInputRef = useRef(null);
    const fileInputRef = useRef(null);
    const fileChunksRef = useRef([]);
    const fileNameRef = useRef(null);
    const fileIVRef = useRef(null);
    const fileTotalSizeRef = useRef(0);
    const receivedBytesRef = useRef(0);
    const downloadStartTimeRef = useRef(null);
    const isFolderRef = useRef(false);
    const originalFolderNameRef = useRef(null);
    const expectedHashRef = useRef(null);
    const receivedChunksRef = useRef(new Set());
    const chunkBufferRef = useRef(new Map());
    const nextExpectedChunkRef = useRef(0);
    const lastActivityTimeRef = useRef(Date.now());
    const pendingChunksRef = useRef(new Map());
    const ackReceivedRef = useRef(new Set());
    const retransmitTimerRef = useRef(null);

    const myIconClasses = mnemonicToIcons(myMnemonic);
    const peerIconClasses = mnemonicToIcons(peerMnemonic);

    function log(msg) {
        console.log(msg);
    }

    function sendMsg(obj) {
        if (!wsRef.current || wsRef.current.readyState !== 1) return;
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
        const sharedAes = await deriveSharedKey(myKeyPairRef.current.privateKey, peerPub);
        aesKeyRef.current = sharedAes;
        havePeerPubRef.current = true;
        setHasAesKey(true);
        log("Derived shared AES key (E2EE ready)");
    }

    function generateRandomRoomId() {
        const adjectives = ["swift", "bold", "calm", "bright", "dark", "warm", "cool", "wise", "neat", "wild"];
        const nouns = ["tiger", "eagle", "ocean", "mountain", "forest", "river", "storm", "star", "moon", "sun"];
        const randomAdjective = adjectives[Math.floor(Math.random() * adjectives.length)];
        const randomNoun = nouns[Math.floor(Math.random() * nouns.length)];
        const randomNumber = Math.floor(Math.random() * 100);
        return `${randomAdjective}-${randomNoun}-${randomNumber}`;
    }

    const connectToRoom = useCallback(async () => {
        let room = roomId.trim().toLowerCase();

        if (!room) {
            room = generateRandomRoomId();
            setRoomId(room);
        }

        if (room.length < 4) {
            setRoomIdError("Room ID must be at least 4 characters. Please enter a longer room name.");
            return;
        }

        setRoomIdError(null);
        setRoomId(room);
        await initKeys();

        const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
        const host = window.location.host;
        const wsUrl = `${protocol}//${host}/ws`;
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
                if (event.data instanceof Blob) {
                    const arrayBuffer = await event.data.arrayBuffer();
                    const buffer = new Uint8Array(arrayBuffer);
                    msg = decodeProtobuf(buffer);
                } else if (typeof event.data === "string") {
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
                setStatus(msg.count === 2 ? "Peer connected. Secure channel ready." : `Connected as ${myMnemonicRef.current || "waiting..."}`);
                return;
            }

            if (msg.type === "chunk_ack") {
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

                if (hadPeerPub) return;
                havePeerPubRef.current = true;

                const peerIcons = mnemonicToIcons(peerName);
                toast.success(
                    <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                        {peerIcons.map((iconClass, index) => (
                            <i key={index} className={`fas ${iconClass}`} aria-hidden="true" style={{ fontSize: "16px" }}></i>
                        ))}
                        <span>{peerName.toUpperCase()} Connected</span>
                    </div>,
                    { duration: 3000 }
                );

                await handlePeerPubKey(msg.pub);
                await announcePublicKey();
                return;
            }

            if (msg.type === "peer_disconnected") {
                const disconnectedPeerName = msg.mnemonic || msg.peerId || "Peer";
                log(`${disconnectedPeerName} disconnected`);

                const peerIcons = mnemonicToIcons(disconnectedPeerName);
                toast.error(
                    <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                        {peerIcons.map((iconClass, index) => (
                            <i key={index} className={`fas ${iconClass}`} aria-hidden="true" style={{ fontSize: "16px" }}></i>
                        ))}
                        <span>{disconnectedPeerName.toUpperCase()} Disconnected</span>
                    </div>,
                    { duration: 4000 }
                );

                setPeerMnemonic(null);
                havePeerPubRef.current = false;
                aesKeyRef.current = null;
                setHasAesKey(false);

                if (wsRef.current) {
                    wsRef.current.close();
                }

                setTimeout(() => {
                    log(`Rejoining room ${roomId}`);
                    connectToRoom();
                }, 500);

                return;
            }

            if (msg.type === "transfer_received") {
                const receiverName = msg.mnemonic || msg.from || "Receiver";
                let transferType = "file";

                if (msg.encrypted_metadata && msg.metadata_iv && aesKeyRef.current) {
                    try {
                        const metadataIV = base64ToUint8(msg.metadata_iv);
                        const encryptedMetadata = base64ToUint8(msg.encrypted_metadata);
                        const metadataBytes = await decryptBytes(aesKeyRef.current, metadataIV, encryptedMetadata);
                        const metadataJSON = new TextDecoder().decode(metadataBytes);
                        const metadata = JSON.parse(metadataJSON);

                        if (metadata.transfer_type) {
                            transferType = metadata.transfer_type;
                        }
                    } catch (err) {
                        console.error("Failed to decrypt transfer_received metadata:", err);
                    }
                }

                const typeLabel = transferType === "text" ? "Text" : "File";
                log(`${receiverName} confirmed receipt of the ${typeLabel.toLowerCase()}`);

                const receiverIcons = mnemonicToIcons(receiverName);
                toast.success(
                    <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                        {receiverIcons.map((iconClass, index) => (
                            <i key={index} className={`fas ${iconClass}`} aria-hidden="true" style={{ fontSize: "16px" }}></i>
                        ))}
                        <span>{receiverName.toUpperCase()} Received {typeLabel}</span>
                    </div>,
                    { duration: 4000 }
                );

                return;
            }

            if (msg.type === "file_start") {
                if (!aesKeyRef.current) {
                    log("Can't decrypt yet (no shared key)");
                    return;
                }

                if (!msg.encrypted_metadata || !msg.metadata_iv) {
                    console.error("Missing encrypted metadata");
                    log("Missing encrypted metadata");
                    return;
                }

                let fileName, totalSize, isFolder, originalFolderName, isMultipleFiles, expectedHash;

                try {
                    const metadataIV = base64ToUint8(msg.metadata_iv);
                    const encryptedMetadata = base64ToUint8(msg.encrypted_metadata);
                    const metadataBytes = await decryptBytes(aesKeyRef.current, metadataIV, encryptedMetadata);
                    const metadataJSON = new TextDecoder().decode(metadataBytes);
                    const metadata = JSON.parse(metadataJSON);

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

                receivedChunksRef.current = new Set();
                chunkBufferRef.current = new Map();
                nextExpectedChunkRef.current = 0;
                lastActivityTimeRef.current = Date.now();

                const displayName = isFolderRef.current ? originalFolderNameRef.current : fileName;
                const typeLabel = isFolderRef.current ? "folder" : "file";
                log(`Incoming encrypted ${typeLabel}: ${displayName} (${formatBytes(totalSize)})`);
                setDownloadProgress({
                    percent: 0,
                    speed: 0,
                    eta: 0,
                    startTime: downloadStartTimeRef.current,
                    fileName: displayName,
                });
                return;
            }

            if (msg.type === "file_chunk") {
                try {
                    const chunkNum = msg.chunk_num;

                    if (receivedChunksRef.current.has(chunkNum)) {
                        sendMsg({ type: "chunk_ack", chunk_num: chunkNum });
                        return;
                    }

                    const chunkIV = base64ToUint8(msg.iv_b64);
                    const cipherChunk = base64ToUint8(msg.chunk_data);
                    const plainChunk = await decryptBytes(aesKeyRef.current, chunkIV, cipherChunk);

                    receivedChunksRef.current.add(chunkNum);
                    lastActivityTimeRef.current = Date.now();

                    if (chunkNum === nextExpectedChunkRef.current) {
                        fileChunksRef.current.push(plainChunk);
                        receivedBytesRef.current += plainChunk.length;
                        nextExpectedChunkRef.current++;

                        while (chunkBufferRef.current.has(nextExpectedChunkRef.current)) {
                            const bufferedChunk = chunkBufferRef.current.get(nextExpectedChunkRef.current);
                            fileChunksRef.current.push(bufferedChunk);
                            receivedBytesRef.current += bufferedChunk.length;
                            chunkBufferRef.current.delete(nextExpectedChunkRef.current);
                            nextExpectedChunkRef.current++;
                        }
                    } else if (chunkNum > nextExpectedChunkRef.current) {
                        chunkBufferRef.current.set(chunkNum, plainChunk);
                    }

                    const elapsed = (Date.now() - downloadStartTimeRef.current) / 1000;
                    const speed = elapsed > 0 ? receivedBytesRef.current / elapsed : 0;
                    const percent = fileTotalSizeRef.current > 0 ? Math.round((receivedBytesRef.current / fileTotalSizeRef.current) * 100) : 0;
                    const remainingBytes = fileTotalSizeRef.current - receivedBytesRef.current;
                    const eta = speed > 0 ? remainingBytes / speed : 0;

                    setDownloadProgress({ percent, speed, eta, startTime: downloadStartTimeRef.current, fileName: fileNameRef.current });
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
                    const totalLen = fileChunksRef.current.reduce((sum, chunk) => sum + chunk.length, 0);
                    const plainBytes = new Uint8Array(totalLen);
                    let offset = 0;
                    for (const chunk of fileChunksRef.current) {
                        plainBytes.set(chunk, offset);
                        offset += chunk.length;
                    }

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

                    let downloadFileName;
                    if (isFolderRef.current && originalFolderNameRef.current) {
                        downloadFileName = originalFolderNameRef.current + ".zip";
                    } else {
                        downloadFileName = fileNameRef.current || "download.bin";
                    }

                    setDownloadProgress({ percent: 100, speed, eta: 0, fileName: downloadFileName });

                    const blob = new Blob([plainBytes], {
                        type: isFolderRef.current ? "application/zip" : "application/octet-stream",
                    });
                    const url = URL.createObjectURL(blob);
                    const typeLabel = isFolderRef.current ? "folder" : "file";

                    setPendingDownload({
                        url: url,
                        name: downloadFileName,
                        size: formatBytes(totalLen),
                        type: typeLabel,
                    });
                    setShowDownloadConfirmModal(true);

                    log(`Decrypted and prepared download "${downloadFileName}" (${typeLabel})`);

                    try {
                        const transferMetadata = { transfer_type: "file" };
                        const metadataJSON = JSON.stringify(transferMetadata);
                        const metadataBytes = new TextEncoder().encode(metadataJSON);
                        const { iv: metadataIV, ciphertext: encryptedMetadataBytes } = await encryptBytes(aesKeyRef.current, metadataBytes);

                        sendMsg({
                            type: "transfer_received",
                            encrypted_metadata: uint8ToBase64(encryptedMetadataBytes),
                            metadata_iv: uint8ToBase64(metadataIV),
                        });
                    } catch (err) {
                        console.error("Failed to encrypt transfer_received metadata:", err);
                        sendMsg({ type: "transfer_received" });
                    }
                } catch (err) {
                    console.error(err);
                    log("Failed to assemble file");
                    setDownloadProgress(null);
                }
                return;
            }

            if (msg.type === "text_message") {
                if (!aesKeyRef.current) {
                    log("Can't decrypt text yet (no shared key)");
                    return;
                }

                if (!msg.encrypted_metadata || !msg.metadata_iv) {
                    console.error("Missing encrypted metadata for text message");
                    log("Missing encrypted metadata for text message");
                    return;
                }

                try {
                    const metadataIV = base64ToUint8(msg.metadata_iv);
                    const encryptedMetadata = base64ToUint8(msg.encrypted_metadata);
                    const metadataBytes = await decryptBytes(aesKeyRef.current, metadataIV, encryptedMetadata);
                    const metadataJSON = new TextDecoder().decode(metadataBytes);
                    const metadata = JSON.parse(metadataJSON);

                    if (metadata.is_text && metadata.text) {
                        setReceivedText(metadata.text);
                        setShowTextModal(true);
                        log("Received text message");

                        try {
                            const transferMetadata = { transfer_type: "text" };
                            const metadataJSON = JSON.stringify(transferMetadata);
                            const metadataBytes = new TextEncoder().encode(metadataJSON);
                            const { iv: metadataIV, ciphertext: encryptedMetadataBytes } = await encryptBytes(aesKeyRef.current, metadataBytes);

                            sendMsg({
                                type: "transfer_received",
                                encrypted_metadata: uint8ToBase64(encryptedMetadataBytes),
                                metadata_iv: uint8ToBase64(metadataIV),
                            });
                        } catch (err) {
                            console.error("Failed to encrypt transfer_received metadata:", err);
                            sendMsg({ type: "transfer_received" });
                        }
                    }
                } catch (err) {
                    console.error("Failed to decrypt text message:", err);
                    log("Failed to decrypt text message");
                }
                return;
            }
        };

        ws.onclose = () => {
            log("WebSocket closed");
            setConnected(false);
            setPeerCount(1);
            setStatus("Not connected");

            if (roomId) {
                log(`Connection lost, attempting to reconnect to room ${roomId}`);
                setPeerMnemonic(null);
                havePeerPubRef.current = false;
                aesKeyRef.current = null;
                setHasAesKey(false);

                setTimeout(() => {
                    log(`Reconnecting to room ${roomId}`);
                    connectToRoom();
                }, 1000);
            }
        };

        ws.onerror = (err) => {
            console.error("WS error", err);
            log("WebSocket error");
        };
    }, [roomId]);

    function handleConnect() {
        let room = roomId.trim().toLowerCase();

        if (!room) {
            room = generateRandomRoomId();
        }

        if (room.length < 4) {
            setRoomIdError("Room ID must be at least 4 characters. Please enter a longer room name.");
            return;
        }

        setRoomIdError(null);
        window.location.href = `/${room}`;
    }

    async function zipFolder(files, folderName) {
        const zip = new JSZip();
        const folder = zip.folder(folderName);

        for (let i = 0; i < files.length; i++) {
            const file = files[i];
            let relativePath = file.webkitRelativePath || file.name;

            const parts = relativePath.split("/");
            if (parts.length > 1) {
                relativePath = parts.slice(1).join("/");
            }

            folder.file(relativePath, file);
        }

        const zipBlob = await zip.generateAsync({
            type: "blob",
            compression: "DEFLATE",
            compressionOptions: { level: 6 },
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
            log(`  File ${i + 1}: ${files[i].name}, webkitRelativePath: ${files[i].webkitRelativePath || "(none)"}`);
        }

        try {
            let fileToSend;
            let isFolder = false;
            let isMultipleFiles = false;
            let originalFolderName = null;

            if (files.length > 1 || (files.length === 1 && files[0].webkitRelativePath)) {
                if (files[0].webkitRelativePath) {
                    isFolder = true;
                    originalFolderName = files[0].webkitRelativePath.split("/")[0];
                } else {
                    isMultipleFiles = true;
                    originalFolderName = "files";
                }

                log(`Zipping ${isFolder ? "folder" : "files"} "${originalFolderName}" (${files.length} files)...`);
                const zipBlob = await zipFolder(files, originalFolderName);

                if (!zipBlob || zipBlob.size === 0) {
                    log("Error: zip file is empty");
                    setUploadProgress(null);
                    return;
                }

                fileToSend = new File([zipBlob], originalFolderName + ".zip", { type: "application/zip" });
                log(`${isFolder ? "Folder" : "Files"} zipped successfully (${formatBytes(zipBlob.size)})`);
            } else {
                fileToSend = files[0];
            }

            const startTime = Date.now();
            const displayName = isFolder || isMultipleFiles ? originalFolderName : fileToSend.name;
            setUploadProgress({ percent: 0, speed: 0, eta: 0, startTime, fileName: displayName });

            const typeLabel = isFolder ? "folder" : isMultipleFiles ? "files" : "file";
            log(`Streaming ${typeLabel} "${displayName}" (${formatBytes(fileToSend.size)})`);

            const fileBuffer = await fileToSend.arrayBuffer();
            const fileHash = await calculateSHA256(fileBuffer);
            log(`Calculated hash: ${fileHash.substring(0, 8)}...${fileHash.substring(fileHash.length - 8)}`);

            const metadata = {
                name: fileToSend.name,
                total_size: fileToSend.size,
                hash: fileHash,
            };

            if (isFolder) {
                metadata.is_folder = true;
                metadata.original_folder_name = originalFolderName;
            } else if (isMultipleFiles) {
                metadata.is_multiple_files = true;
            }

            const metadataJSON = JSON.stringify(metadata);
            const metadataBytes = new TextEncoder().encode(metadataJSON);
            const { iv: metadataIV, ciphertext: encryptedMetadataBytes } = await encryptBytes(aesKeyRef.current, metadataBytes);

            const fileStartMsg = {
                type: "file_start",
                encrypted_metadata: uint8ToBase64(encryptedMetadataBytes),
                metadata_iv: uint8ToBase64(metadataIV),
            };

            sendMsg(fileStartMsg);

            pendingChunksRef.current = new Map();
            ackReceivedRef.current = new Set();
            lastActivityTimeRef.current = Date.now();

            const maxRetries = 3;
            const ackTimeout = 5000;
            const transferTimeout = 30000;
            let sendingComplete = false;

            retransmitTimerRef.current = setInterval(() => {
                const now = Date.now();

                if (now - lastActivityTimeRef.current > transferTimeout) {
                    clearInterval(retransmitTimerRef.current);
                    retransmitTimerRef.current = null;
                    log("Transfer timeout: no activity for 30 seconds");
                    setUploadProgress(null);
                    return;
                }

                for (const [chunkNum, chunkInfo] of pendingChunksRef.current.entries()) {
                    if (now - chunkInfo.sentTime > ackTimeout) {
                        if (chunkInfo.retries >= maxRetries) {
                            clearInterval(retransmitTimerRef.current);
                            retransmitTimerRef.current = null;
                            log(`Failed to send chunk ${chunkNum} after ${maxRetries} retries`);
                            setUploadProgress(null);
                            return;
                        }

                        sendMsg({
                            type: "file_chunk",
                            chunk_num: chunkNum,
                            chunk_data: chunkInfo.chunkData,
                            iv_b64: chunkInfo.ivB64,
                        });
                        chunkInfo.sentTime = now;
                        chunkInfo.retries++;
                        lastActivityTimeRef.current = now;
                    }
                }
            }, 500);

            const chunkSize = 512 * 1024;
            let sentBytes = 0;
            let chunkNum = 0;

            const stream = fileToSend.stream();
            const reader = stream.getReader();

            try {
                while (true) {
                    const { done, value } = await reader.read();
                    if (done) break;

                    const { iv, ciphertext } = await encryptBytes(aesKeyRef.current, value);
                    const chunkData = uint8ToBase64(ciphertext);
                    const ivB64 = uint8ToBase64(iv);

                    pendingChunksRef.current.set(chunkNum, {
                        chunkData: chunkData,
                        ivB64: ivB64,
                        sentTime: Date.now(),
                        retries: 0,
                    });

                    sendMsg({ type: "file_chunk", chunk_num: chunkNum, chunk_data: chunkData, iv_b64: ivB64 });

                    sentBytes += value.length;
                    const elapsed = (Date.now() - startTime) / 1000;
                    const speed = elapsed > 0 ? sentBytes / elapsed : 0;
                    const percent = Math.round((sentBytes / fileToSend.size) * 100);
                    const remainingBytes = fileToSend.size - sentBytes;
                    const eta = speed > 0 ? remainingBytes / speed : 0;

                    setUploadProgress({ percent, speed, eta, startTime, fileName: displayName });

                    chunkNum++;
                    lastActivityTimeRef.current = Date.now();

                    await new Promise((resolve) => setTimeout(resolve, 10));
                }
            } finally {
                reader.releaseLock();
            }

            sendingComplete = true;

            const waitStart = Date.now();
            while (pendingChunksRef.current.size > 0) {
                if (Date.now() - waitStart > 30000) {
                    clearInterval(retransmitTimerRef.current);
                    retransmitTimerRef.current = null;
                    log("Timeout waiting for chunk acknowledgments");
                    setUploadProgress(null);
                    return;
                }
                await new Promise((resolve) => setTimeout(resolve, 100));
            }

            clearInterval(retransmitTimerRef.current);
            retransmitTimerRef.current = null;

            sendMsg({ type: "file_end" });

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

    async function handleTextSend() {
        if (!textInput.trim() || !aesKeyRef.current) {
            return;
        }

        try {
            log(`Sending text message`);

            const metadata = { is_text: true, text: textInput };
            const metadataJSON = JSON.stringify(metadata);
            const metadataBytes = new TextEncoder().encode(metadataJSON);
            const { iv: metadataIV, ciphertext: encryptedMetadataBytes } = await encryptBytes(aesKeyRef.current, metadataBytes);

            const textMsg = {
                type: "text_message",
                encrypted_metadata: uint8ToBase64(encryptedMetadataBytes),
                metadata_iv: uint8ToBase64(metadataIV),
            };

            sendMsg(textMsg);
            log(`Sent encrypted text`);
            setTextInput("");
            toast.success("Text sent!", { duration: 2000 });
        } catch (err) {
            console.error(err);
            log("Failed to send text: " + (err.message || "unknown error"));
            toast.error("Failed to send text");
        }
    }

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

            for (let i = 0; i < items.length; i++) {
                const item = items[i];
                if (item.kind === "file") {
                    const entry = item.webkitGetAsEntry();
                    if (entry) {
                        if (entry.isDirectory) {
                            isFolder = true;
                            folderName = entry.name;
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
                const fileList = {
                    length: allFiles.length,
                    item: (index) => allFiles[index],
                    [Symbol.iterator]: function* () {
                        for (let i = 0; i < allFiles.length; i++) {
                            yield allFiles[i];
                        }
                    },
                };

                for (let i = 0; i < allFiles.length; i++) {
                    fileList[i] = allFiles[i];
                }

                const syntheticEvent = { target: { files: fileList } };
                handleFileSelect(syntheticEvent);
            }
        } catch (err) {
            console.error("Error processing dropped items:", err);
            log("Failed to process dropped items");
        }
    }

    async function readDirectory(dirEntry, basePath = "") {
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
                                    const path = basePath ? `${basePath}/${f.name}` : f.name;
                                    const newFile = new File([f], f.name, { type: f.type, lastModified: f.lastModified });
                                    Object.defineProperty(newFile, "webkitRelativePath", { value: path, writable: false });
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

                    readEntries();
                }, reject);
            };

            readEntries();
        });
    }

    function handleConfirmDownload() {
        if (!pendingDownload) return;

        const a = document.createElement("a");
        a.href = pendingDownload.url;
        a.download = pendingDownload.name;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);

        setDownloadUrl(pendingDownload.url);
        setDownloadName(pendingDownload.name);
        setShowDownloadConfirmModal(false);

        log(`Download started: "${pendingDownload.name}"`);
    }

    function handleCancelDownload() {
        if (pendingDownload) {
            URL.revokeObjectURL(pendingDownload.url);
            setPendingDownload(null);
        }
        setShowDownloadConfirmModal(false);
        log("Download cancelled");
    }

    useEffect(() => {
        return () => {
            if (wsRef.current && wsRef.current.readyState === 1) {
                wsRef.current.close();
            }
        };
    }, []);

    useEffect(() => {
        if (pathRoom && !connected) {
            connectToRoom();
        }
    }, [pathRoom, connected, connectToRoom]);

    useEffect(() => {
        document.title = roomId ? `e2ecp · ${roomId.toUpperCase()}` : "e2ecp";
    }, [roomId]);

    useEffect(() => {
        if (!pathRoom && !connected && roomInputRef.current) {
            roomInputRef.current.focus();
        }
    }, []);

    useEffect(() => {
        try {
            const saved = localStorage.getItem("darkMode");
            if (saved !== null) {
                setDarkMode(saved === "true");
            } else if (window.matchMedia) {
                const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
                setDarkMode(mediaQuery.matches);

                const handleChange = (e) => {
                    try {
                        const userPref = localStorage.getItem("darkMode");
                        if (userPref === null) {
                            setDarkMode(e.matches);
                        }
                    } catch (error) {
                        setDarkMode(e.matches);
                    }
                };

                mediaQuery.addEventListener("change", handleChange);
                return () => mediaQuery.removeEventListener("change", handleChange);
            }
        } catch (error) {
            console.warn("Could not access localStorage for dark mode preference:", error);
            if (window.matchMedia) {
                setDarkMode(window.matchMedia("(prefers-color-scheme: dark)").matches);
            }
        }
    }, []);

    const toggleDarkMode = () => {
        setDarkMode((prev) => {
            const newValue = !prev;
            try {
                localStorage.setItem("darkMode", String(newValue));
            } catch (error) {
                console.warn("Could not save dark mode preference:", error);
            }
            return newValue;
        });
    };

    useEffect(() => {
        if (darkMode) {
            document.documentElement.setAttribute("data-theme", "dark");
        } else {
            document.documentElement.setAttribute("data-theme", "light");
        }
    }, [darkMode]);

    return (
        <div className="min-h-screen bg-base-100 p-4 md:p-8 flex flex-col items-center justify-center">
            <Toaster
                position="bottom-right"
                toastOptions={{
                    duration: 2000,
                    style: {
                        background: "var(--toast-bg)",
                        color: "var(--toast-text)",
                        border: "var(--toast-border)",
                        borderRadius: "0.5rem",
                        padding: "12px 16px",
                    },
                    success: {
                        iconTheme: {
                            primary: "var(--toast-icon-primary)",
                            secondary: "var(--toast-icon-secondary)",
                        },
                    },
                }}
            />

            <div className="max-w-5xl w-full flex-grow flex flex-col justify-center gap-6">
                {/* Header */}
                <div className="card bg-gradient-to-r from-primary to-secondary text-primary-content shadow-xl">
                    <div className="card-body">
                        <div className="flex items-start justify-between flex-wrap gap-4">
                            <div className="flex-1">
                                <div className="flex items-center gap-3 mb-3">
                                    <h1 className="text-4xl md:text-6xl font-bold">
                                        <a href="/" className="hover:opacity-80 transition-opacity">
                                            e2ecp
                                        </a>
                                    </h1>
                                    <div className="flex gap-2">
                                        <button
                                            onClick={() => setShowAboutModal(true)}
                                            className="btn btn-sm btn-circle btn-ghost"
                                            aria-label="About e2ecp"
                                        >
                                            <i className="fas fa-question text-lg"></i>
                                        </button>
                                        <button
                                            onClick={toggleDarkMode}
                                            className="btn btn-sm btn-circle btn-ghost"
                                            aria-label={darkMode ? "Switch to light mode" : "Switch to dark mode"}
                                        >
                                            <i className={`fas ${darkMode ? "fa-sun" : "fa-moon"} text-lg`}></i>
                                        </button>
                                    </div>
                                </div>
                                <p className="text-lg md:text-xl opacity-90 mb-4">
                                    Transfer files or folders between machines with end-to-end encryption
                                </p>
                                {myMnemonic && (
                                    <div className="flex flex-wrap items-center gap-3">
                                        <IconBadge mnemonic={myMnemonic} label="You" />
                                        <i className="fas fa-arrows-left-right text-2xl opacity-70"></i>
                                        <button
                                            onClick={() => {
                                                const url = `${window.location.protocol}//${window.location.host}/${roomId}`;
                                                navigator.clipboard.writeText(url).then(() => {
                                                    toast.success("Copied to clipboard");
                                                }).catch((err) => {
                                                    toast.error("Failed to copy");
                                                    console.error("Failed to copy:", err);
                                                });
                                            }}
                                            className="btn btn-sm btn-outline hover:btn-accent"
                                            title="Copy URL to clipboard"
                                        >
                                            {roomId ? roomId.toUpperCase() : "ROOM"}
                                        </button>
                                        {peerMnemonic && (
                                            <>
                                                <i className="fas fa-arrows-left-right text-2xl opacity-70"></i>
                                                <IconBadge mnemonic={peerMnemonic} label="Peer" />
                                            </>
                                        )}
                                    </div>
                                )}
                            </div>
                            {myMnemonic && !peerMnemonic && roomId && (
                                <div className="w-32 md:w-40">
                                    <div className="bg-white p-2 rounded-lg">
                                        <QRCodeSVG
                                            value={`${window.location.origin}/${roomId}`}
                                            size={140}
                                            level="M"
                                            className="w-full h-auto"
                                        />
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                </div>

                {/* Connection Panel - only show on home page */}
                {!pathRoom && (
                    <div className="card bg-base-200 shadow-lg">
                        <div className="card-body">
                            <div className="flex flex-col sm:flex-row gap-3 mb-4">
                                <input
                                    ref={roomInputRef}
                                    type="text"
                                    placeholder="Enter room ID or press Connect"
                                    value={roomId}
                                    disabled={connected}
                                    onChange={(e) => {
                                        setRoomId(e.target.value);
                                        setRoomIdError(null);
                                    }}
                                    onKeyDown={(e) => e.key === "Enter" && !connected && handleConnect()}
                                    className={`input input-bordered flex-1 ${roomIdError ? "input-error" : ""}`}
                                />
                                <button
                                    onClick={handleConnect}
                                    disabled={connected}
                                    className="btn btn-primary"
                                >
                                    {connected ? "Connected" : "Connect"}
                                </button>
                            </div>
                            <div className={`alert ${roomIdError ? "alert-error" : connected && myMnemonic ? "alert-success" : "alert-info"}`}>
                                <div className="flex items-center gap-2">
                                    <i className={`fas ${roomIdError ? "fa-exclamation-circle" : connected && myMnemonic ? "fa-check-circle" : "fa-info-circle"}`}></i>
                                    <span>
                                        {roomIdError ? (
                                            <>ERROR: {roomIdError.toUpperCase()}</>
                                        ) : connected && myMnemonic ? (
                                            peerMnemonic ? (
                                                <>
                                                    Connected as {myMnemonic.toUpperCase()} (
                                                    {myIconClasses.map((iconClass, index) => (
                                                        <i key={index} className={`fas ${iconClass} mx-1`} aria-hidden="true"></i>
                                                    ))}
                                                    ) to {peerMnemonic.toUpperCase()} (
                                                    {peerIconClasses.map((iconClass, index) => (
                                                        <i key={index} className={`fas ${iconClass} mx-1`} aria-hidden="true"></i>
                                                    ))}
                                                    )
                                                </>
                                            ) : (
                                                <>
                                                    Connected as {myMnemonic.toUpperCase()} (
                                                    {myIconClasses.map((iconClass, index) => (
                                                        <i key={index} className={`fas ${iconClass} mx-1`} aria-hidden="true"></i>
                                                    ))}
                                                    )
                                                </>
                                            )
                                        ) : (
                                            <>Status: {status.toUpperCase()}</>
                                        )}
                                    </span>
                                </div>
                            </div>
                        </div>
                    </div>
                )}

                {/* File Transfer Panel */}
                {connected && (
                    <div className="card bg-base-200 shadow-lg">
                        <div className="card-body">
                            {uploadProgress && <ProgressBar progress={uploadProgress} label={`Sending ${uploadProgress.fileName}`} />}
                            {downloadProgress && <ProgressBar progress={downloadProgress} label={`Receiving ${downloadProgress.fileName}`} />}

                            <div
                                className={`card ${hasAesKey ? (isDragging ? "bg-accent/20 border-accent border-dashed" : "bg-base-100") : "bg-base-300"} border-2 transition-all`}
                                onDragOver={handleDragOver}
                                onDragEnter={handleDragEnter}
                                onDragLeave={handleDragLeave}
                                onDrop={handleDrop}
                            >
                                <div className="card-body text-center">
                                    {hasAesKey ? (
                                        isDragging ? (
                                            <div className="text-xl font-bold flex items-center justify-center gap-2">
                                                <i className="fas fa-folder-open text-3xl"></i>
                                                <span>Drop files or folder here</span>
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
                                                    className="btn btn-lg btn-outline btn-primary mb-4"
                                                >
                                                    <i className="fas fa-upload mr-2"></i>
                                                    Click or drop files here
                                                </button>

                                                <div className="divider">OR SEND TEXT</div>

                                                <div className="flex gap-2">
                                                    <input
                                                        type="text"
                                                        value={textInput}
                                                        onChange={(e) => setTextInput(e.target.value)}
                                                        onKeyDown={(e) => e.key === "Enter" && handleTextSend()}
                                                        placeholder="Type your message here..."
                                                        disabled={!hasAesKey}
                                                        className="input input-bordered flex-1"
                                                    />
                                                    <button
                                                        onClick={handleTextSend}
                                                        disabled={!hasAesKey || !textInput.trim()}
                                                        className="btn btn-primary"
                                                    >
                                                        <i className="fas fa-paper-plane"></i>
                                                    </button>
                                                </div>
                                            </>
                                        )
                                    ) : (
                                        <div className="flex items-center justify-center gap-2">
                                            <div className="loading loading-spinner loading-md"></div>
                                            <span>Waiting for peer to join {window.location.host}/{roomId}</span>
                                            <button
                                                onClick={() => {
                                                    const url = `${window.location.protocol}//${window.location.host}/${roomId}`;
                                                    navigator.clipboard.writeText(url).then(() => {
                                                        toast.success("Copied to clipboard");
                                                    }).catch((err) => {
                                                        toast.error("Failed to copy");
                                                        console.error("Failed to copy:", err);
                                                    });
                                                }}
                                                className="btn btn-sm btn-ghost"
                                                title="Copy URL to clipboard"
                                            >
                                                <i className="fas fa-copy"></i>
                                            </button>
                                        </div>
                                    )}
                                </div>
                            </div>

                            {downloadUrl && (
                                <div className="alert alert-success mt-4">
                                    <div>
                                        <div className="font-bold mb-2">File Ready:</div>
                                        <a
                                            href={downloadUrl}
                                            download={downloadName}
                                            className="link link-primary text-lg font-bold break-all"
                                        >
                                            {downloadName}
                                        </a>
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                )}
            </div>

            {/* Error Modal */}
            {showErrorModal && (
                <div className="modal modal-open">
                    <div className="modal-box">
                        <h3 className="font-bold text-2xl mb-4 text-center">Maximum Rooms</h3>
                        <p className="text-center mb-6">Please try again later</p>
                        <div className="modal-action">
                            <button
                                onClick={() => {
                                    setShowErrorModal(false);
                                    setRoomId("");
                                }}
                                className="btn btn-primary w-full"
                            >
                                OK
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* About Modal */}
            {showAboutModal && (
                <div className="modal modal-open" onClick={() => setShowAboutModal(false)}>
                    <div className="modal-box" onClick={(e) => e.stopPropagation()}>
                        <h3 className="font-bold text-2xl mb-4 text-center">What is e2ecp?</h3>
                        <p className="mb-4 text-center">
                            e2ecp allows two computers to transfer files with end-to-end encryption via a zero-knowledge relay.
                        </p>
                        <p className="mb-4 text-center">
                            Use the CLI to transfer files between web or terminals:
                            <br />
                            <code className="bg-base-300 px-2 py-1 rounded">curl https://e2ecp.com | bash</code>
                        </p>
                        <div className="mb-4 text-center">
                            <a
                                href="https://github.com/schollz/e2ecp"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="link link-primary text-xl"
                                aria-label="View on GitHub"
                            >
                                <i className="fab fa-github text-3xl"></i>
                            </a>
                        </div>
                        <div className="modal-action">
                            <button onClick={() => setShowAboutModal(false)} className="btn btn-primary w-full">
                                Close
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* Download Confirmation Modal */}
            {showDownloadConfirmModal && pendingDownload && (
                <div className="modal modal-open">
                    <div className="modal-box">
                        <h3 className="font-bold text-2xl mb-4 text-center">Download File?</h3>
                        <div className="bg-base-300 p-4 rounded-lg mb-4">
                            <p className="mb-2">
                                <span className="font-bold">Name:</span> {pendingDownload.name}
                            </p>
                            <p className="mb-2">
                                <span className="font-bold">Type:</span> {pendingDownload.type}
                            </p>
                            <p>
                                <span className="font-bold">Size:</span> {pendingDownload.size}
                            </p>
                        </div>
                        <p className="text-center mb-6">Do you want to download this {pendingDownload.type}?</p>
                        <div className="modal-action">
                            <button onClick={handleCancelDownload} className="btn btn-ghost flex-1">
                                Cancel
                            </button>
                            <button onClick={handleConfirmDownload} className="btn btn-primary flex-1">
                                Download
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* Text Message Modal */}
            {showTextModal && receivedText && (
                <div className="modal modal-open">
                    <div className="modal-box">
                        <h3 className="font-bold text-2xl mb-4 text-center">Received Text</h3>
                        <div className="bg-base-300 p-4 rounded-lg mb-4 relative">
                            <div className="whitespace-pre-wrap break-words max-h-96 overflow-y-auto">
                                {receivedText}
                            </div>
                            <button
                                onClick={() => {
                                    navigator.clipboard.writeText(receivedText).then(() => {
                                        toast.success("Text copied to clipboard!");
                                    }).catch((err) => {
                                        toast.error("Failed to copy text");
                                        console.error("Failed to copy:", err);
                                    });
                                }}
                                className="btn btn-sm btn-circle btn-ghost absolute top-2 right-2"
                                title="Copy to clipboard"
                            >
                                <i className="fas fa-copy"></i>
                            </button>
                        </div>
                        <div className="modal-action">
                            <button
                                onClick={() => {
                                    setShowTextModal(false);
                                    setReceivedText(null);
                                }}
                                className="btn btn-primary w-full"
                            >
                                Close
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
