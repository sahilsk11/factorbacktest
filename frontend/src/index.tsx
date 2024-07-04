import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Routes, Route } from "react-router-dom";

import './index.css';
import App from './App';
import { BondBuilder } from './Bond/Bond';
import { GoogleOAuthProvider } from '@react-oauth/google';

const root = ReactDOM.createRoot(
  document.getElementById('root') as HTMLElement
);

root.render(
  <React.StrictMode>
    <GoogleOAuthProvider clientId="553014490207-3s25moanhrdjeckdsvbu9ea5rdik0uh2.apps.googleusercontent.com">

      <BrowserRouter>
        <Routes>
          <Route index element={<App />} />
          <Route path="bonds" element={<BondBuilder user={null} setUser={() => {}} />} />
          <Route path="*" element={<p>not found</p>} />
        </Routes>
      </BrowserRouter>
    </GoogleOAuthProvider>
  </React.StrictMode >
);
