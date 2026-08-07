import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { api, ApiError } from "@/lib/api";

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
      .catch((err) => {
        if (!(err instanceof ApiError) || err.status !== 401) {
          // Non-auth errors (e.g. 503 dashboard disabled) still leave us logged out.
        }
        setUsername(null);
      })
      .finally(() => setLoading(false));
  }, []);

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
