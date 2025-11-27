import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "./AuthContext";
import toast from "react-hot-toast";

export default function DeviceAuth() {
    const { token } = useAuth();
    const navigate = useNavigate();
    const [userCode, setUserCode] = useState("");
    const [approving, setApproving] = useState(false);

    const handleApprove = async (e) => {
        e.preventDefault();

        if (!token) {
            toast.error("You must be logged in to approve a device");
            navigate("/login");
            return;
        }

        const trimmedCode = userCode.trim().toUpperCase();
        console.log("Sending user_code:", trimmedCode);

        setApproving(true);
        try {
            const requestBody = { user_code: trimmedCode };
            console.log("Request body:", JSON.stringify(requestBody));

            const response = await fetch("/api/auth/device/approve", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    "Authorization": `Bearer ${token}`,
                },
                body: JSON.stringify(requestBody),
            });

            if (!response.ok) {
                const errorText = await response.text();
                console.error("Error response:", errorText);
                let errorMsg;
                try {
                    const error = JSON.parse(errorText);
                    errorMsg = error.error || "Failed to approve device";
                } catch {
                    errorMsg = errorText || "Failed to approve device";
                }
                throw new Error(errorMsg);
            }

            toast.success("Device approved successfully!");
            setUserCode("");
            // Optionally redirect to profile
            setTimeout(() => navigate("/profile"), 1500);
        } catch (err) {
            console.error("Approval error:", err);
            toast.error(err.message || "Failed to approve device");
        } finally {
            setApproving(false);
        }
    };

    return (
        <div className="min-h-screen bg-white dark:bg-black text-black dark:text-white flex items-center justify-center p-4">
            <div className="w-full max-w-md border-4 border-black dark:border-white p-6 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
                <h1 className="text-3xl font-black uppercase mb-4">Device Auth</h1>
                <p className="mb-6 text-sm">
                    Enter the code displayed on your device to authorize it to access your account.
                </p>

                <form onSubmit={handleApprove} className="space-y-4">
                    <div>
                        <label className="block text-sm font-bold mb-2 uppercase">
                            Device Code
                        </label>
                        <input
                            type="text"
                            value={userCode}
                            onChange={(e) => setUserCode(e.target.value.toUpperCase())}
                            placeholder="XXXXXX"
                            maxLength="6"
                            required
                            className="w-full border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 focus:outline-none text-center text-2xl font-bold tracking-widest"
                        />
                    </div>

                    <button
                        type="submit"
                        disabled={approving || userCode.length !== 6}
                        className="w-full border-2 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-4 py-2 font-bold uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors disabled:opacity-60"
                    >
                        {approving ? "Approving..." : "Approve Device"}
                    </button>
                </form>

                <div className="mt-6 pt-6 border-t-2 border-black dark:border-white">
                    <button
                        onClick={() => navigate("/profile")}
                        className="text-sm font-bold underline"
                    >
                        Back to Profile
                    </button>
                </div>
            </div>
        </div>
    );
}
