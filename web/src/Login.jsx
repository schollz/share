import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "./AuthContext";
import toast from "react-hot-toast";
import { useConfig } from "./ConfigContext";

export default function Login() {
    const [isLogin, setIsLogin] = useState(true);
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const [loading, setLoading] = useState(false);
    const [errorMessage, setErrorMessage] = useState("");
    const [captchaPrompt, setCaptchaPrompt] = useState("");
    const [captchaToken, setCaptchaToken] = useState("");
    const [captchaAnswer, setCaptchaAnswer] = useState("");
    const [captchaLoading, setCaptchaLoading] = useState(false);
    const { login, register, isAuthenticated, loading: authLoading } =
        useAuth();
    const { storageEnabled, loading: configLoading, freeLimitBytes } = useConfig();
    const navigate = useNavigate();
    const humanizeBytes = (bytes) => {
        if (!bytes || bytes <= 0) return "0 B";
        const units = ["B", "KB", "MB", "GB", "TB"];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        const value = bytes / Math.pow(1024, i);
        const rounded = value >= 10 ? value.toFixed(0) : value.toFixed(1);
        return `${rounded} ${units[i]}`;
    };

    // If already signed in, bounce to profile
    React.useEffect(() => {
        if (!configLoading && !storageEnabled) {
            navigate("/", { replace: true });
        } else if (!authLoading && isAuthenticated) {
            navigate("/profile", { replace: true });
        }
    }, [authLoading, isAuthenticated, navigate, storageEnabled, configLoading]);

    // Clear error when switching mode
    React.useEffect(() => {
        setErrorMessage("");
    }, [isLogin]);

    const loadCaptcha = React.useCallback(async () => {
        setCaptchaLoading(true);
        try {
            const response = await fetch("/api/auth/captcha");
            if (!response.ok) {
                throw new Error("Failed to load captcha");
            }
            const data = await response.json();
            setCaptchaPrompt(data.prompt || "");
            setCaptchaToken(data.token || "");
            setCaptchaAnswer("");
        } catch (error) {
            const message = error.message || "Could not load captcha";
            toast.error(message);
            setErrorMessage(message);
        } finally {
            setCaptchaLoading(false);
        }
    }, []);

    React.useEffect(() => {
        if (!isLogin) {
            loadCaptcha();
        }
    }, [isLogin, loadCaptcha]);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setLoading(true);
        setErrorMessage("");

        try {
            if (isLogin) {
                await login(email, password);
                toast.success("Logged in successfully!");
                navigate("/profile");
            } else {
                if (!captchaToken || !captchaAnswer.trim()) {
                    setErrorMessage("Please solve the captcha to continue.");
                    setLoading(false);
                    return;
                }

                await register(email, password, captchaToken, captchaAnswer);
                navigate("/signup-success", { state: { email } });
            }
        } catch (error) {
            const message = error.message || "Something went wrong";
            toast.error(message);
            setErrorMessage(message);

            // If the email already exists, switch to sign in to guide the user
            if (message.toLowerCase().includes("exists")) {
                setIsLogin(true);
            }
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen bg-white dark:bg-black text-black dark:text-white flex items-center justify-center p-4">
            <div className="w-full max-w-md">
                <div className="mb-8 text-center">
                    <h1 className="text-4xl sm:text-5xl font-black uppercase mb-2">
                        {isLogin ? "Sign In" : "Sign Up"}
                    </h1>
                    <p className="text-lg">
                        Secure file storage with {humanizeBytes(freeLimitBytes || 2 * 1024 * 1024 * 1024)} free space
                    </p>
                </div>

                <form
                    onSubmit={handleSubmit}
                    className="border-4 border-black dark:border-white p-6 sm:p-8 bg-white dark:bg-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)]"
                >
                    {errorMessage && (
                        <div className="mb-4 border-2 border-red-500 text-red-700 dark:text-red-300 px-4 py-3 bg-red-50 dark:bg-red-900/30">
                            <p className="font-semibold">{errorMessage}</p>
                            {!isLogin && (
                                <button
                                    type="button"
                                    className="mt-2 underline font-bold"
                                    onClick={() => setIsLogin(true)}
                                >
                                    Email exists — try signing in
                                </button>
                            )}
                        </div>
                    )}

                    <div className="mb-6">
                        <label
                            htmlFor="email"
                            className="block text-lg font-bold mb-2 uppercase"
                        >
                            Email
                        </label>
                        <input
                            type="email"
                            id="email"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            required
                            className="w-full border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-4 py-3 text-base focus:outline-none focus:ring-2 focus:ring-black dark:focus:ring-white"
                            placeholder="you@example.com"
                        />
                    </div>

                    <div className="mb-6">
                        <label
                            htmlFor="password"
                            className="block text-lg font-bold mb-2 uppercase"
                        >
                            Password
                        </label>
                        <input
                            type="password"
                            id="password"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            required
                            minLength={6}
                            className="w-full border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-4 py-3 text-base focus:outline-none focus:ring-2 focus:ring-black dark:focus:ring-white"
                            placeholder="••••••••"
                        />
                        {!isLogin && (
                            <p className="text-sm mt-1">
                                Minimum 6 characters
                            </p>
                        )}
                    </div>

                    {!isLogin && (
                        <div className="mb-6">
                            <div className="flex items-center justify-between mb-2">
                                <label className="block text-lg font-bold uppercase">
                                    Captcha
                                </label>
                                <button
                                    type="button"
                                    onClick={loadCaptcha}
                                    disabled={captchaLoading}
                                    className="text-sm font-bold underline disabled:opacity-60"
                                >
                                    {captchaLoading ? "Refreshing..." : "New question"}
                                </button>
                            </div>
                            <p className="text-sm mb-3">
                                {captchaPrompt || "Loading challenge..."}
                            </p>
                            <input
                                type="text"
                                inputMode="numeric"
                                pattern="[0-9]*"
                                value={captchaAnswer}
                                onChange={(e) => setCaptchaAnswer(e.target.value)}
                                required={!isLogin}
                                className="w-full border-2 border-black dark:border-white bg-white dark:bg-black text-black dark:text-white px-4 py-3 text-base focus:outline-none focus:ring-2 focus:ring-black dark:focus:ring-white"
                                placeholder="Enter your answer"
                            />
                        </div>
                    )}

                    <button
                        type="submit"
                        disabled={loading}
                        className="w-full border-2 sm:border-4 border-black dark:border-white bg-black dark:bg-white text-white dark:text-black px-4 py-3 sm:py-4 text-base sm:text-lg font-black uppercase hover:bg-gray-900 dark:hover:bg-gray-300 transition-colors cursor-pointer shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,1)] hover:translate-x-1 hover:translate-y-1 hover:shadow-none active:translate-x-2 active:translate-y-2 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:translate-x-0 disabled:hover:translate-y-0"
                    >
                        {loading
                            ? "Processing..."
                            : isLogin
                              ? "Sign In"
                              : "Sign Up"}
                    </button>

                    <div className="mt-6 text-center">
                        <button
                            type="button"
                            onClick={() => {
                                setIsLogin(!isLogin);
                                setErrorMessage("");
                            }}
                            className="text-base font-bold hover:underline"
                        >
                            {isLogin
                                ? "Need an account? Sign up"
                                : "Have an account? Sign in"}
                        </button>
                    </div>

                    <div className="mt-4 text-center">
                        <button
                            type="button"
                            onClick={() => navigate("/")}
                            className="text-base hover:underline"
                        >
                            Back to Home
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
}
