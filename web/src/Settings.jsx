import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "./AuthContext";
import toast from "react-hot-toast";
import {
    deriveKey,
    decryptFileKey,
    encryptFileKey,
    decryptString,
    encryptString,
} from "./encryption";
import { useConfig } from "./ConfigContext";

export default function Settings() {
    const { token, user, encryptionKey, setEncryptionKey, logout } = useAuth();
    const { storageEnabled, loading: configLoading } = useConfig();
    const navigate = useNavigate();
    const [currentPassword, setCurrentPassword] = useState("");
    const [newPassword, setNewPassword] = useState("");
    const [changingPassword, setChangingPassword] = useState(false);
    const [changeMessage, setChangeMessage] = useState(null);
    const [deletePassword, setDeletePassword] = useState("");
    const [deleting, setDeleting] = useState(false);

    useEffect(() => {
        if (!configLoading && !storageEnabled) {
            navigate("/");
            return;
        }
        if (!token) {
            navigate("/login");
        }
    }, [token, navigate, storageEnabled, configLoading]);

    const persistEncryptionKey = async (key) => {
        const exportedKey = await window.crypto.subtle.exportKey("raw", key);
        const keyArray = new Uint8Array(exportedKey);
        const keyHex = Array.from(keyArray)
            .map((b) => b.toString(16).padStart(2, "0"))
            .join("");
        sessionStorage.setItem("encryptionKey", keyHex);
        setEncryptionKey(key);
    };

    const prepareReencryptedFiles = async (newMasterKey) => {
        if (!encryptionKey) {
            throw new Error("Encryption key not available. Please re-login and try again.");
        }

        const listRes = await fetch("/api/files/list", {
            headers: {
                Authorization: `Bearer ${token}`,
            },
        });

        if (!listRes.ok) {
            const err = await listRes.json().catch(() => ({}));
            throw new Error(err.error || "Failed to load files for re-encryption");
        }

        const data = await listRes.json();
        const files = data.files || [];

        if (files.length === 0) {
            return { files: [], count: 0 };
        }

        const reencrypted = await Promise.all(
            files.map(async (file) => {
                const fileKey = await decryptFileKey(file.encrypted_key, encryptionKey);
                const newEncryptedKey = await encryptFileKey(fileKey, newMasterKey);
                const filename = await decryptString(file.encrypted_filename, encryptionKey);
                const newEncryptedFilename = await encryptString(filename, newMasterKey);
                return {
                    id: file.id,
                    encrypted_key: newEncryptedKey,
                    encrypted_filename: newEncryptedFilename,
                };
            })
        );

        return { files: reencrypted, count: files.length };
    };

    const handleChangePassword = async (e) => {
        e.preventDefault();
        setChangingPassword(true);
        setChangeMessage(null);

        if (!user?.encryption_salt) {
            setChangeMessage({
                type: "error",
                text: "Profile data unavailable. Please re-login and try again.",
            });
            setChangingPassword(false);
            return;
        }

        try {
            const newMasterKey = await deriveKey(newPassword, user.encryption_salt);

            const { files: reencryptedFiles, count } =
                await prepareReencryptedFiles(newMasterKey);

            const res = await fetch("/api/auth/change-password", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({
                    current_password: currentPassword,
                    new_password: newPassword,
                }),
            });

            if (!res.ok) {
                const err = await res.json().catch(() => ({}));
                throw new Error(err.error || "Failed to change password");
            }

            if (reencryptedFiles.length > 0) {
                const rekeyRes = await fetch("/api/files/rekey", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        Authorization: `Bearer ${token}`,
                    },
                    body: JSON.stringify({ files: reencryptedFiles }),
                });

                if (!rekeyRes.ok) {
                    const rekeyErr = await rekeyRes.json().catch(() => ({}));
                    // Attempt to roll back password to keep files accessible
                    let rollbackSucceeded = false;
                    try {
                        const rollbackRes = await fetch("/api/auth/change-password", {
                            method: "POST",
                            headers: {
                                "Content-Type": "application/json",
                                Authorization: `Bearer ${token}`,
                            },
                            body: JSON.stringify({
                                current_password: newPassword,
                                new_password: currentPassword,
                            }),
                        });
                        rollbackSucceeded = rollbackRes.ok;
                    } catch {
                        rollbackSucceeded = false;
                    }

                    const baseMessage = rekeyErr.error || "Failed to re-encrypt files with new password";
                    const rollbackMessage = rollbackSucceeded
                        ? " Password was reverted to keep your files accessible. Please try again."
                        : " Password change may have completed without re-encrypting files. Please log in with your previous password and try again.";

                    throw new Error(baseMessage + rollbackMessage);
                }
            }

            await persistEncryptionKey(newMasterKey);

            setChangeMessage({
                type: "success",
                text:
                    count > 0
                        ? `Password changed and ${count} file${count === 1 ? "" : "s"} re-encrypted.`
                        : "Password changed successfully.",
            });
            setCurrentPassword("");
            setNewPassword("");
        } catch (err) {
            setChangeMessage({
                type: "error",
                text: err.message || "Failed to change password",
            });
        } finally {
            setChangingPassword(false);
        }
    };

    const handleDeleteAccount = async (e) => {
        e.preventDefault();
        const confirmed = confirm(
            "This will delete your account and all uploaded files. This cannot be undone. Continue?",
        );
        if (!confirmed) return;
        setDeleting(true);

        try {
            const res = await fetch("/api/auth/delete-account", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({ password: deletePassword }),
            });

            if (!res.ok) {
                const err = await res.json().catch(() => ({}));
                throw new Error(err.error || "Failed to delete account");
            }

            toast.success("Account deleted");
            logout();
            navigate("/");
        } catch (err) {
            toast.error(err.message || "Failed to delete account");
        } finally {
            setDeleting(false);
        }
    };

    return (
        <div className="min-h-screen bg-white dark:bg-black text-black dark:text-white p-4 sm:p-8">
            <div className="max-w-3xl mx-auto space-y-8">
                <div className="flex justify-between items-center">
                    <h1 className="text-4xl sm:text-5xl font-black uppercase">
                        Settings
                    </h1>
                    <div className="flex gap-3">
                        <button
                            onClick={() => navigate("/profile")}
                            className="border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-4 py-2 font-bold uppercase hover:bg-gray-100 dark:hover:bg-gray-900 transition-colors"
                        >
                            Back to Files
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

                <div className="border-4 border-black dark:border-white p-6 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                    <h2 className="text-2xl font-black uppercase mb-4">
                        Change Password
                    </h2>
                    {changeMessage && (
                        <div
                            className={`mb-4 border-2 px-3 py-2 font-bold uppercase text-sm ${
                                changeMessage.type === "success"
                                    ? "border-green-600 text-green-700 dark:text-green-300"
                                    : "border-red-600 text-red-700 dark:text-red-300"
                            }`}
                        >
                            {changeMessage.text}
                        </div>
                    )}
                    <form onSubmit={handleChangePassword} className="space-y-4">
                        <div>
                            <label className="block text-sm font-bold mb-2 uppercase">
                                Current Password
                            </label>
                            <input
                                type="password"
                                value={currentPassword}
                                onChange={(e) => setCurrentPassword(e.target.value)}
                                required
                                className="w-full border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 focus:outline-none"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-bold mb-2 uppercase">
                                New Password
                            </label>
                            <input
                                type="password"
                                value={newPassword}
                                onChange={(e) => setNewPassword(e.target.value)}
                                minLength={6}
                                required
                                className="w-full border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 focus:outline-none"
                            />
                            <p className="text-xs mt-1">Minimum 6 characters</p>
                        </div>
                        <button
                            type="submit"
                            disabled={changingPassword}
                            className="border-2 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-4 py-2 font-bold uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors disabled:opacity-60"
                        >
                            {changingPassword ? "Updating..." : "Update Password"}
                        </button>
                    </form>
                </div>

                <div className="border-4 border-red-600 text-red-700 dark:text-red-300 dark:border-red-400 p-6 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(220,38,38,1)] dark:shadow-[8px_8px_0px_0px_rgba(248,113,113,1)]">
                    <h2 className="text-2xl font-black uppercase mb-3">
                        Delete Account
                    </h2>
                    <p className="text-sm mb-4">
                        This removes your account and all stored files. This action cannot be undone.
                    </p>
                    <form onSubmit={handleDeleteAccount} className="space-y-3">
                        <div>
                            <label className="block text-sm font-bold mb-2 uppercase">
                                Confirm with Password
                            </label>
                            <input
                                type="password"
                                value={deletePassword}
                                onChange={(e) => setDeletePassword(e.target.value)}
                                required
                                className="w-full border-2 border-red-600 dark:border-red-400 bg-white dark:bg-black text-black dark:text-white px-3 py-2 focus:outline-none"
                            />
                        </div>
                        <button
                            type="submit"
                            disabled={deleting}
                            className="border-2 border-red-700 text-white bg-red-700 px-4 py-2 font-bold uppercase hover:bg-red-800 transition-colors disabled:opacity-60"
                        >
                            {deleting ? "Deleting..." : "Delete Account"}
                        </button>
                    </form>
                </div>
            </div>
        </div>
    );
}
