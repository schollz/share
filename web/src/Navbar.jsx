import { useNavigate } from "react-router-dom";
import { useAuth } from "./AuthContext";
import { useConfig } from "./ConfigContext";
import { useDarkMode } from "./DarkModeContext";

export default function Navbar({ title = "e2ecp", subtitle = null }) {
    const navigate = useNavigate();
    const { isAuthenticated, logout } = useAuth();
    const { storageEnabled } = useConfig();
    const { darkMode, toggleDarkMode } = useDarkMode();

    return (
        <div className="sticky top-0 z-50 bg-white dark:bg-black border-b-4 border-black dark:border-white">
            <div className="max-w-6xl mx-auto px-4 py-4 flex justify-between items-center">
                <div>
                    <h1 className="text-2xl sm:text-3xl font-black text-black dark:text-white">{title}</h1>
                    {subtitle && <p className="text-sm sm:text-base mt-1 text-black dark:text-white">{subtitle}</p>}
                </div>
                <div className="flex gap-2 sm:gap-3">
                    <button
                        onClick={() => navigate("/")}
                        className="border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 sm:px-4 text-sm sm:text-base font-bold uppercase hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black hover:scale-105 transition-all cursor-pointer"
                    >
                        <i className="fas fa-exchange-alt"></i>
                        <span className="hidden sm:inline sm:ml-2">Transfer</span>
                    </button>
                    <button
                        onClick={toggleDarkMode}
                        className="border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 sm:px-4 text-sm sm:text-base font-bold hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black hover:scale-105 transition-all cursor-pointer"
                        aria-label={darkMode ? "Switch to light mode" : "Switch to dark mode"}
                    >
                        <i className={`fas ${darkMode ? "fa-sun" : "fa-moon"}`}></i>
                    </button>
                    <button
                        onClick={() => navigate("/about")}
                        className="border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 sm:px-4 text-sm sm:text-base font-bold hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black hover:scale-105 transition-all cursor-pointer"
                        aria-label="About"
                    >
                        ?
                    </button>
                    {isAuthenticated ? (
                        <>
                            {storageEnabled && (
                                <button
                                    onClick={() => navigate("/storage")}
                                    className="border-2 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-3 py-2 sm:px-4 text-sm sm:text-base font-bold uppercase hover:bg-white hover:text-black dark:hover:bg-black dark:hover:text-white hover:scale-105 transition-all cursor-pointer"
                                >
                                    <i className="fas fa-hdd"></i>
                                    <span className="hidden sm:inline sm:ml-2">Storage</span>
                                </button>
                            )}
                            <button
                                onClick={logout}
                                className="border-2 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-3 py-2 sm:px-4 text-sm sm:text-base font-bold uppercase hover:bg-white hover:text-black dark:hover:bg-black dark:hover:text-white hover:scale-105 transition-all cursor-pointer"
                            >
                                <i className="fas fa-sign-out-alt"></i>
                                <span className="hidden sm:inline sm:ml-2">Logout</span>
                            </button>
                        </>
                    ) : (
                        <>
                            <button
                                onClick={() => navigate("/login?mode=login")}
                                className="border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 sm:px-4 text-sm sm:text-base font-bold uppercase hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black hover:scale-105 transition-all cursor-pointer"
                            >
                                Sign In
                            </button>
                            <button
                                onClick={() => navigate("/login?mode=signup")}
                                className="border-2 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-3 py-2 sm:px-4 text-sm sm:text-base font-bold uppercase hover:bg-white hover:text-black dark:hover:bg-black dark:hover:text-white hover:scale-105 transition-all cursor-pointer"
                            >
                                Sign Up
                            </button>
                        </>
                    )}
                </div>
            </div>
        </div>
    );
}
