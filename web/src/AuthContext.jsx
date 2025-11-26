import React, { createContext, useContext, useState, useEffect } from 'react';

const AuthContext = createContext(null);

const API_BASE = '';

export function AuthProvider({ children }) {
    const [user, setUser] = useState(null);
    const [token, setToken] = useState(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        // Check for existing token on mount
        const savedToken = localStorage.getItem('e2ecp_token');
        const savedUser = localStorage.getItem('e2ecp_user');
        
        if (savedToken && savedUser) {
            // Verify token is still valid
            verifyToken(savedToken).then(valid => {
                if (valid) {
                    setToken(savedToken);
                    setUser(JSON.parse(savedUser));
                } else {
                    // Token expired or invalid, clear storage
                    localStorage.removeItem('e2ecp_token');
                    localStorage.removeItem('e2ecp_user');
                }
                setLoading(false);
            });
        } else {
            setLoading(false);
        }
    }, []);

    const verifyToken = async (tokenToVerify) => {
        try {
            const response = await fetch(`${API_BASE}/api/auth/verify`, {
                headers: {
                    'Authorization': `Bearer ${tokenToVerify}`
                }
            });
            return response.ok;
        } catch {
            return false;
        }
    };

    const register = async (email, password) => {
        const response = await fetch(`${API_BASE}/api/auth/register`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ email, password })
        });

        const data = await response.json();
        
        if (!response.ok) {
            throw new Error(data.error || 'Registration failed');
        }

        setToken(data.token);
        setUser(data.user);
        localStorage.setItem('e2ecp_token', data.token);
        localStorage.setItem('e2ecp_user', JSON.stringify(data.user));
        
        return data;
    };

    const login = async (email, password) => {
        const response = await fetch(`${API_BASE}/api/auth/login`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ email, password })
        });

        const data = await response.json();
        
        if (!response.ok) {
            throw new Error(data.error || 'Login failed');
        }

        setToken(data.token);
        setUser(data.user);
        localStorage.setItem('e2ecp_token', data.token);
        localStorage.setItem('e2ecp_user', JSON.stringify(data.user));
        
        return data;
    };

    const logout = () => {
        setToken(null);
        setUser(null);
        localStorage.removeItem('e2ecp_token');
        localStorage.removeItem('e2ecp_user');
    };

    const refreshProfile = async () => {
        if (!token) return;
        
        try {
            const response = await fetch(`${API_BASE}/api/profile`, {
                headers: {
                    'Authorization': `Bearer ${token}`
                }
            });
            
            if (response.ok) {
                const userData = await response.json();
                setUser(userData);
                localStorage.setItem('e2ecp_user', JSON.stringify(userData));
            }
        } catch (error) {
            console.error('Failed to refresh profile:', error);
        }
    };

    return (
        <AuthContext.Provider value={{
            user,
            token,
            loading,
            isAuthenticated: !!token,
            register,
            login,
            logout,
            refreshProfile
        }}>
            {children}
        </AuthContext.Provider>
    );
}

export function useAuth() {
    const context = useContext(AuthContext);
    if (!context) {
        throw new Error('useAuth must be used within an AuthProvider');
    }
    return context;
}
