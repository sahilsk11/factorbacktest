import { useEffect, useState } from 'react';
import BacktestChart from './BacktestChart';
import FactorForm from "./Form";
import InspectFactorData from './FactorSnapshot';
import BenchmarkManager from './BenchmarkSelector';
import { BacktestSnapshot, GoogleAuthUser } from "./models";
import { minMaxDates } from './util';
import { v4 as uuidv4 } from 'uuid';
import { ContactModal, HelpModal } from './Modals';
import StatsFooter from './Footer';
import { Nav } from './Nav';
import styles from './App.module.css'

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

const App = () => {
  // legacy token that identifies unique user
  const [userID, setUserID] = useState("");
  const [showHelpModal, setShowHelpModal] = useState(false);
  const [showContactModal, setShowContactModal] = useState(false);

  // google auth user
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
    // if (getCookie("userID") === null) {
    //   setShowHelpModal(true);
    // }
    setUserID(getOrCreateUserID());
    updateUserFromCookie()
  }, []);

  return <>
    <div className={styles.bond_ad} onClick={() => { window.location.href = "/bonds" }}>
      <p className={styles.bond_ad_text}><b>Bond Ladder Backtesting is Live →</b></p>
    </div>
    <Nav loggedIn={user !== null} setUser={setUser} showLinks={true} setShowHelpModal={setShowHelpModal} setShowContactModal={setShowContactModal} />
    <div className={styles.centered_container}>
      <FactorBacktestMain userID={userID} user={user} />
    </div>

    <StatsFooter user={user} userID={userID} />
    <ContactModal user={user} userID={userID} show={showContactModal} close={() => setShowContactModal(false)} />
    <HelpModal show={showHelpModal} close={() => setShowHelpModal(false)} />
  </>
}

function FactorBacktestMain({ userID, user }: {
  userID: string
  user: GoogleAuthUser | null
}) {
  const [factorData, updateFactorData] = useState<FactorData[]>([]);
  const [benchmarkData, updateBenchmarkData] = useState<BenchmarkData[]>([]);
  const [inspectFactorDataIndex, updateInspectFactorDataIndex] = useState<number | null>(null);
  const [inspectFactorDataDate, updateInspectFactorDataDate] = useState<string | null>(null);

  let takenNames: string[] = [];
  factorData.forEach(fd => {
    takenNames.push(fd.name)
  })
  benchmarkData.forEach(bd => {
    takenNames.push(bd.symbol)
  })
  const { min: minFactorDate, max: maxFactorDate } = minMaxDates(factorData);

  useEffect(() => {
    if (factorData.length > 0) {
      updateInspectFactorDataIndex(factorData.length - 1);
      const d = factorData[factorData.length - 1].data;
      const key = Object.keys(d).reduce((a, b) => (a > b ? b : a));
      updateInspectFactorDataDate(key);
    }
  }, [factorData])

  const classicView = (
    <>
      <div className={`${styles.column} ${styles.form_wrapper}`}>
        <FactorForm
          user={user}
          // set this to the benchmark names that are already in used
          userID={userID}
          takenNames={takenNames}
          appendFactorData={(newFactorData: FactorData) => {
            updateFactorData([...factorData, newFactorData])
          }}
          fullscreenView={false}
        />
        <BenchmarkManager
          user={user}
          userID={userID}
          minDate={minFactorDate}
          maxDate={maxFactorDate}
          updateBenchmarkData={updateBenchmarkData}
        />
      </div>
      <div id="backtest-chart" className={`${styles.column} ${styles.backtest_chart_container}`}>
        <BacktestChart
          benchmarkData={benchmarkData}
          factorData={factorData}
          updateInspectFactorDataIndex={updateInspectFactorDataIndex}
          updateInspectFactorDataDate={updateInspectFactorDataDate}
        />
        <InspectFactorData
          fdIndex={inspectFactorDataIndex}
          fdDate={inspectFactorDataDate}
          factorData={factorData}
        />
      </div>
    </>
  )

  return (
    <div className={styles.container}>
      {false ? <div className={`${styles.tile} ${styles.verbose_builder}`}>

        <div style={{ textAlign: "center" }}>
          <h2>Factor Backtest</h2>
          <p>Create and backtest factor-based investment strategies.</p>
        </div>


        <div className={styles.container}>
          <div className={styles.column}>

            <p>What: Select asset universe</p>
            <p>When: How often to re-evaluate and rebalance holdings</p>
            <p>When: How often to re-evaluate and rebalance holdings</p>
            <p>When: range</p>

          </div>
          <div className={styles.column}>
            <p>how: what strategy do you want to use?</p>
            <p>Advanced: start cash and num assets</p>

          </div>
        </div>

        <button>backtest</button>
      </div> : classicView}
    </div >
  );
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
  document.cookie = cookieName + "=" + cookieValue + "; " + expires;
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

