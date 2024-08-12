import React, { useEffect, useState } from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Routes, Route } from "react-router-dom";
import 'bootstrap/dist/css/bootstrap.min.css';
import './index.css';
import App, { getCookie } from './App';
import { BondBuilder } from './pages/Bond/Bond';
import { GoogleOAuthProvider } from '@react-oauth/google';
import { GoogleAuthUser } from './models';
import Invest from './pages/Investments/Invest';
import { Home } from 'pages/Home/Home';
import AuthProvider from 'auth';

const root = ReactDOM.createRoot(
  document.getElementById('root') as HTMLElement
);



const AppWrapper = () => {
  const [user, setUser] = useState<GoogleAuthUser | null>(null);


  const app = <App user={user} setUser={setUser} />;

  return (
    <GoogleOAuthProvider clientId="553014490207-3s25moanhrdjeckdsvbu9ea5rdik0uh2.apps.googleusercontent.com">
      <AuthProvider>
        <BrowserRouter>
          <Routes>
            <Route index element={<Home user={user} setUser={setUser} />} />
            <Route path="backtest" element={app} />
            <Route path="bonds" element={<BondBuilder user={user} setUser={setUser} />} />
            <Route path="investments" element={<Invest user={user} setUser={setUser} />} />
            <Route path="*" element={<p>not found</p>} />
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </GoogleOAuthProvider>
  );
}

root.render(
  // <React.StrictMode>
  <AppWrapper />
  // </React.StrictMode >
);
