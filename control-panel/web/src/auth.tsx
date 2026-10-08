import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { login as apiLogin } from "./api";

interface AuthState {
  token: string;
  csrf: string;
  username: string;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthState | null>(null);

const TOKEN_KEY = "ldndrc.token";
const CSRF_KEY = "ldndrc.csrf";
const USER_KEY = "ldndrc.user";

export function AuthProvider({ children }: { children: ReactNode }) {
  // Token lives in state (memory-first per R3) with a sessionStorage mirror
  // so a reload does not force re-login; CSRF mirrors the same way.
  const [token, setToken] = useState(() => sessionStorage.getItem(TOKEN_KEY) ?? "");
  const [csrf, setCsrf] = useState(() => sessionStorage.getItem(CSRF_KEY) ?? "");
  const [username, setUsername] = useState(
    () => sessionStorage.getItem(USER_KEY) ?? "",
  );

  const login = useCallback(async (user: string, password: string) => {
    const { token: t, csrf: c } = await apiLogin(user, password);
    sessionStorage.setItem(TOKEN_KEY, t);
    sessionStorage.setItem(CSRF_KEY, c);
    sessionStorage.setItem(USER_KEY, user);
    setToken(t);
    setCsrf(c);
    setUsername(user);
  }, []);

  const logout = useCallback(() => {
    sessionStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(CSRF_KEY);
    sessionStorage.removeItem(USER_KEY);
    setToken("");
    setCsrf("");
    setUsername("");
  }, []);

  // Cross-tab sign-out stays consistent.
  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key === TOKEN_KEY && !e.newValue) logout();
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, [logout]);

  const value = useMemo(
    () => ({ token, csrf, username, login, logout }),
    [token, csrf, username, login, logout],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth outside AuthProvider");
  return ctx;
}
