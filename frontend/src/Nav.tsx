import { GoogleAuthUser } from "./models";
import { googleLogout, useGoogleLogin } from '@react-oauth/google';
import Navbar from 'react-bootstrap/Navbar';
import Container from 'react-bootstrap/Container';
import BootstrapNav from 'react-bootstrap/Nav';
import NavDropdown from 'react-bootstrap/NavDropdown';
import styles from './Nav.module.css';


export function Nav({ setShowHelpModal, setShowContactModal, showLinks, setUser, loggedIn }: {
  showLinks: boolean;
  setShowHelpModal: React.Dispatch<React.SetStateAction<boolean>>;
  setShowContactModal: React.Dispatch<React.SetStateAction<boolean>>;
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>;
  loggedIn: boolean;
}) {
  const login = useGoogleLogin({
    onSuccess: (codeResponse) => {
      // console.log(codeResponse)
      const date = new Date();
      date.setTime(date.getTime() + (codeResponse.expires_in * 1000));
      const expires = "expires=" + date.toUTCString();

      document.cookie = "googleAuthAccessToken" + "=" + codeResponse.access_token + "; " + expires;

      setUser({
        accessToken: codeResponse.access_token
      } as GoogleAuthUser);
    },
    onError: (error) => console.log('Login Failed:', error)
  });

  const authTab = !loggedIn ? (
    <BootstrapNav.Link onClick={() => login()}>Login</BootstrapNav.Link>
  ) : (
    <NavDropdown title="Account" id="basic-nav-dropdown">
      {/* <NavDropdown.Item href="#action/3.1">Action</NavDropdown.Item>
      <NavDropdown.Item href="#action/3.2">
        Another action
      </NavDropdown.Item>
      <NavDropdown.Item href="#action/3.3">Something</NavDropdown.Item> */}
      {/* <NavDropdown.Divider /> */}
      <NavDropdown.Item onClick={() => {
        googleLogout();
        setUser(null);
        console.log("logout");
      }} style={{fontSize:"14px"}}>
        Logout
      </NavDropdown.Item>
    </NavDropdown>
  );

  return <>
    <Navbar data-bs-theme="dark" bg="dark" expand="sm" className={`${styles.nav} bg-body-tertiary `}>
      <Container>
        <Navbar.Brand href="/">factorbacktest.net</Navbar.Brand>
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
  </>;
}
