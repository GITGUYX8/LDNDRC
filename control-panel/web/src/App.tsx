import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { useAuth } from "./auth";
import type { ReactNode } from "react";
import { LoginPage } from "./pages/Login";
import { LaunchPage } from "./pages/Launch";
import { AdminPage } from "./pages/Admin";
import { WorkspacePage } from "./pages/Workspace";
import { DesktopPage } from "./pages/Desktop";

function RequireAuth({ children }: { children: ReactNode }) {
  const { token } = useAuth();
  const location = useLocation();
  if (!token) {
    return (
      <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />
    );
  }
  return <>{children}</>;
}

export function App() {
  const { token } = useAuth();
  return (
    <Routes>
      <Route
        path="/"
        element={
          <Navigate to={token ? "/launch" : "/login"} replace />
        }
      />
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/launch"
        element={
          <RequireAuth>
            <LaunchPage />
          </RequireAuth>
        }
      />
      <Route
        path="/app"
        element={
          <RequireAuth>
            <WorkspacePage />
          </RequireAuth>
        }
      />
      <Route
        path="/desktop"
        element={
          <RequireAuth>
            <DesktopPage />
          </RequireAuth>
        }
      />
      <Route
        path="/admin"
        element={
          <RequireAuth>
            <AdminPage />
          </RequireAuth>
        }
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
