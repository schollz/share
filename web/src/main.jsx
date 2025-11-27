import React from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { Toaster } from "react-hot-toast";
import { AuthProvider } from "./AuthContext.jsx";
import { ConfigProvider } from "./ConfigContext.jsx";
import Landing from "./Landing.jsx";
import Login from "./Login.jsx";
import Profile from "./Profile.jsx";
import SharedFile from "./SharedFile.jsx";
import Settings from "./Settings.jsx";
import VerifyEmail from "./VerifyEmail.jsx";
import SignupSuccess from "./SignupSuccess.jsx";
import DeviceAuth from "./DeviceAuth.jsx";
import "@fontsource/monaspace-neon";
import "./index.css";
import "@fortawesome/fontawesome-free/css/all.min.css";

createRoot(document.getElementById("root")).render(
    <React.StrictMode>
        <BrowserRouter>
            <ConfigProvider>
                <AuthProvider>
                    <Toaster
                        position="bottom-center"
                        toastOptions={{
                            duration: 3000,
                            style: {
                                background: "#000",
                                color: "#fff",
                                border: "2px solid #fff",
                                fontWeight: "bold",
                            },
                        }}
                    />
                    <Routes>
                        <Route path="/" element={<Landing />} />
                        <Route path="/login" element={<Login />} />
                        <Route path="/profile" element={<Profile />} />
                        <Route path="/settings" element={<Settings />} />
                        <Route path="/verify-email" element={<VerifyEmail />} />
                        <Route path="/signup-success" element={<SignupSuccess />} />
                        <Route path="/device-auth" element={<DeviceAuth />} />
                        <Route path="/share/:token" element={<SharedFile />} />
                        <Route path="/:room" element={<Landing />} />
                    </Routes>
                </AuthProvider>
            </ConfigProvider>
        </BrowserRouter>
    </React.StrictMode>
);
