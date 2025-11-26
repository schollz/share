import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "./AuthContext";
import toast from "react-hot-toast";

export default function Settings() {
    const { token, logout } = useAuth();
    const navigate = useNavigate();
    const [currentPassword, setCurrentPassword] = useState("");
    const [newPassword, setNewPassword] = useState("");
    const [changingPassword, setChangingPassword] = useState(false);
    const [deletePassword, setDeletePassword] = useState("");
    const [deleting, setDeleting] = useState(false);

    useEffect(() => {
        if (!token) {
            navigate("/login");
        }
    }, [token, navigate]);

    const handleChangePassword = async (e) => {
        e.preventDefault();
        setChangingPassword(true);

        try {
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

            toast.success("Password updated");
            setCurrentPassword("");
            setNewPassword("");
        } catch (err) {
            toast.error(err.message || "Failed to change password");
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
