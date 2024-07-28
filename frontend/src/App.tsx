import { useEffect, useState } from 'react';
import {  BacktestSnapshot, GoogleAuthUser } from "./models";
import { v4 as uuidv4 } from 'uuid';
import { ContactModal, HelpModal } from './common/Modals';
import StatsFooter from './common/Footer';
import { Nav } from './common/Nav';
import styles from './App.module.css'
import FactorBacktestMain from 'pages/Backtest/Backtest';

// we should move all contents from here to FactorBacktest or index

export interface FactorData {
  name: string,
  expression: string,
  // options
  data: Record<string, BacktestSnapshot>,
}

export const endpoint = (process.env.NODE_ENV === 'production') ? "https://tgwmxgtk07.execute-api.us-east-1.amazonaws.com/prod" : "http://localhost:3009";

export interface BenchmarkData {
  symbol: string,
  data: Record<string, number>
}



const App = ({ user, setUser }: { user: GoogleAuthUser | null, setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>; }) => {
  // legacy token that identifies unique user
  const [userID, setUserID] = useState("");
  const [showHelpModal, setShowHelpModal] = useState(false);
  const [showContactModal, setShowContactModal] = useState(false);

  useEffect(() => {
    // if (getCookie("userID") === null) {
    //   setShowHelpModal(true);
    // }
    setUserID(getOrCreateUserID());
  }, []);


  return <>
    <div className={styles.bond_ad} onClick={() => { window.location.href = "/bonds" }}>
      <p className={styles.bond_ad_text}><b>Bond Ladder Backtesting is Live →</b></p>
    </div>
    <Nav loggedIn={user !== null} setUser={setUser} showLinks={true} setShowHelpModal={setShowHelpModal} setShowContactModal={setShowContactModal} />
    <div className={styles.centered_container}>
      <FactorBacktestMain userID={userID} user={user} setUser={setUser} />
    </div>

    <StatsFooter user={user} userID={userID} />
    <ContactModal user={user} userID={userID} show={showContactModal} close={() => setShowContactModal(false)} />
    <HelpModal show={showHelpModal} close={() => setShowHelpModal(false)} />
  </>
}


export default App;


export function getCookie(cookieName: string) {
  const name = cookieName + "=";
  const decodedCookie = decodeURIComponent(document.cookie);
  const cookieArray = decodedCookie.split(';');

  for (let i = 0; i < cookieArray.length; i++) {
    let cookie = cookieArray[i];
    while (cookie.charAt(0) === ' ') {
      cookie = cookie.substring(1);
    }
    if (cookie.indexOf(name) === 0) {
      return cookie.substring(name.length, cookie.length);
    }
  }
  return null; // Cookie not found
}

function setCookie(cookieName: string, cookieValue: string) {
  const date = new Date();
  date.setTime(date.getTime() + (900 * 24 * 60 * 60 * 1000));
  const expires = "expires=" + date.toUTCString();
  document.cookie = cookieName + "=" + cookieValue + "; " + expires + ";SameSite=Strict;Secure;HttpOnly";
}

export function getOrCreateUserID(): string {
  const cookieUserID = getCookie("userID")
  if (cookieUserID !== null) {
    return cookieUserID;
  }

  const queryString = window.location.search;
  const urlParams = new URLSearchParams(queryString);
  const newUserID = urlParams.get('userID') || urlParams.get('id') || uuidv4();
  setCookie("userID", newUserID);

  return newUserID;
}

