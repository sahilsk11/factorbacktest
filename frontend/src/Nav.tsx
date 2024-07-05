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
    <p onClick={() => login()} className='nav-element-text'>Login</p>
  ) : (
    <p onClick={() => {
      googleLogout();
      setUser(null);
      console.log("logout");
    }} className='nav-element-text'>Logout</p>
  );

  return <>
    <Navbar data-bs-theme="dark" bg="dark" expand="md" className={`bg-body-tertiary ${styles.nav}`}>
      <Container>
        <Navbar.Brand href="#home">factorbacktest.net</Navbar.Brand>
        <Navbar.Toggle aria-controls="basic-navbar-nav" />
        <Navbar.Collapse id="basic-navbar-nav">
          <BootstrapNav className="ms-auto">
            <BootstrapNav.Link href="#home">Home</BootstrapNav.Link>
            <BootstrapNav.Link href="#link">Link</BootstrapNav.Link>
            <NavDropdown title="Dropdown" id="basic-nav-dropdown">
              <NavDropdown.Item href="#action/3.1">Action</NavDropdown.Item>
              <NavDropdown.Item href="#action/3.2">
                Another action
              </NavDropdown.Item>
              <NavDropdown.Item href="#action/3.3">Something</NavDropdown.Item>
              <NavDropdown.Divider />
              <NavDropdown.Item href="#action/3.4">
                Separated link
              </NavDropdown.Item>
            </NavDropdown>
          </BootstrapNav>
        </Navbar.Collapse>
        {/* <Navbar.Brand href="/">factorbacktest.net</Navbar.Brand> */}

        {/* {showLinks ?
          <div className='nav-element-container'>
            <div className='nav-element-wrapper'>
              <p onClick={() => setShowContactModal(true)} className='nav-element-text'>Contact</p>
            </div>
            <div className='nav-element-wrapper'>
              <p onClick={() => setShowHelpModal(true)} className='nav-element-text'>User Guide</p>
            </div>
            <div style={{ width: "70px" }} className='nav-element-wrapper'>
              {authTab}
            </div>
          </div>
          : null} */}
      </Container>
    </Navbar>
  </>;
}
