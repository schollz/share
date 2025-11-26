import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useAuth } from './AuthContext';
import toast from 'react-hot-toast';

export default function AuthPage() {
    const [isLogin, setIsLogin] = useState(true);
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [confirmPassword, setConfirmPassword] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');
    
    const { login, register } = useAuth();
    const navigate = useNavigate();

    const handleSubmit = async (e) => {
        e.preventDefault();
        setError('');
        
        if (!email || !password) {
            setError('Email and password are required');
            return;
        }

        if (!isLogin && password !== confirmPassword) {
            setError('Passwords do not match');
            return;
        }

        if (!isLogin && password.length < 6) {
            setError('Password must be at least 6 characters');
            return;
        }

        setLoading(true);

        try {
            if (isLogin) {
                await login(email, password);
                toast.success('Welcome back!');
            } else {
                await register(email, password);
                toast.success('Account created!');
            }
            navigate('/profile');
        } catch (err) {
            setError(err.message);
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen bg-white dark:bg-black p-4 sm:p-8 font-mono flex flex-col items-center justify-center transition-colors duration-200">
            <div className="max-w-md w-full">
                {/* Header */}
                <div className="bg-black dark:bg-black text-white border-4 sm:border-8 border-black dark:border-white p-4 sm:p-6 mb-4 sm:mb-6 header-shadow transition-colors duration-200"
                    style={{
                        clipPath: "polygon(0 0, calc(100% - 20px) 0, 100% 20px, 100% 100%, 0 100%)"
                    }}>
                    <h1 className="text-3xl sm:text-4xl font-black uppercase tracking-tight mb-2">
                        <Link to="/" className="text-white no-underline hover:underline">e2ecp</Link>
                    </h1>
                    <p className="text-sm sm:text-base font-bold">
                        {isLogin ? 'SIGN IN TO YOUR ACCOUNT' : 'CREATE A NEW ACCOUNT'}
                    </p>
                </div>

                {/* Auth Form */}
                <div className="bg-gray-200 dark:bg-black border-4 sm:border-8 border-black dark:border-white p-4 sm:p-6 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,1)] sm:shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] dark:sm:shadow-[8px_8px_0px_0px_rgba(255,255,255,1)] transition-colors duration-200">
                    
                    {error && (
                        <div className="bg-red-600 text-white border-2 sm:border-4 border-black p-3 mb-4 font-bold text-sm uppercase">
                            {error}
                        </div>
                    )}

                    <form onSubmit={handleSubmit}>
                        <div className="mb-4">
                            <label className="block text-sm font-black uppercase mb-2 text-black dark:text-white">
                                Email
                            </label>
                            <input
                                type="email"
                                value={email}
                                onChange={(e) => setEmail(e.target.value)}
                                className="w-full border-2 sm:border-4 border-black dark:border-white p-3 text-base font-bold bg-white dark:bg-black dark:text-white focus:outline-hidden focus:ring-4 focus:ring-black dark:focus:ring-white transition-colors duration-200"
                                placeholder="YOUR@EMAIL.COM"
                                disabled={loading}
                            />
                        </div>

                        <div className="mb-4">
                            <label className="block text-sm font-black uppercase mb-2 text-black dark:text-white">
                                Password
                            </label>
                            <input
                                type="password"
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                className="w-full border-2 sm:border-4 border-black dark:border-white p-3 text-base font-bold bg-white dark:bg-black dark:text-white focus:outline-hidden focus:ring-4 focus:ring-black dark:focus:ring-white transition-colors duration-200"
                                placeholder="••••••••"
                                disabled={loading}
                            />
                        </div>

                        {!isLogin && (
                            <div className="mb-4">
                                <label className="block text-sm font-black uppercase mb-2 text-black dark:text-white">
                                    Confirm Password
                                </label>
                                <input
                                    type="password"
                                    value={confirmPassword}
                                    onChange={(e) => setConfirmPassword(e.target.value)}
                                    className="w-full border-2 sm:border-4 border-black dark:border-white p-3 text-base font-bold bg-white dark:bg-black dark:text-white focus:outline-hidden focus:ring-4 focus:ring-black dark:focus:ring-white transition-colors duration-200"
                                    placeholder="••••••••"
                                    disabled={loading}
                                />
                            </div>
                        )}

                        <button
                            type="submit"
                            disabled={loading}
                            className={`w-full border-2 sm:border-4 border-black dark:border-white px-6 py-3 text-base sm:text-lg font-black uppercase transition-all ${
                                loading
                                    ? 'bg-gray-400 dark:bg-gray-500 cursor-not-allowed'
                                    : 'bg-black dark:bg-white text-white dark:text-black hover:translate-x-1 hover:translate-y-1 hover:shadow-none active:translate-x-2 active:translate-y-2 cursor-pointer'
                            } shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,1)]`}
                        >
                            {loading ? 'LOADING...' : (isLogin ? 'SIGN IN' : 'CREATE ACCOUNT')}
                        </button>
                    </form>

                    <div className="mt-4 text-center">
                        <button
                            onClick={() => {
                                setIsLogin(!isLogin);
                                setError('');
                            }}
                            className="text-black dark:text-white font-bold underline hover:no-underline cursor-pointer bg-transparent border-none"
                        >
                            {isLogin ? 'NEED AN ACCOUNT? REGISTER' : 'ALREADY HAVE AN ACCOUNT? SIGN IN'}
                        </button>
                    </div>

                    <div className="mt-4 text-center">
                        <Link
                            to="/"
                            className="text-black dark:text-white font-bold underline hover:no-underline"
                        >
                            ← BACK TO HOME
                        </Link>
                    </div>
                </div>

                {/* Info box */}
                <div className="mt-4 bg-white dark:bg-black border-2 sm:border-4 border-black dark:border-white p-4 text-black dark:text-white transition-colors duration-200">
                    <p className="text-sm font-bold text-center">
                        Get 2GB of free storage for your files
                    </p>
                </div>
            </div>
        </div>
    );
}
