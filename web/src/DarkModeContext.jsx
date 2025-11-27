import { createContext, useContext, useState, useEffect } from "react";

const DarkModeContext = createContext();

export function DarkModeProvider({ children }) {
    const [darkMode, setDarkMode] = useState(false);

    // Load saved preference on mount
    useEffect(() => {
        try {
            const saved = localStorage.getItem("darkMode");
            if (saved !== null) {
                setDarkMode(saved === "true");
            } else {
                // If no saved preference, use system preference
                const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
                setDarkMode(prefersDark);
            }
        } catch (error) {
            console.warn("Could not load dark mode preference:", error);
        }
    }, []);

    // Listen for system theme changes
    useEffect(() => {
        const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
        const handleChange = (e) => {
            // Only update if user hasn't manually set a preference
            try {
                const userPref = localStorage.getItem("darkMode");
                if (userPref === null) {
                    setDarkMode(e.matches);
                }
            } catch (error) {
                console.warn("Could not check dark mode preference:", error);
            }
        };

        mediaQuery.addEventListener("change", handleChange);
        return () => mediaQuery.removeEventListener("change", handleChange);
    }, []);

    // Apply dark mode class to document
    useEffect(() => {
        if (darkMode) {
            document.documentElement.classList.add("dark");
        } else {
            document.documentElement.classList.remove("dark");
        }
    }, [darkMode]);

    const toggleDarkMode = () => {
        setDarkMode((prev) => {
            const newValue = !prev;
            try {
                localStorage.setItem("darkMode", String(newValue));
            } catch (error) {
                console.warn("Could not save dark mode preference:", error);
            }
            return newValue;
        });
    };

    return (
        <DarkModeContext.Provider value={{ darkMode, toggleDarkMode }}>
            {children}
        </DarkModeContext.Provider>
    );
}

export function useDarkMode() {
    const context = useContext(DarkModeContext);
    if (!context) {
        throw new Error("useDarkMode must be used within a DarkModeProvider");
    }
    return context;
}
