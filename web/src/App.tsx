import { useEffect, useState } from "react";
import { Navigate, Route, Routes, useNavigate } from "react-router-dom";
import { api, getToken, setToken } from "./api";
import { AppShell } from "./components/AppShell";
import { Dashboard } from "./pages/Dashboard";
import { Devices } from "./pages/Devices";
import { Locations } from "./pages/Locations";
import { Login } from "./pages/Login";
import type { Me } from "./types";

export function App() {
  const [me, setMe] = useState<Me | null>(null);
  const [boot, setBoot] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    const token = getToken();
    if (!token) {
      setBoot(false);
      return;
    }
    api
      .me()
      .then(setMe)
      .catch(() => setToken(null))
      .finally(() => setBoot(false));
  }, []);

  async function onLogin(email: string, password: string) {
    setError(null);
    const mode = (import.meta.env.VITE_AUTH_MODE || "local").toLowerCase();
    try {
      if (mode === "cognito") {
        const token = await api.loginCognito(email, password);
        setToken(token);
      } else {
        const { token } = await api.loginLocal(email);
        setToken(token);
      }
      const profile = await api.me();
      setMe(profile);
      navigate("/");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Login failed");
    }
  }

  function logout() {
    setToken(null);
    setMe(null);
    navigate("/login");
  }

  if (boot) {
    return (
      <div className="boot">
        <div className="spinner" aria-label="Loading" />
      </div>
    );
  }

  return (
    <Routes>
      <Route
        path="/login"
        element={me ? <Navigate to="/" replace /> : <Login onSubmit={onLogin} error={error} />}
      />
      <Route
        element={
          me ? (
            <AppShell me={me} onLogout={logout} />
          ) : (
            <Navigate to="/login" replace />
          )
        }
      >
        <Route path="/" element={<Dashboard />} />
        <Route
          path="/devices"
          element={me?.role === "org_admin" ? <Devices /> : <Navigate to="/" replace />}
        />
        <Route
          path="/locations"
          element={me?.role === "org_admin" ? <Locations /> : <Navigate to="/" replace />}
        />
      </Route>
      <Route path="*" element={<Navigate to={me ? "/" : "/login"} replace />} />
    </Routes>
  );
}

export type LoginHandler = (email: string, password: string) => Promise<void> | void;
