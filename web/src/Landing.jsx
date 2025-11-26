import React from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "./AuthContext";
import App from "./App";
import { useConfig } from "./ConfigContext";

export default function Landing() {
    const navigate = useNavigate();
    const { user, isAuthenticated } = useAuth();
    const { storageEnabled, loading: configLoading } = useConfig();

    // Add a profile button to the existing App
    // We'll inject a header component
    return (
        <div className="relative">
            {/* Profile button overlay */}
            <div className="fixed top-4 right-4 z-50">
                {isAuthenticated ? (
                    <button
                        onClick={() => navigate("/profile")}
                        className="border-2 sm:border-4 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-4 py-2 sm:px-6 sm:py-3 text-sm sm:text-base font-black uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors cursor-pointer shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,1)] hover:translate-x-1 hover:translate-y-1 hover:shadow-none active:translate-x-2 active:translate-y-2"
                    >
                        <i className="fas fa-user mr-2"></i>
                        Profile
                    </button>
                ) : storageEnabled && !configLoading ? (
                    <button
                        onClick={() => navigate("/login")}
                        className="border-2 sm:border-4 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-4 py-2 sm:px-6 sm:py-3 text-sm sm:text-base font-black uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors cursor-pointer shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,1)] hover:translate-x-1 hover:translate-y-1 hover:shadow-none active:translate-x-2 active:translate-y-2"
                    >
                        <i className="fas fa-sign-in-alt mr-2"></i>
                        Sign In
                    </button>
                ) : null}
            </div>

            {/* Original App component */}
            <App />
        </div>
    );
}
