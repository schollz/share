import { useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "./AuthContext";
import { useConfig } from "./ConfigContext";
import { useDarkMode } from "./DarkModeContext";

export default function Navbar({ title = "e2ecp", subtitle = null }) {
    const navigate = useNavigate();
    const location = useLocation();
    const { isAuthenticated, logout } = useAuth();
    const { storageEnabled } = useConfig();
    const { darkMode, toggleDarkMode } = useDarkMode();

    const pathname = location.pathname;
    const searchParams = new URLSearchParams(location.search);
    const loginMode = searchParams.get("mode") || "login";

    const activeRing = "ring-4 ring-black dark:ring-white ring-offset-2 ring-offset-white dark:ring-offset-black";

    const isTransferActive = pathname === "/";
    const isAboutActive = pathname === "/about";
    const isStorageActive = pathname.startsWith("/storage");
    const isSettingsActive = pathname.startsWith("/settings");
    const isLoginPage = pathname === "/login";
    const isSignUpActive = isLoginPage && loginMode === "signup";
    const isSignInActive = isLoginPage && !isSignUpActive;

    return (
        <div className="sticky top-0 z-50 bg-white dark:bg-black border-b-4 border-black dark:border-white">
            <div className="max-w-6xl mx-auto px-4 py-4 flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3">
                <div className="hidden sm:block sm:flex-1 sm:w-auto">
                    <h1 className="text-2xl sm:text-3xl font-black text-black dark:text-white">{title}</h1>
                    {subtitle && <p className="text-sm sm:text-base mt-1 text-black dark:text-white">{subtitle}</p>}
                </div>
                <div className="flex w-full sm:w-auto sm:flex-none flex-wrap sm:flex-nowrap gap-2 sm:gap-3 justify-between sm:justify-end">
                    <button
                        onClick={() => navigate("/")}
                        className={`border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 sm:px-4 text-sm sm:text-base font-bold uppercase hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black hover:scale-105 transition-all cursor-pointer ${isTransferActive ? activeRing : ""}`}
                        aria-current={isTransferActive ? "page" : undefined}
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
                        className={`border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 sm:px-4 text-sm sm:text-base font-bold hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black hover:scale-105 transition-all cursor-pointer ${isAboutActive ? activeRing : ""}`}
                        aria-label="About"
                        aria-current={isAboutActive ? "page" : undefined}
                    >
                        ?
                    </button>
                    {isAuthenticated ? (
                        <>
                            {storageEnabled && (
                                <button
                                    onClick={() => navigate("/storage")}
                                    className={`border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 sm:px-4 text-sm sm:text-base font-bold uppercase hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black hover:scale-105 transition-all cursor-pointer ${isStorageActive ? activeRing : ""}`}
                                    aria-current={isStorageActive ? "page" : undefined}
                                >
                                    <i className="fas fa-hdd"></i>
                                </button>
                            )}
                            <button
                                onClick={() => navigate("/settings")}
                                className={`border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 sm:px-4 text-sm sm:text-base font-bold uppercase hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black hover:scale-105 transition-all cursor-pointer ${isSettingsActive ? activeRing : ""}`}
                                aria-current={isSettingsActive ? "page" : undefined}
                                aria-label="Settings"
                            >
                                <i className="fas fa-cog"></i>
                            </button>
                            <button
                                onClick={logout}
                                className="border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 sm:px-4 text-sm sm:text-base font-bold uppercase hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black hover:scale-105 transition-all cursor-pointer"
                                aria-label="Logout"
                            >
                                <i className="fas fa-sign-out-alt"></i>
                            </button>
                        </>
                    ) : (
                        <>
                            <button
                                onClick={() => navigate("/login?mode=login")}
                                className={`border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-3 py-2 sm:px-4 text-sm sm:text-base font-bold uppercase hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black hover:scale-105 transition-all cursor-pointer ${isSignInActive ? activeRing : ""}`}
                                aria-current={isSignInActive ? "page" : undefined}
                                aria-label="Sign In"
                            >
                                <i className="fas fa-sign-in-alt"></i>
                            </button>
                            <button
                                onClick={() => navigate("/login?mode=signup")}
                                className={`border-2 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-3 py-2 sm:px-4 text-sm sm:text-base font-bold uppercase hover:bg-white hover:text-black dark:hover:bg-black dark:hover:text-white hover:scale-105 transition-all cursor-pointer ${isSignUpActive ? activeRing : ""}`}
                                aria-current={isSignUpActive ? "page" : undefined}
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
