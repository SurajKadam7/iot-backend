import { useEffect, useState } from "react";
import { Navigate, Route, Routes, useNavigate } from "react-router-dom";
import { api, getToken, setToken } from "./api";
import { AppShell } from "./components/AppShell";
import { InternalShell } from "./components/InternalShell";
import { Dashboard } from "./pages/Dashboard";
import { Devices } from "./pages/Devices";
import { Locations } from "./pages/Locations";
import { Login } from "./pages/Login";
import { Organizations } from "./pages/Organizations";
import { Users } from "./pages/Users";
import type { Me } from "./types";

function homePath(me: Me | null): string {
  if (!me) return "/login";
  if (me.role === "platform_admin") return "/internal";
  return "/";
}

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
      navigate(homePath(profile));
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
      <div className="grid min-h-screen place-items-center">
        <span className="loading loading-spinner loading-lg text-primary" aria-label="Loading" />
      </div>
    );
  }

  const client = me && me.role !== "platform_admin";
  const operator = me?.role === "platform_admin";

  return (
    <Routes>
      <Route
        path="/login"
        element={me ? <Navigate to={homePath(me)} replace /> : <Login onSubmit={onLogin} error={error} />}
      />
      <Route
        element={
          operator && me ? (
            <InternalShell me={me} onLogout={logout} />
          ) : (
            <Navigate to={homePath(me)} replace />
          )
        }
      >
        <Route path="/internal" element={<Organizations />} />
      </Route>
      <Route
        element={
          client ? (
            <AppShell me={me} onLogout={logout} />
          ) : (
            <Navigate to={homePath(me)} replace />
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
        <Route
          path="/users"
          element={me?.role === "org_admin" ? <Users /> : <Navigate to="/" replace />}
        />
      </Route>
      <Route path="*" element={<Navigate to={homePath(me)} replace />} />
    </Routes>
  );
}

export type LoginHandler = (email: string, password: string) => Promise<void> | void;
