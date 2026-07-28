import { createContext, useCallback, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { getAccessToken, getStoredUser, setStoredUser, setTokens, clearSession } from "@/lib/tokens";
import type { AuthResponse, User } from "@/lib/types";

interface AuthContextValue {
  user: User | null;
  isAuthenticated: boolean;
  login: (authResponse: AuthResponse) => void;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(getStoredUser);

  const login = useCallback((authResponse: AuthResponse) => {
    setTokens(authResponse);
    setStoredUser(authResponse.user);
    setUser(authResponse.user);
  }, []);

  const logout = useCallback(() => {
    clearSession();
    setUser(null);
  }, []);

  // The API client dispatches this when a refresh attempt fails, so the
  // whole app can react (e.g. redirect to /login) without api.js knowing
  // anything about React.
  useEffect(() => {
    window.addEventListener("auth:unauthorized", logout);
    return () => window.removeEventListener("auth:unauthorized", logout);
  }, [logout]);

  const isAuthenticated = Boolean(user && getAccessToken());

  return (
    <AuthContext.Provider value={{ user, isAuthenticated, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
