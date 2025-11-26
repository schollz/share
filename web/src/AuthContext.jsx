import React, { createContext, useContext, useState, useEffect } from "react";
import { deriveKey } from "./encryption";

const AuthContext = createContext(null);

export const useAuth = () => {
    const context = useContext(AuthContext);
    if (!context) {
        throw new Error("useAuth must be used within an AuthProvider");
    }
    return context;
};

export const AuthProvider = ({ children }) => {
    const [user, setUser] = useState(null);
    const [token, setToken] = useState(localStorage.getItem("token"));
    const [loading, setLoading] = useState(true);
    const [encryptionKey, setEncryptionKey] = useState(null);

    // Restore encryption key from sessionStorage on mount
    useEffect(() => {
        const restoreEncryptionKey = async () => {
            const storedKeyHex = sessionStorage.getItem("encryptionKey");
            if (storedKeyHex) {
                try {
                    // Convert hex to bytes
                    const keyBytes = new Uint8Array(
                        storedKeyHex.match(/.{1,2}/g).map((byte) => parseInt(byte, 16))
                    );
                    // Import the key
                    const key = await window.crypto.subtle.importKey(
                        "raw",
                        keyBytes,
                        { name: "AES-GCM", length: 256 },
                        true,
                        ["encrypt", "decrypt"]
                    );
                    setEncryptionKey(key);
                } catch (error) {
                    console.error("Failed to restore encryption key:", error);
                    sessionStorage.removeItem("encryptionKey");
                }
            }
        };

        restoreEncryptionKey();
    }, []);

    // Verify token on mount
    useEffect(() => {
        const verifyToken = async () => {
            if (!token) {
                setLoading(false);
                return;
            }

            try {
                const response = await fetch("/api/auth/verify", {
                    headers: {
                        Authorization: `Bearer ${token}`,
                    },
                });

                if (response.ok) {
                    const userData = await response.json();
                    setUser(userData);
                } else {
                    // Token is invalid, clear it
                    localStorage.removeItem("token");
                    sessionStorage.removeItem("encryptionKey");
                    setToken(null);
                    setUser(null);
                }
            } catch (error) {
                console.error("Token verification failed:", error);
                localStorage.removeItem("token");
                sessionStorage.removeItem("encryptionKey");
                setToken(null);
                setUser(null);
            } finally {
                setLoading(false);
            }
        };

        verifyToken();
    }, [token]);

    const login = async (email, password) => {
        const response = await fetch("/api/auth/login", {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify({ email, password }),
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || "Login failed");
        }

        const data = await response.json();
        localStorage.setItem("token", data.token);
        setToken(data.token);
        setUser(data.user);

        // Derive encryption key from password and salt
        const key = await deriveKey(password, data.user.encryption_salt);
        setEncryptionKey(key);

        // Store encryption key in sessionStorage for persistence across page refreshes
        const exportedKey = await window.crypto.subtle.exportKey("raw", key);
        const keyArray = new Uint8Array(exportedKey);
        const keyHex = Array.from(keyArray)
            .map((b) => b.toString(16).padStart(2, "0"))
            .join("");
        sessionStorage.setItem("encryptionKey", keyHex);

        return data;
    };

    const register = async (email, password) => {
        const response = await fetch("/api/auth/register", {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify({ email, password }),
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || "Registration failed");
        }

        const data = await response.json();
        return data;
    };

    const verifyEmailToken = async (tokenParam) => {
        const response = await fetch(`/api/auth/verify-email?token=${encodeURIComponent(tokenParam)}`);
        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
            throw new Error(error.error || "Verification failed");
        }

        const data = await response.json();
        if (data?.token && data?.user) {
            localStorage.setItem("token", data.token);
            setToken(data.token);
            setUser(data.user);
        } else {
            throw new Error("Verification response invalid");
        }
        return data;
    };

    const logout = () => {
        localStorage.removeItem("token");
        sessionStorage.removeItem("encryptionKey");
        setToken(null);
        setUser(null);
        setEncryptionKey(null);
    };

    const value = {
        user,
        token,
        loading,
        encryptionKey,
        setEncryptionKey,
        login,
        register,
        verifyEmailToken,
        logout,
        isAuthenticated: !!user,
    };

    return (
        <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
    );
};
