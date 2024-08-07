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
  setSavedStrategies,
  strategyID,
}: {
  user: GoogleAuthUser | null,
  setUser: Dispatch<SetStateAction<GoogleAuthUser | null>>,
  bookmarked: boolean,
  setBookmarked: Dispatch<SetStateAction<boolean>>,
  backtestInputs: BacktestInputs,
  setFactorName: Dispatch<SetStateAction<string>>,
  setSelectedFactor: Dispatch<SetStateAction<string>>,
  setSavedStrategies: Dispatch<SetStateAction<GetSavedStrategiesResponse[]>>,
  strategyID: string | null,
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
      if (!/[^0-9]/.test(x) && x.length < 3) {
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
        strategyID={strategyID}
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
  strategyID,
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
  strategyID: string | null,
}) {
  const [checked, setChecked] = useState<boolean>(false);
  const [invested, setInvested] = useState(false);

  if (!show) return null;

  function closeWrapper() {
    setChecked(false);
    setInvested(false);
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
      return
    }
    if (!strategyID) {
      alert("savedStrategyID not set")
      return
    }
    try {
      const response = await fetch(endpoint + "/investInStrategy", {
        method: "POST",
        headers: {
          "Authorization": user ? "Bearer " + user.accessToken : ""
        },
        body: JSON.stringify({
          amountDollars: depositAmount,
          strategyID,
        } as InvestInStrategyRequest)
      });
      if (!response.ok) {
        const j = await response.json()
        alert(j.error)
        console.error("Error submitting data:", response.status);
      } else {
        setInvested(true)
      }
    } catch (error) {
      alert((error as Error).message)
      console.error("Error:", error);
    }
  }

  const fontSize = "14px";
  const height = "30px";

  const submitForm = <>
    <form onSubmit={(e) => {
      e.preventDefault();
      invest()
      setBookmarked(true)
    }}>
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
      <label className={formStyles.label}>Deposit Amount</label>
      <div style={{
        display: "flex",
        justifyContent: "center",
        marginTop: "5px"
      }} className="input-group mb-3 center">
        <span className="input-group-text" style={{
          fontSize,
          height
        }}>$</span>
        <input
          type="text"
          className={`${iisStyles.deposit_input} form-control `}
          aria-label="Amount (to the nearest dollar)"
          value={depositAmount.toLocaleString()}
          onChange={(e) => setDepositAmount(e)}
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

      <p style={{marginTop:"30px"}} className={iisStyles.subtext}>
        You will receive a Venmo request in the next 24 hours for the given amount.
        <br />
        Portfolio will rebalance once per day at market open.
        <br /><br />
        Past performance does not guarantee future results - may lose value.
      </p>

      <div style={{ marginBottom: "10px" }}>
        <input
          type='checkbox'
          checked={checked}
          onChange={() => { setChecked(!checked) }}
        />
        <label
          style={{ fontSize: "13px" }}
        > I understand</label>
      </div>
      <button
        className={`${formStyles.backtest_btn} ${iisStyles.deposit_btn}`}
        type='submit'
        disabled={!checked}
      >Submit</button>

    </form >
  </>

  const confirmation = <>
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
  </>



  return (
    <div id="invest-modal" className={modalStyles.modal} onClick={handleOverlayClick}>
      <div className={modalStyles.modal_content}>
        <span onClick={() => closeWrapper()} className={modalStyles.close} id="closeInvestModalBtn">&times;</span>
        <h2 style={{ marginBottom: "40px" }}>Invest in Strategy</h2>

        {invested ? confirmation : submitForm}

      </div>

    </div>
  );
}
