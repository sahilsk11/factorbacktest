import { useEffect, useState } from 'react';
import BacktestChart from './BacktestChart';
import FactorForm from "./Form";
import Inspector from './FactorSnapshot';
import BenchmarkManager from './BenchmarkSelector';
import { BacktestInputs, BacktestSnapshot, GetSavedStrategiesResponse, GoogleAuthUser, LatestHoldings } from "./models";
import { minMaxDates } from './util';
import { v4 as uuidv4 } from 'uuid';
import { ContactModal, HelpModal } from './Modals';
import StatsFooter from './Footer';
import { Nav } from './Nav';
import styles from './App.module.css'
import { useLocation, useNavigate } from 'react-router-dom';

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

function FactorBacktestMain({ userID, user, setUser }: {
  userID: string
  user: GoogleAuthUser | null,
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>,
}) {
  const [factorData, updateFactorData] = useState<FactorData[]>([]);
  const [benchmarkData, updateBenchmarkData] = useState<BenchmarkData[]>([]);
  const [inspectFactorDataIndex, updateInspectFactorDataIndex] = useState<number | null>(null);
  const [inspectFactorDataDate, updateInspectFactorDataDate] = useState<string | null>(null);
  const [latestHoldings, setLatestHoldings] = useState<LatestHoldings | null>(null);
  const [assetUniverse, setAssetUniverse] = useState<string>("--");

  const [bookmarked, setBookmarked] = useState(false);
  const [savedStrategies, setSavedStrategies] = useState<GetSavedStrategiesResponse[]>([]);


  // everything related to inputs pmuch
  const [numSymbols, setNumSymbols] = useState(10);
  const [factorExpression, setFactorExpression] = useState(`pricePercentChange(
  nDaysAgo(7),
  currentDate
)`);
  const [factorName, setFactorName] = useState("7_day_momentum_weekly");
  const [backtestStart, setBacktestStart] = useState(twoYearsAgoAsString());
  const [backtestEnd, setBacktestEnd] = useState(todayAsString());
  const [samplingIntervalUnit, setSamplingIntervalUnit] = useState("monthly");
  const [selectedFactor, setSelectedFactor] = useState("momentum");

  const backtestInputs: BacktestInputs = {
    factorName,
    factorExpression,
    backtestStart,
    backtestEnd,
    rebalanceInterval: samplingIntervalUnit,
    numAssets: numSymbols,
    assetUniverse,
  }

  const location = useLocation();
  const pathname = location.pathname;
  const navigate = useNavigate();

  if (pathname === "/" && factorData.length > 0) {
    navigate("/backtest")
  } else if (pathname === "/backtest" && factorData.length === 0) {
    navigate("/")
  }

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

  const updateFdIndex = (newVal: number) => {
    updateInspectFactorDataIndex(newVal);
    if (!inspectFactorDataDate || !factorData[newVal].data.hasOwnProperty(inspectFactorDataDate)) {
      // todo - look for closest date instead of resetting
      const date = Object.keys(factorData[newVal].data)[0];
      updateInspectFactorDataDate(date);
    }
  }

  const useVerboseBuilder = window.innerWidth > 767 && pathname !== "/backtest";

  return (
    <>
      <div className={styles.my_container}>
        <div className={`${styles.column} ${styles.form_wrapper}`}>
          <FactorForm
            // set this to the benchmark names that are already in used
            user={user}
            userID={userID}
            takenNames={takenNames}
            appendFactorData={(newFactorData: FactorData) => {
              updateFactorData([...factorData, newFactorData])
            }}
            setUser={setUser}
            fullscreenView={useVerboseBuilder}
            setLatestHoldings={setLatestHoldings}
            numSymbols={numSymbols}
            setNumSymbols={setNumSymbols}
            factorExpression={factorExpression}
            setFactorExpression={setFactorExpression}
            factorName={factorName}
            setFactorName={setFactorName}
            backtestStart={backtestStart}
            setBacktestStart={setBacktestStart}
            backtestEnd={backtestEnd}
            setBacktestEnd={setBacktestEnd}
            samplingIntervalUnit={samplingIntervalUnit}
            setSamplingIntervalUnit={setSamplingIntervalUnit}
            bookmarked={bookmarked}
            setBookmarked={setBookmarked}
            assetUniverse={assetUniverse}
            setAssetUniverse={setAssetUniverse}
            selectedFactor={selectedFactor}
            setSelectedFactor={setSelectedFactor}
            savedStrategies={savedStrategies}
            setSavedStrategies={setSavedStrategies}
          />
          <BenchmarkManager
            user={user}
            userID={userID}
            minDate={minFactorDate}
            maxDate={maxFactorDate}
            updateBenchmarkData={updateBenchmarkData}
          />
        </div>
        {!useVerboseBuilder ?
          <div id="backtest-chart" className={`${styles.column} ${styles.backtest_chart_container}`}>
            <BacktestChart
              benchmarkData={benchmarkData}
              factorData={factorData}
              updateInspectFactorDataIndex={updateFdIndex}
              updateInspectFactorDataDate={updateInspectFactorDataDate}
            />
            <Inspector
              fdIndex={inspectFactorDataIndex}
              fdDate={inspectFactorDataDate}
              factorData={factorData}
              updateInspectFactorDataIndex={updateFdIndex}
              updateInspectFactorDataDate={updateInspectFactorDataDate}
              user={user}
              latestHoldings={latestHoldings}
              bookmarked={bookmarked}
              setBookmarked={setBookmarked}
              backtestInputs={backtestInputs}
              setFactorName={setFactorName}
              setSelectedFactor={setSelectedFactor}
              setSavedStrategies={setSavedStrategies}
            />
          </div> : null}
      </div>
    </>
  )


  // return (
  //   <div className={styles.my_container}>
  //     {useVerboseBuilder ? formComponent : classicView}
  //   </div >
  // );
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

function todayAsString() {
  const today = new Date();
  const year = today.getFullYear();
  const month = String(today.getMonth() + 1).padStart(2, '0'); // Months are 0-based, so add 1
  const day = String(today.getDate()).padStart(2, '0');

  return `${year}-${month}-${day}`;
}

function twoYearsAgoAsString() {
  const today = new Date();
  const year = today.getFullYear() - 2;
  const month = String(today.getMonth() + 1).padStart(2, '0'); // Months are 0-based, so add 1
  const day = String(today.getDate()).padStart(2, '0');

  return `${year}-${month}-${day}`;
}