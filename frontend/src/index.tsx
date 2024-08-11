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
import { getAuth } from "@firebase/auth";
import { initializeApp } from '@firebase/app';

export const auth = getAuth(initializeApp({
  apiKey: "",
  authDomain: "factor-backtest.firebaseapp.com",
}))

const root = ReactDOM.createRoot(
  document.getElementById('root') as HTMLElement
);

async function isValidUser(user: GoogleAuthUser) {
  const url = "https://www.googleapis.com/oauth2/v1/userinfo?access_token=" + user.accessToken;

  try {
    const response = await fetch(url);

    // Check if the response is OK (status code 200)
    if (response.ok) {
      return true;
    } else {
      return false;
    }
  } catch (error) {
    console.error("Error checking access token:", error);
    return false;
  }
}

const AppWrapper = () => {
  const [user, setUser] = useState<GoogleAuthUser | null>(null);

  async function updateUserFromCookie() {
    const accessToken = getCookie("googleAuthAccessToken");
    if (accessToken) {
      const tmpUser = {
        accessToken
      } as GoogleAuthUser;
      if (await isValidUser(tmpUser)) {
        setUser(tmpUser);
      }
    }
  }

  useEffect(() => {
    updateUserFromCookie()
  }, []);

  const app = <App user={user} setUser={setUser} />;

  return (
    <GoogleOAuthProvider clientId="553014490207-3s25moanhrdjeckdsvbu9ea5rdik0uh2.apps.googleusercontent.com">
      <BrowserRouter>
        <Routes>
          <Route index element={<Home user={user} setUser={setUser} />} />
          <Route path="backtest" element={app} />
          <Route path="bonds" element={<BondBuilder user={user} setUser={setUser} />} />
          <Route path="investments" element={<Invest user={user} setUser={setUser} />} />
          <Route path="*" element={<p>not found</p>} />
        </Routes>
      </BrowserRouter>
    </GoogleOAuthProvider>
  );
}

root.render(
  // <React.StrictMode>
  <AppWrapper />
  // </React.StrictMode >
);
