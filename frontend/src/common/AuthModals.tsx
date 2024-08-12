import { useState } from "react";
import { useAuth } from "auth";
import modalsStyle from "./Modals.module.css";
import { GoogleLogin } from "@react-oauth/google";

export default function LoginModal({ close }: { close: () => void }) {
  const { supabase, session } = useAuth()
  const [phone, setPhone] = useState<string>('');
  const [error, setError] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);

  if (!supabase) {
    return null;
  }

  const handleOverlayClick = (e: any) => {
    if (e.target.id === "login-modal") {
      close();
    }
  };

  if (session) {
    close();
  }

  return (
    <div id="login-modal" className={modalsStyle.modal} onClick={handleOverlayClick}>
      <div className={modalsStyle.modal_content}>
        <span onClick={() => close()} className={modalsStyle.close} id="closeModalBtn">&times;</span>
        <h2>Login</h2>
        <div style={{ display: "flex", justifyContent: "center", marginTop:"40px" }}>
          <GoogleLogin
            onSuccess={credentialResponse => {
              if (credentialResponse.credential) {
                supabase.auth.signInWithIdToken({
                  provider: 'google',
                  token: credentialResponse.credential,
                })
                close()
              } else {
                alert("Failed to login with Google")
              }
            }}
            onError={() => {
              alert("Failed to login with Google")
            }}
          />
        </div>
        {/* <p>or</p>
        <label>phone number</label>
        <input />

        <p>or</p>
        <label>phone number</label>
        <input /> */}
      </div>
    </div>
  );
}
