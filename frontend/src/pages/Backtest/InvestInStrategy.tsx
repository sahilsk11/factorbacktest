import appStyles from 'App.module.css'
import iisStyles from "./InvestInStrategy.module.css";
import formStyles from "./Form.module.css"
import { Dispatch, SetStateAction, useState } from 'react';
import backtestStyles from './Backtest.module.css';
import { endpoint } from 'App';
import { GoogleAuthUser, BacktestInputs, GetSavedStrategiesResponse, InvestInStrategyRequest } from 'models';
import { Pagination } from 'react-bootstrap';
import ConfettiExplosion from 'react-confetti-explosion';
import { useNavigate } from 'react-router-dom';
import { updateBookmarked, getStrategies } from './Form';
import { useGoogleLogin } from '@react-oauth/google';
import modalStyles from "common/Modals.module.css";
import factorSnapshotStyles from "./FactorSnapshot.module.css";


export function InvestInStrategy({
  user,
  setUser,
  bookmarked,
  setBookmarked,
  backtestInputs,
  setFactorName,
  setSelectedFactor,
  setSavedStrategies
}: {
  user: GoogleAuthUser | null,
  setUser: Dispatch<SetStateAction<GoogleAuthUser | null>>,
  bookmarked: boolean,
  setBookmarked: Dispatch<SetStateAction<boolean>>,
  backtestInputs: BacktestInputs,
  setFactorName: Dispatch<SetStateAction<string>>,
  setSelectedFactor: Dispatch<SetStateAction<string>>,
  setSavedStrategies: Dispatch<SetStateAction<GetSavedStrategiesResponse[]>>,
}) {
  const [depositAmount, setDepositAmount] = useState(10);
  const [showInvestModal, setShowInvestModal] = useState(false);


  function updateDepositAmount(e: any) {
    {
      let x = e.target.value.replace(/,/g, '')
      x = x.replace(/\$ /g, '')
      if (x.length === 0) {
        x = "0";
      }
      if (!/[^0-9]/.test(x) && x.length < 12) {
        setDepositAmount(parseFloat(x))
      }
    }
  }

  function deposit(e: any) {
    e.preventDefault()
    if (user) {
      // maybe bookmark strategy
      setShowInvestModal(true)
    } else {
      login()
    }

  }

  const {
    factorName,
    factorExpression,
    assetUniverse,
  } = backtestInputs;

  const login = useGoogleLogin({
    onSuccess: (codeResponse) => {
      // console.log(codeResponse)
      const date = new Date();
      date.setTime(date.getTime() + (codeResponse.expires_in * 1000));
      const expires = "expires=" + date.toUTCString();

      document.cookie = "googleAuthAccessToken" + "=" + codeResponse.access_token + "; " + expires + ";SameSite=Strict;Secure";
      const newUser = {
        accessToken: codeResponse.access_token
      } as GoogleAuthUser
      setUser(newUser);

      setShowInvestModal(true)

    },
    onError: (error) => console.log('Login Failed:', error)
  });

  const fontSize = "14px";
  const height = "30px";

  return (
    <>
      <div className={`${appStyles.tile} ${backtestStyles.flex_container}`}>
        <div className={`${iisStyles.deposit_container}`}>
          <div>
            <p className={iisStyles.invest_title}>Invest in Strategy</p>
            <p className={`${appStyles.subtext} ${iisStyles.subtext}`}>Paper trade or deposit real funds</p>
          </div>

          <div>
            <form onSubmit={deposit}>
              <div className="input-group mb-3">
                <span className="input-group-text" style={{
                  fontSize,
                  height
                }}>$</span>
                <input
                  type="text"
                  className={`${iisStyles.deposit_input} form-control `}
                  aria-label="Amount (to the nearest dollar)"
                  value={depositAmount.toLocaleString()}
                  onChange={(e) => updateDepositAmount(e)}
                  style={{
                    width: "30px",
                    fontSize,
                    height,
                  }}
                />
                <span style={{
                  fontSize,
                  height,
                }} className="input-group-text">.00</span>
              </div>
              {/* <input
            // id="cash"
            className={iisStyles.deposit_input}
            value={"$ " + depositAmount.toLocaleString()}
            style={{ paddingLeft: "5px" }}
            onChange={(e) => updateDepositAmount(e)}
          /> */}
              <button className={`${formStyles.backtest_btn} ${iisStyles.deposit_btn}`} type="submit">Start</button>
            </form>
          </div>
        </div>
      </div>
      <InvestModal
        user={user}
        show={showInvestModal}
        close={() => { setShowInvestModal(false) }}
        factorName={factorName}
        setFactorName={setFactorName}
        bookmarked={bookmarked}
        // bookmarkStategy={}
        depositAmount={depositAmount}
        setDepositAmount={updateDepositAmount}
        setSavedStrategies={setSavedStrategies}
        setSelectedFactor={setSelectedFactor}
        backtestInputs={backtestInputs}
        setBookmarked={setBookmarked}
      />
    </>
  )
}

function InvestModal({
  user,
  show,
  close,
  factorName,
  setFactorName,
  setBookmarked,
  // bookmarkStategy,
  bookmarked,
  depositAmount,
  setDepositAmount,
  backtestInputs,
  setSavedStrategies,
  setSelectedFactor,
  // onSubmit,
}: {
  user: GoogleAuthUser | null,
  show: boolean;
  close: () => void;
  factorName: string,
  setFactorName: React.Dispatch<SetStateAction<string>>;
  setBookmarked: React.Dispatch<SetStateAction<boolean>>;
  // bookmarkStategy: () => void;
  bookmarked: boolean;
  depositAmount: number,
  setDepositAmount: (e: any) => void,
  backtestInputs: BacktestInputs,
  setSavedStrategies: Dispatch<SetStateAction<GetSavedStrategiesResponse[]>>,
  setSelectedFactor: Dispatch<SetStateAction<string>>,

  // user: GoogleAuthUser | null,
  // onSubmit: () => Promise<void>
}) {
  const [stepNumber, setSetStepNumber] = useState(0);
  const [clickedVenmoLink, setClickedVenmoLink] = useState(false);
  const [savedStrategyID, setSavedStrategyID] = useState<string | null>(null)
  const [depositSuccessful, setDepositSuccessful] = useState(false)
  const [saveSuccessful, setSaveSuccessful] = useState(false)
  // useEffect(() => {
  //   if (bookmarked) {
  //     setSetStepNumber(Math.max(stepNumber, 1))
  //   } else {
  //     setSetStepNumber(0)
  //   }
  // }, [bookmarked])

  const navigate = useNavigate();

  if (!show) return null;

  async function bookmarkStrategy() {
    if (user) {
      setBookmarked(true)
      const strategyID = await updateBookmarked(user, true, backtestInputs)
      if (!strategyID) {
        alert("failed to retrieve bookmarked strategy ID")
      }
      setSavedStrategyID(strategyID);
      await getStrategies(user, setSavedStrategies);
      setSelectedFactor(factorName)
      setSaveSuccessful(true)
    } else {
      // should be impossible
      alert("must be logged in")
    }
  }

  function closeWrapper() {
    setSetStepNumber(0)
    setClickedVenmoLink(false);
    close()
  }

  const handleOverlayClick = (e: any) => {
    if (e.target.id === "invest-modal") {
      closeWrapper();
    }
  };


  async function invest() {
    if (!user) {
      alert("must be logged in to invest")
    }
    if (!savedStrategyID) {
      alert("savedStrategyID not set")
    }
    try {
      const response = await fetch(endpoint + "/investInStrategy", {
        method: "POST",
        headers: {
          "Authorization": user ? "Bearer " + user.accessToken : ""
        },
        body: JSON.stringify({
          amountDollars: depositAmount,
          savedStrategyID,
        } as InvestInStrategyRequest)
      });
      if (!response.ok) {
        const j = await response.json()
        alert(j.error)
        console.error("Error submitting data:", response.status);
      } else {
        setDepositSuccessful(true)
      }
    } catch (error) {
      alert((error as Error).message)
      console.error("Error:", error);
    }
  }

  const steps = [
    {
      component: (<>
        <div>
          <label className={formStyles.label}>Strategy Name</label>
          <input
            type="text"
            value={factorName}
            className={modalStyles.contact_form_email_input}
            onChange={(e) => {
              setFactorName(e.target.value)
            }}
          />
        </div>
        {/* <button className={formStyles.backtest_btn} type='submit'>Submit</button> */}
      </>),
      onComplete: () => {
        bookmarkStrategy()
      },
      canProceed: true
    },
    {
      component: (<>
        <div>
          <label className={formStyles.label}>Deposit Funds</label>
          Please venmo @sahilsk11 ${depositAmount}
          <br />
          <br />
          <a href="https://venmo.com/sahilsk11" target="_blank" onClick={() => setClickedVenmoLink(true)}>Click here to launch Venmo</a>
        </div>
        {!clickedVenmoLink ? <p className={appStyles.subtext}>complete the Venmo transaction to continue</p> : null}
        {/* <button className={formStyles.backtest_btn} type='submit'>Submit</button> */}
      </>),
      onComplete: () => {
        invest()
      },
      canProceed: saveSuccessful && clickedVenmoLink && savedStrategyID,
    },
    {
      component: (<>
        <div style={{ position: "relative" }}>
          <div style={{
            position: "absolute",
            left: 0,
            right: 0,
            top: "-40px",
            width: "1px",
            margin: "0px auto",
            display: "block"
          }}>
            <ConfettiExplosion zIndex={1000} duration={3000} />
          </div>
          <label className={formStyles.label}>Thanks</label>
          You're all set. Track your investments <a href="/investments">here</a>.
        </div>
        {/* <button className={formStyles.backtest_btn} type='submit'>Submit</button> */}
      </>),
      onComplete: () => { },
      canProceed: false,
    },
  ]

  return (
    <div id="invest-modal" className={modalStyles.modal} onClick={handleOverlayClick}>
      <div className={modalStyles.modal_content}>
        <span onClick={() => closeWrapper()} className={modalStyles.close} id="closeInvestModalBtn">&times;</span>
        <h2 style={{ marginBottom: "40px" }}>Invest in Strategy</h2>
        {steps[stepNumber].component}

        <div className={factorSnapshotStyles.invest_modal_pagination_container}>
          {stepNumber < steps.length - 1 ? <Pagination>
            <Pagination.Item
              onClick={() => setSetStepNumber(
                Math.max(stepNumber - 1, 0)
              )}
              disabled={stepNumber === 0}
            >Prev</Pagination.Item>
            <Pagination.Item
              onClick={() => {
                setSetStepNumber(
                  Math.min(stepNumber + 1, steps.length - 1)
                )
                steps[stepNumber].onComplete()
              }}
              disabled={stepNumber === steps.length - 1 || !steps[stepNumber].canProceed}
            >
              Next
            </Pagination.Item>
          </Pagination> : <button className={`${formStyles.backtest_btn} ${factorSnapshotStyles.deposit_btn}`} onClick={() => closeWrapper()}>Close</button>}
        </div>
      </div>

    </div>
  );
}
