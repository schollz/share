import React, { useEffect, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import toast from "react-hot-toast";
import { useAuth } from "./AuthContext";
import { deriveKey } from "./encryption";

export default function VerifyEmail() {
  const [searchParams] = useSearchParams();
  const tokenParam = searchParams.get("token") || "";
  const { verifyEmailToken, setEncryptionKey } = useAuth();
  const navigate = useNavigate();
  const [status, setStatus] = useState("pending");
  const [user, setUser] = useState(null);
  const [password, setPassword] = useState("");
  const [unlocking, setUnlocking] = useState(false);
  const attempted = useRef(false);

  useEffect(() => {
    const run = async () => {
      if (attempted.current) return;
      attempted.current = true;
      if (!tokenParam) {
        setStatus("error");
        return;
      }
      try {
        const data = await verifyEmailToken(tokenParam);
        if (!data?.user) {
          throw new Error("Verification response invalid");
        }
        setUser(data.user);
        setStatus("verified");
        toast.success("Email verified!");
      } catch (err) {
        setStatus("error");
        toast.error(err.message || "Verification failed");
      }
    };
    run();
  }, [tokenParam, verifyEmailToken]);

  const handleUnlock = async (e) => {
    e.preventDefault();
    if (!user) return;
    setUnlocking(true);
    try {
      const key = await deriveKey(password, user.encryption_salt);
      setEncryptionKey(key);

      const exportedKey = await window.crypto.subtle.exportKey("raw", key);
      const keyArray = new Uint8Array(exportedKey);
      const keyHex = Array.from(keyArray)
        .map((b) => b.toString(16).padStart(2, "0"))
        .join("");
      sessionStorage.setItem("encryptionKey", keyHex);

      toast.success("Signed in!");
      navigate("/profile");
    } catch (err) {
      toast.error("Invalid password for unlocking encryption key");
    } finally {
      setUnlocking(false);
    }
  };

  return (
    <div className="min-h-screen bg-white dark:bg-black text-black dark:text-white flex items-center justify-center p-4">
      <div className="w-full max-w-md border-4 border-black dark:border-white p-6 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]">
        {status === "pending" && (
          <div className="text-center text-lg font-bold">Verifying...</div>
        )}
        {status === "error" && (
          <div className="text-center">
            <h1 className="text-2xl font-black uppercase mb-2">
              Verification failed
            </h1>
            <p className="mb-4">The link is invalid or expired.</p>
            <button
              onClick={() => navigate("/login")}
              className="border-2 border-black dark:border-white px-4 py-2 font-bold uppercase"
            >
              Back to Login
            </button>
          </div>
        )}
        {status === "verified" && user && (
          <div>
            <h1 className="text-3xl font-black uppercase mb-3">
              Email Verified
            </h1>
            <p className="mb-4 text-sm">
              Enter your password to unlock your encryption key and finish
              signing in.
            </p>
            <form onSubmit={handleUnlock} className="space-y-4">
              <div>
                <label className="block text-sm font-bold mb-2 uppercase">
                  Password
                </label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  className="w-full border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 focus:outline-none"
                />
              </div>
              <button
                type="submit"
                disabled={unlocking}
                className="border-2 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-4 py-2 font-bold uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors disabled:opacity-60"
              >
                {unlocking ? "Signing in..." : "Sign In"}
              </button>
            </form>
          </div>
        )}
      </div>
    </div>
  );
}
