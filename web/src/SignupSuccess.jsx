import React from "react";
import { useLocation, useNavigate } from "react-router-dom";
import Navbar from "./Navbar";

export default function SignupSuccess() {
    const navigate = useNavigate();
    const location = useLocation();
    const email = location.state?.email;

    return (
        <div className="min-h-screen bg-white dark:bg-black text-black dark:text-white">
            <Navbar title="Sign Up Success" />
            <div className="flex items-center justify-center p-4">
            <div className="w-full max-w-xl border-4 border-black dark:border-white bg-white dark:bg-black p-8 shadow-[10px_10px_0px_0px_rgba(0,0,0,1)] dark:shadow-[10px_10px_0px_0px_rgba(255,255,255,1)]">
                <h1 className="text-4xl sm:text-5xl font-black uppercase mb-4">
                    Signed up!
                </h1>
                <p className="text-lg sm:text-xl mb-6">
                    Check your email for the login link
                    {email ? ` (${email})` : ""}. We send it right away so you
                    can finish verifying your account.
                </p>
                <div className="space-y-3">
                    <button
                        type="button"
                        onClick={() => navigate("/login", { replace: true })}
                        className="w-full border-2 sm:border-4 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-4 py-3 text-base sm:text-lg font-black uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors cursor-pointer shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] dark:shadow-[6px_6px_0px_0px_rgba(255,255,255,1)] hover:translate-x-1 hover:translate-y-1 hover:shadow-none active:translate-x-2 active:translate-y-2"
                    >
                        Go to Login
                    </button>
                    <button
                        type="button"
                        onClick={() => navigate("/", { replace: true })}
                        className="w-full border-2 border-black dark:border-white px-4 py-3 text-base font-bold uppercase hover:underline"
                    >
                        Back to Home
                    </button>
                </div>
            </div>
            </div>
        </div>
    );
}
