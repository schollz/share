import React, { createContext, useContext, useEffect, useState } from "react";

const ConfigContext = createContext(null);

export const useConfig = () => {
    const ctx = useContext(ConfigContext);
    if (!ctx) {
        throw new Error("useConfig must be used within a ConfigProvider");
    }
    return ctx;
};

export const ConfigProvider = ({ children }) => {
    const [storageEnabled, setStorageEnabled] = useState(false);
    const [loading, setLoading] = useState(true);
    const [freeLimitBytes, setFreeLimitBytes] = useState(null);

    useEffect(() => {
        const loadConfig = async () => {
            try {
                const res = await fetch("/api/config");
                if (!res.ok) throw new Error("Failed to load config");
                const data = await res.json();
                setStorageEnabled(Boolean(data.storage_profile_enabled));
                if (typeof data.free_storage_limit === "number") {
                    setFreeLimitBytes(data.free_storage_limit);
                }
            } catch (error) {
                setStorageEnabled(false);
            } finally {
                setLoading(false);
            }
        };
        loadConfig();
    }, []);

    return (
        <ConfigContext.Provider value={{ storageEnabled, loading, freeLimitBytes }}>
            {children}
        </ConfigContext.Provider>
    );
};
