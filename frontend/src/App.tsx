import { useEffect, useState } from 'react';
import './app.css'
import BacktestChart from './BacktestChart';
import FactorForm from "./Form";
import InspectFactorData from './FactorSnapshot';
import BenchmarkManager from './BenchmarkSelector';
import { BacktestSnapshot } from "./models";
import { minMaxDates } from './util';
import { v4 as uuidv4 } from 'uuid';
import { ContactModal, HelpModal } from './Modals';
import StatsFooter from './Footer';


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

const App = () => {
  const [userID, setUserID] = useState("");
  const [factorData, updateFactorData] = useState<FactorData[]>([]);
  const [benchmarkData, updateBenchmarkData] = useState<BenchmarkData[]>([]);
  const [inspectFactorDataIndex, updateInspectFactorDataIndex] = useState<number | null>(null);
  const [inspectFactorDataDate, updateInspectFactorDataDate] = useState<string | null>(null);
  const [showHelpModal, setShowHelpModal] = useState(false);
  const [showContactModal, setShowContactModal] = useState(false);

  useEffect(() => {
    if (getCookie("userID") === null) {
      setShowHelpModal(true);
    }
    setUserID(getOrCreateUserID());
  }, []);

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

  return <>
    <Nav setShowHelpModal={setShowHelpModal} setShowContactModal={setShowContactModal} />
    <div className="centered-container">
      <div className="container">
        <div className="column form-wrapper">
          <FactorForm
            // set this to the benchmark names that are already in used
            userID={userID}
            takenNames={takenNames}
            appendFactorData={(newFactorData: FactorData) => {
              updateFactorData([...factorData, newFactorData])
            }}
          />
          <BenchmarkManager
            userID={userID}
            minDate={minFactorDate}
            maxDate={maxFactorDate}
            updateBenchmarkData={updateBenchmarkData}
          />
        </div>
        <div id="backtest-chart" className="column backtest-chart-container">
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
      </div>
    </div>

    <StatsFooter userID={userID} />
    <ContactModal userID={userID} show={showContactModal} close={() => setShowContactModal(false)} />
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

export function Nav({ setShowHelpModal, setShowContactModal }: {
  setShowHelpModal: React.Dispatch<React.SetStateAction<boolean>>;
  setShowContactModal: React.Dispatch<React.SetStateAction<boolean>>;

}) {
  return <>
    <div className='nav'>
      <h4 className='nav-title' onClick={() => window.location.href = "/"}>factorbacktest.net</h4>
      <div className='nav-element-container'>
        <div className='nav-element-wrapper'>
          <p onClick={() => setShowContactModal(true)} className='nav-element-text'>Contact</p>
        </div>
        <div className='nav-element-wrapper'>
          <p onClick={() => setShowHelpModal(true)} className='nav-element-text'>User Guide</p>
        </div>
      </div>
    </div>
  </>
}