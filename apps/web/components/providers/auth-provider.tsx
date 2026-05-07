import { createContext, useContext } from "react";
import { type User } from "../../lib/auth";
import { useLogout, useMe } from "../../hooks/use-auth";

type AuthContextType = {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  logout: () => Promise<boolean>;
};

const AuthContext = createContext<AuthContextType | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const { data: user, isLoading } = useMe();
  const logout = useLogout();
  const value = { user, isLoading, isAuthenticated: user !== null, logout: logout.mutateAsync } satisfies AuthContextType;
  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);

  if (!ctx) {
    throw new Error("useAuth must be used inside AuthProvider");
  }

  return ctx;
}
