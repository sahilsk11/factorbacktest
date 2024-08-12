import { GoogleAuthUser } from "../models";
import { googleLogout, useGoogleLogin } from '@react-oauth/google';
import Navbar from 'react-bootstrap/Navbar';
import Container from 'react-bootstrap/Container';
import BootstrapNav from 'react-bootstrap/Nav';
import NavDropdown from 'react-bootstrap/NavDropdown';
import styles from './Nav.module.css';
import { useNavigate } from "react-router-dom";
import { useState } from "react";
import LoginModal from "./AuthModals";
import { useAuth } from "auth";


export function Nav({ setShowHelpModal, setShowContactModal, showLinks, setUser, loggedIn }: {
  showLinks: boolean;
  setShowHelpModal: React.Dispatch<React.SetStateAction<boolean>>;
  setShowContactModal: React.Dispatch<React.SetStateAction<boolean>>;
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>;
  loggedIn: boolean;
}) {
  const [showLoginModal, setShowLoginModal] = useState(false);

  const { supabase, session } = useAuth();

  const navigate = useNavigate()

  const authTab = !loggedIn && !session ? (
    <BootstrapNav.Link onClick={() => setShowLoginModal(true)}>Login</BootstrapNav.Link>
  ) : (
    <NavDropdown title="Account" id="basic-nav-dropdown">
      {/* <NavDropdown.Item href="#action/3.1">Action</NavDropdown.Item>
      <NavDropdown.Item href="#action/3.2">
        Another action
      </NavDropdown.Item>*/}
      <NavDropdown.Item onClick={() => navigate("/investments")} className={styles.nav_link}>
        Your Investments
      </NavDropdown.Item>
      <NavDropdown.Divider />
      <NavDropdown.Item onClick={() => {
        googleLogout();
        setUser(null);
        supabase?.auth.signOut()
        document.cookie = "googleAuthAccessToken=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/; SameSite=Strict; Secure";
      }} className={styles.nav_link}>
        Logout
      </NavDropdown.Item>
    </NavDropdown>
  );

  return <>
    <Navbar data-bs-theme="dark" bg="dark" expand="sm" className={`${styles.nav} bg-body-tertiary `}>
      <Container>
        <Navbar.Brand style={{ fontSize: "16px", fontWeight: "500", cursor: "pointer" }} onClick={() => navigate("/")}>factorbacktest.net</Navbar.Brand>
        <Navbar.Toggle aria-controls="basic-navbar-nav" />
        <Navbar.Collapse id="basic-navbar-nav">
          <BootstrapNav className="ms-auto">
            <BootstrapNav.Link onClick={() => setShowContactModal(true)}>Contact</BootstrapNav.Link>

            <BootstrapNav.Link onClick={() => setShowHelpModal(true)}>User Guide</BootstrapNav.Link>
            {authTab}

          </BootstrapNav>

        </Navbar.Collapse>
      </Container>
    </Navbar>
    {/* <div id="g_id_onload"
      data-client_id="553014490207-3s25moanhrdjeckdsvbu9ea5rdik0uh2.apps.googleusercontent.com"
      data-context="signin"
      data-ux_mode="popup"
      data-callback="loginWithGoogleHelper"
      data-auto_select="true"
      data-itp_support="true">
    </div> */}

    {/* <div className="g_id_signin"
      data-type="standard"
      data-shape="rectangular"
      data-theme="outline"
      data-text="signin_with"
      data-size="large"
      data-logo_alignment="left">
    </div> */}
    {showLoginModal ? <LoginModal close={() => setShowLoginModal(false)} /> : null}
  </>;
}
