import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import './App.css'
import { AuthProvider } from './auth/AuthContext'
import { ProtectedRoute } from './auth/ProtectedRoute'
import { SessionExpiredListener } from './auth/SessionExpiredListener'
import { Layout } from './components/Layout'
import { ConfirmDialogProvider } from './components/ConfirmDialogProvider'
import { SnackbarProvider } from './components/SnackbarProvider'
import { LoginPage } from './pages/LoginPage'
import { ResetPasswordPage } from './pages/ResetPasswordPage'
import { BoardPage } from './pages/BoardPage'
import { ExceptionsPage } from './pages/ExceptionsPage'
import { UnitDetailPage } from './pages/UnitDetailPage'
import { PublicSignPage } from './pages/PublicSignPage'
import { ReportsPage } from './pages/ReportsPage'
import { AdminPage } from './pages/AdminPage'
import { AuditLogPage } from './pages/AuditLogPage'
import { LogisticsPage } from './pages/LogisticsPage'
import { DocketDetailPage } from './pages/DocketDetailPage'
import { useGlobalUppercaseInputs } from './hooks/useGlobalUppercaseInputs'

function App() {
  useGlobalUppercaseInputs()

  return (
    <AuthProvider>
      <SnackbarProvider>
        <ConfirmDialogProvider>
          <BrowserRouter>
            <SessionExpiredListener />
            <Routes>
              <Route path="/login" element={<LoginPage />} />
              <Route path="/reset-password/:token" element={<ResetPasswordPage />} />
              <Route path="/sign/:token" element={<PublicSignPage />} />
              <Route element={<ProtectedRoute />}>
                <Route element={<Layout />}>
                  <Route path="/board" element={<BoardPage />} />
                  <Route path="/exceptions" element={<ExceptionsPage />} />
                  <Route path="/units/:id" element={<UnitDetailPage />} />
                  <Route path="/logistics" element={<LogisticsPage />} />
                  <Route path="/logistics/:id" element={<DocketDetailPage />} />
                  <Route path="/reports" element={<ReportsPage />} />
                  <Route path="/admin" element={<AdminPage />} />
                  <Route path="/audit-log" element={<AuditLogPage />} />
                </Route>
              </Route>
              <Route path="*" element={<Navigate to="/board" replace />} />
            </Routes>
          </BrowserRouter>
        </ConfirmDialogProvider>
      </SnackbarProvider>
    </AuthProvider>
  )
}

export default App
