import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { onUnauthorized } from "@/lib/auth-events";

interface AuthContextValue {
  username: string | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [username, setUsername] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .session()
      .then((s) => setUsername(s.username))
      .catch(() => {
        // Any failure (401 not-logged-in, or 503 dashboard disabled) leaves us
        // logged out; the router sends the user to the login page.
        setUsername(null);
      })
      .finally(() => setLoading(false));
  }, []);

  // When any authenticated request returns 401 (session expired/revoked), clear
  // auth so the router redirects to login instead of hanging on error states.
  useEffect(() => onUnauthorized(() => setUsername(null)), []);

  const login = useCallback(async (u: string, p: string) => {
    const res = await api.login(u, p);
    setUsername(res.username);
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.logout();
    } finally {
      setUsername(null);
    }
  }, []);

  return (
    <AuthContext.Provider value={{ username, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
