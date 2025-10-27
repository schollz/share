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

/* ------------------- React Component ------------------- */

export default function App() {
    // Parse room from URL path (e.g., /myroom -> "myroom")
    const pathRoom = window.location.pathname.slice(1);
    const [roomId, setRoomId] = useState(pathRoom);
    const [connected, setConnected] = useState(false);
    const [peerCount, setPeerCount] = useState(1);
    const [status, setStatus] = useState("Not connected");
    const [downloadUrl, setDownloadUrl] = useState(null);
    const [downloadName, setDownloadName] = useState(null);
    const [myMnemonic, setMyMnemonic] = useState(null);
    const [peerMnemonic, setPeerMnemonic] = useState(null);
    const [hasAesKey, setHasAesKey] = useState(false);

    const myKeyPairRef = useRef(null);
    const aesKeyRef = useRef(null);
    const havePeerPubRef = useRef(false);

    const wsRef = useRef(null);
    const selfIdRef = useRef(null);
    const myMnemonicRef = useRef(null);
    const clientIdRef = useRef(crypto.randomUUID());

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
        log("🔐 Generated ECDH key pair");
    }

    async function announcePublicKey() {
        const raw = await exportPubKey(myKeyPairRef.current.publicKey);
        sendMsg({ type: "pubkey", pub: uint8ToBase64(raw) });
        log("📡 Sent my public key");
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
        log("🤝 Derived shared AES key (E2EE ready)");
    }

    const connectToRoom = useCallback(async () => {
        const room = roomId.trim();
        if (!room) {
            alert("Enter a room ID first.");
            return;
        }
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
            log("🌐 WebSocket open");
            sendMsg({ type: "join", roomId: room, clientId: clientIdRef.current });
        };

        ws.onmessage = async (event) => {
            let msg;
            try {
                msg = JSON.parse(event.data);
            } catch {
                return;
            }

            if (msg.type === "joined") {
                selfIdRef.current = msg.selfId;
                const mnemonic = msg.mnemonic || msg.selfId;
                myMnemonicRef.current = mnemonic;
                setMyMnemonic(mnemonic);
                log(`✅ Joined room ${msg.roomId} as ${mnemonic}`);
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
                log(`📥 Received peer public key from ${peerName}`);
                const hadPeerPub = havePeerPubRef.current;
                await handlePeerPubKey(msg.pub);
                if (!hadPeerPub) await announcePublicKey();
                return;
            }

            if (msg.type === "file") {
                const { name, iv_b64, data_b64 } = msg;
                log(`📦 Incoming encrypted file: ${name}`);

                if (!aesKeyRef.current) {
                    log("❌ Can't decrypt yet (no shared key)");
                    return;
                }

                try {
                    const iv = base64ToUint8(iv_b64);
                    const ciphertext = base64ToUint8(data_b64);
                    const plainBytes = await decryptBytes(aesKeyRef.current, iv, ciphertext);

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

                    log(`✅ Decrypted and prepared download "${name}"`);
                } catch (err) {
                    console.error(err);
                    log("❌ Decryption failed");
                }
                return;
            }
        };

        ws.onclose = () => {
            log("🔌 WebSocket closed");
            setConnected(false);
            setPeerCount(1);
            setStatus("Not connected");
        };

        ws.onerror = (err) => {
            console.error("WS error", err);
            log("⚠️ WebSocket error");
        };
    }, [roomId]);

    async function handleFileSelect(e) {
        const file = e.target.files?.[0];
        if (!file || !aesKeyRef.current) {
            log("⚠️ No file or key not ready");
            return;
        }
        const arrBuf = await file.arrayBuffer();
        log(`🔒 Encrypting "${file.name}" (${arrBuf.byteLength} bytes)`);
        const { iv, ciphertext } = await encryptBytes(aesKeyRef.current, arrBuf);
        sendMsg({
            type: "file",
            name: file.name,
            iv_b64: uint8ToBase64(iv),
            data_b64: uint8ToBase64(ciphertext)
        });
        log(`🚀 Sent encrypted file "${file.name}"`);
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

    return (
        <div className="min-h-screen bg-white p-4 md:p-8 font-mono">
            <div className="max-w-4xl mx-auto">
                {/* Header */}
                <div className="bg-black text-white border-8 border-black p-8 mb-6 shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]">
                    <h1 className="text-6xl font-black mb-4 uppercase tracking-tight">
                        SHARE
                    </h1>
                    <p className="text-xl font-bold leading-tight">
                        E2EE FILE TRANSFER // ECDH + AES-GCM // ZERO-KNOWLEDGE RELAY
                    </p>
                    {myMnemonic && (
                        <div className="mt-4 bg-white text-black px-4 py-2 inline-block border-4 border-black font-black text-2xl uppercase">
                            {myMnemonic}
                        </div>
                    )}
                </div>

                {/* Connection Panel */}
                <div className="bg-gray-200 border-8 border-black p-6 mb-6 shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]">
                    <h2 className="text-3xl font-black mb-4 uppercase">ROOM</h2>
                    <div className="flex flex-col sm:flex-row gap-4 mb-4">
                        <input
                            type="text"
                            placeholder="ENTER ROOM ID"
                            value={roomId}
                            disabled={connected}
                            onChange={(e) => setRoomId(e.target.value)}
                            onKeyDown={(e) => e.key === "Enter" && !connected && connectToRoom()}
                            className="flex-1 border-4 border-black p-4 text-xl font-bold uppercase bg-white disabled:bg-gray-300 disabled:cursor-not-allowed focus:outline-hidden focus:ring-4 focus:ring-black"
                        />
                        <button
                            onClick={connectToRoom}
                            disabled={connected}
                            className={`border-4 border-black px-8 py-4 text-xl font-black uppercase transition-all whitespace-nowrap ${connected
                                ? "bg-gray-400 cursor-not-allowed"
                                : "bg-white hover:translate-x-1 hover:translate-y-1 hover:shadow-none active:translate-x-2 active:translate-y-2"
                                } shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]`}
                        >
                            {connected ? "CONNECTED" : "CONNECT"}
                        </button>
                    </div>
                    <div className="bg-black text-white border-4 border-black p-3 font-bold text-lg break-words">
                        {peerMnemonic ? (
                            <>PEER: {peerMnemonic.toUpperCase()} // CONNECTED</>
                        ) : (
                            <>STATUS: {status.toUpperCase()}</>
                        )}
                    </div>
                </div>

                {/* File Transfer Panel */}
                {connected && (
                    <div className="bg-gray-300 border-8 border-black p-6 mb-6 shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]">
                        {/* <h2 className="text-3xl font-black mb-4 uppercase">TRANSFER</h2> */}
                        <label
                            className={`block border-4 border-black p-8 text-center text-2xl font-black uppercase ${hasAesKey
                                ? "bg-white cursor-pointer hover:translate-x-1 hover:translate-y-1 hover:shadow-none transition-all shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]"
                                : "bg-gray-400 cursor-not-allowed"
                                }`}
                        >
                            {hasAesKey ? "SHARE" : "WAITING FOR PEER..."}
                            <input
                                type="file"
                                className="hidden"
                                onChange={handleFileSelect}
                                disabled={!hasAesKey}
                            />
                        </label>

                        {downloadUrl && (
                            <div className="mt-4 bg-white border-4 border-black p-4">
                                <div className="text-xl font-black mb-2">FILE READY:</div>
                                <a
                                    href={downloadUrl}
                                    download={downloadName}
                                    className="text-2xl font-black underline hover:no-underline text-black"
                                >
                                    📥 {downloadName}
                                </a>
                            </div>
                        )}
                    </div>
                )}

            </div>
        </div>
    );
}
