import React from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { Toaster } from "react-hot-toast";
import { AuthProvider } from "./AuthContext.jsx";
import Landing from "./Landing.jsx";
import Login from "./Login.jsx";
import Profile from "./Profile.jsx";
import SharedFile from "./SharedFile.jsx";
import "./index.css";
import "@fortawesome/fontawesome-free/css/all.min.css";

createRoot(document.getElementById("root")).render(
    <React.StrictMode>
        <BrowserRouter>
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
                    <Route path="/share/:token" element={<SharedFile />} />
                    <Route path="/:room" element={<Landing />} />
                </Routes>
            </AuthProvider>
        </BrowserRouter>
    </React.StrictMode>
);
