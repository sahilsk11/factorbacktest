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
import { ContactModal, HelpModal } from "./Modals";


export function Nav({ showLinks, setUser, loggedIn }: {
  showLinks: boolean;
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>;
  loggedIn: boolean;
}) {
  const [showLoginModal, setShowLoginModal] = useState(false);
  const [showContactModal, setShowContactModal] = useState(false);
  const [showHelpModal, setShowHelpModal] = useState(false);

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

    {showLoginModal ? <LoginModal close={() => setShowLoginModal(false)} /> : null}
    {showContactModal ? <ContactModal show={showContactModal} close={() => setShowContactModal(false)} /> : null}
    {showHelpModal ? <HelpModal show={showHelpModal} close={() => setShowHelpModal(false)} /> : null}
  </>;
}
