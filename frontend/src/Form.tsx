import React, { Dispatch, SetStateAction, useEffect, useState } from 'react';
import { FactorData, endpoint } from "./App";
import formStyles from "./Form.module.css";
import appStyles from "./l3_service.module.css";
import { BacktestRequest, GetAssetUniversesResponse, BacktestResponse, FactorOptions, GoogleAuthUser, BookmarkStrategyRequest, GetSavedStrategiesResponse, LatestHoldings } from './models';
import 'react-tooltip/dist/react-tooltip.css'
import { daysBetweenDates } from './util';
import { FactorExpressionInput } from './FactorExpressionInput';
import { Col, Container, Row } from 'react-bootstrap';
import { useNavigate } from 'react-router-dom';
import { FaBookmark, FaRegBookmark } from "react-icons/fa";
import { Tooltip as ReactTooltip } from 'react-tooltip';
import { useGoogleLogin } from '@react-oauth/google';
import modalsStyle from "./Modals.module.css";

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

export default function FactorForm({
  userID,
  takenNames,
  appendFactorData,
  fullscreenView,
  user,
  setUser,
  setLatestHoldings,
}: {
  user: GoogleAuthUser | null,
  userID: string,
  takenNames: string[];
  appendFactorData: (newFactorData: FactorData) => void;
  fullscreenView: boolean,
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>,
  setLatestHoldings: React.Dispatch<React.SetStateAction<LatestHoldings | null>>,
}) {
  const [factorExpression, setFactorExpression] = useState(`pricePercentChange(
  nDaysAgo(7),
  currentDate
)`);
  const [factorName, setFactorName] = useState("7_day_momentum_weekly");
  const [backtestStart, setBacktestStart] = useState(twoYearsAgoAsString());
  const [backtestEnd, setBacktestEnd] = useState(todayAsString());
  const [samplingIntervalUnit, setSamplingIntervalUnit] = useState("monthly");

  const [cash, setCash] = useState(10_000);

  const [numSymbols, setNumSymbols] = useState(10);
  const [names, setNames] = useState<string[]>([...takenNames]);
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [assetUniverse, setAssetUniverse] = useState<string>("--");
  const [assetUniverses, setAssetUniverses] = useState<GetAssetUniversesResponse[]>([]);
  const [savedStrategies, setSavedStrategies] = useState<GetSavedStrategiesResponse[]>([]);


  async function getStrategies() {
    try {
      const response = await fetch(endpoint + "/savedStrategies", {
        headers: {
          "Authorization": user ? "Bearer " + user.accessToken : ""
        }
      });
      if (!response.ok) {
        const j = await response.json()
        alert(j.error)
        console.error("Error submitting data:", response.status);
      } else {
        const j = await response.json() as GetSavedStrategiesResponse[];
        setSavedStrategies(j.filter(e => e.bookmarked))
      }
    } catch (error) {
      alert((error as Error).message)
      console.error("Error:", error);
    }
  }

  const getUniverses = async () => {
    try {
      const response = await fetch(endpoint + "/assetUniverses", {
        headers: {
          "Authorization": user ? "Bearer " + user.accessToken : ""
        }
      });
      if (response.ok) {
        const results: GetAssetUniversesResponse[] = await response.json()
        if (Object.keys(results).length === 0) {
          setErr("No universes results were retrieved");
          return;
        }
        setAssetUniverses(results);
      } else {
        const j = await response.json()
        setErr(j.error)
        console.error("Error submitting data:", response.status);
      }
    } catch (error) {
      setLoading(false)
      setErr((error as Error).message)
      console.error("Error:", error);
    }
  };

  useEffect(() => {
    getUniverses()
  }, []);
  useEffect(() => {
    if (user) {
      getStrategies()
    }
  }, [user]);
  useEffect(() => {
    if (assetUniverses.length > 0) {
      setAssetUniverse(assetUniverses[0].code)
    }
  }, [assetUniverses]);

  let i = 0;
  let assetUniverseSelectOptions = assetUniverses.map(u => {
    return <option key={i++} value={u.code}>{u.displayName}</option>
  })

  const updateName = (newName: string) => {
    setFactorName(newName + "_" + samplingIntervalUnit)
  }

  useEffect(() => {
    let name = factorName;
    if (name.endsWith("_monthly")) {
      name = name.substring(0, name.indexOf("_monthly"))
    }
    if (name.endsWith("_weekly")) {
      name = name.substring(0, name.indexOf("_weekly"))
    }
    if (name.endsWith("_daily")) {
      name = name.substring(0, name.indexOf("_daily"))
    }
    updateName(name);
  }, [samplingIntervalUnit])

  const handleSubmit = async (e: any) => {
    e.preventDefault();
    setErr(null);
    setLoading(true);

    const data: BacktestRequest = {
      factorOptions: {
        expression: factorExpression,
        name: factorName,
      } as FactorOptions,
      backtestStart,
      backtestEnd,
      samplingIntervalUnit,
      startCash: cash,
      numSymbols,
      userID,
      assetUniverse,
    };

    try {
      const response = await fetch(endpoint + "/backtest", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": user ? "Bearer " + user.accessToken : ""
        },
        body: JSON.stringify(data)
      });
      setLoading(false);
      if (response.ok) {
        const result: BacktestResponse = await response.json()
        if (Object.keys(result.backtestSnapshots).length === 0) {
          setErr("No backtest results were calculated");
          return;
        }
        jumpToAnchorOnSmallScreen("backtest-chart")

        let newName = data.factorOptions.name;
        let found = false;
        let nextNum = 1;
        names.forEach(n => {
          if (n.includes(factorName)) {
            found = true;
            const match = n.match(/\((\d+)\)/);
            if (match) {
              const number = parseInt(match[1], 10);
              nextNum = Math.max(number + 1, nextNum)
            }
          }
        })
        if (found) {
          newName += " (" + nextNum.toString() + ")"
        }

        setNames([...names, newName])
        const fd: FactorData = {
          name: newName,
          data: result.backtestSnapshots,
          expression: data.factorOptions.expression
        } as FactorData;

        appendFactorData(fd)
        setLatestHoldings(result.latestHoldings);
      } else {
        const j = await response.json()
        setErr(j.error)
        console.error("Error submitting data:", response.status);
      }
    } catch (error) {
      setLoading(false)
      setErr((error as Error).message)
      console.error("Error:", error);
    }
  };

  let rebalanceDuration = 1;
  switch (samplingIntervalUnit) {
    case "weekly": rebalanceDuration = 7; break;
    case "monthly": rebalanceDuration = 30; break;
    case "yearly": rebalanceDuration = 365; break;
  }

  const numAssetsInSelectedUniverse = assetUniverses.find(
    e => e.code === assetUniverse)?.numAssets;


  const maxDate = new Date().toISOString().split('T')[0];
  let numComputations = 0;
  if (backtestStart <= backtestEnd && backtestEnd <= maxDate) {
    if (numAssetsInSelectedUniverse) {
      // const numAssets = assetDetails.numAssets;
      numComputations = numAssetsInSelectedUniverse * daysBetweenDates(backtestStart, backtestEnd) / rebalanceDuration / 4;
    }
  }

  // TODO - attach these to the form inputs instead
  const cashInput = document.getElementById("cash");
  if (cash <= 0) {
    (cashInput as HTMLInputElement)?.setCustomValidity("Please enter more than $0.")
  } else {
    (cashInput as HTMLInputElement)?.setCustomValidity("")
  }
  const numSymbolsInput = document.getElementById("num-symbols");
  if (numSymbols <= 2) {
    (numSymbolsInput as HTMLInputElement)?.setCustomValidity("Please enter more than 2 assets.")
  } else if (numSymbols > (numAssetsInSelectedUniverse || 100)) {
    (numSymbolsInput as HTMLInputElement)?.setCustomValidity(`Please use less than ${(numAssetsInSelectedUniverse || 100)} assets.`)
  } else {
    (numSymbolsInput as HTMLInputElement)?.setCustomValidity("")
  }

  // we can't have any components manage shared state
  // (between verbose and regular view) under this component

  const [gptInput, setGptInput] = useState("");
  const [selectedFactor, setSelectedFactor] = useState("momentum");

  // todo - create a separate object that contains the setters
  // for all the backtest inputs, and pass that to FactorExpressionInput
  const props: FormViewProps = {
    handleSubmit,
    factorName,
    setFactorName,
    userID,
    factorExpression,
    setFactorExpression,
    updateName,
    maxDate,
    backtestStart,
    setBacktestStart,
    backtestEnd,
    setBacktestEnd,
    samplingIntervalUnit,
    setSamplingIntervalUnit,
    numSymbols,
    setNumSymbols,
    cash,
    setCash,
    assetUniverse,
    setAssetUniverse,
    assetUniverseSelectOptions,
    numComputations,
    loading,
    err,
    user,
    setUser,
    factorExpressionInput: null,
    getStrategies,
    setSelectedFactor,
  }

  const factorExpressionInput = <FactorExpressionInput
    userID={userID}
    factorExpression={factorExpression}
    setFactorExpression={setFactorExpression}
    updateName={updateName}
    user={user}
    gptInput={gptInput}
    selectedFactor={selectedFactor}
    setSelectedFactor={setSelectedFactor}
    setGptInput={setGptInput}
    savedStrategies={savedStrategies}

    // we have to let this guy change all the props in case a
    // bookmarked strat is selected. do this the lazy way
    formProps={props}
  />

  props.factorExpressionInput = factorExpressionInput;



  return fullscreenView ? <VerboseFormView props={props} /> : <ClassicFormView props={props} />
}

export interface FormViewProps {
  handleSubmit: (e: any) => Promise<void>,
  factorName: string,
  setFactorName: Dispatch<SetStateAction<string>>,
  userID: string,
  factorExpression: string,
  setFactorExpression: Dispatch<SetStateAction<string>>,
  updateName: (newName: string) => void,
  maxDate: string,
  backtestStart: string,
  setBacktestStart: Dispatch<SetStateAction<string>>,
  backtestEnd: string,
  setBacktestEnd: Dispatch<SetStateAction<string>>,
  samplingIntervalUnit: string,
  setSamplingIntervalUnit: Dispatch<SetStateAction<string>>,
  numSymbols: number,
  setNumSymbols: Dispatch<SetStateAction<number>>,
  cash: number,
  setCash: Dispatch<SetStateAction<number>>,
  assetUniverse: string,
  setAssetUniverse: Dispatch<SetStateAction<string>>,
  assetUniverseSelectOptions: JSX.Element[],
  numComputations: number,
  loading: boolean,
  err: string | null,
  user: GoogleAuthUser | null,
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>,
  factorExpressionInput: JSX.Element | null,

  getStrategies: () => Promise<void>,
  setSelectedFactor: Dispatch<SetStateAction<string>>,
}

function ClassicFormView({
  props
}: {
  props: FormViewProps
}) {
  const {
    handleSubmit,
    factorName,
    setFactorName,
    userID,
    factorExpression,
    setFactorExpression,
    updateName,
    maxDate,
    backtestStart,
    setBacktestStart,
    backtestEnd,
    setBacktestEnd,
    samplingIntervalUnit,
    setSamplingIntervalUnit,
    numSymbols,
    setNumSymbols,
    cash,
    setCash,
    assetUniverse,
    setAssetUniverse,
    assetUniverseSelectOptions,
    numComputations,
    loading,
    err,
    user,
    setUser,
    factorExpressionInput,
  } = props;
  return (
    <div className={appStyles.tile} style={{ position: "relative" }}>
      <h2 style={{ textAlign: "left", margin: "0px" }}>Backtest Strategy</h2>
      <p className={appStyles.subtext}>Define your quantitative strategy and customize backtest parameters.</p>
      <BookmarkStrategy user={user} setUser={setUser} formProps={props} />
      <form onSubmit={handleSubmit}>

        <div className={formStyles.form_element}>
          {factorExpressionInput}
        </div>



        <div className={formStyles.form_element}>
          <label className={formStyles.label}>Backtest Range</label>
          <input
            min={'2010-01-01'}
            max={backtestEnd > maxDate ? maxDate : backtestEnd}
            required
            type="date"
            value={backtestStart}
            onChange={(e) => setBacktestStart(e.target.value)}
          />
          <p style={{ display: "inline" }}> to </p>
          <input
            max={maxDate}
            required
            type="date"
            value={backtestEnd}
            onChange={(e) => setBacktestEnd(e.target.value)}
          />
        </div>

        <div className={formStyles.form_element}>
          <label className={formStyles.label}>Rebalance Interval</label>
          <p className={formStyles.label_subtext}>How frequently should we re-evaluate portfolio holdings.</p>
          <select value={samplingIntervalUnit} onChange={(e) => setSamplingIntervalUnit(e.target.value)}>
            <option value="daily">daily</option>
            <option value="weekly">weekly</option>
            <option value="monthly">monthly</option>
            <option value="yearly">yearly</option>
          </select>
        </div>


        <div>
          <label className={formStyles.label}>Number of Assets</label>
          <p className={formStyles.label_subtext}>How many assets the target portfolio should hold at any time.</p>
          <input
            id="num-symbols"
            // max={numAssetsInSelectedUniverse}
            style={{ width: "80px" }}
            value={numSymbols}
            // min={3}
            onChange={(e) => {
              let x = e.target.value;
              if (x.length === 0) {
                x = "0";
              }
              if (!/[^0-9]/.test(x)) {
                setNumSymbols(parseFloat(x))
              }
            }
            }
          />
        </div>

        {/* <div>
          <label className={formStyles.label}>Starting Cash</label>
          <span style={{ fontSize: "14px" }}>$</span> <input
            id="cash"
            value={cash.toLocaleString()}
            style={{ paddingLeft: "5px" }}
            onChange={(e) => {
              let x = e.target.value.replace(/,/g, '')
              if (x.length === 0) {
                x = "0";
              }
              if (!/[^0-9]/.test(x) && x.length < 12) {
                setCash(parseFloat(x))
              }
            }}
          />
        </div> */}
        <div className={formStyles.form_element}>
          <label className={formStyles.label}>Asset Universe</label>
          <p className={formStyles.label_subtext}>The pool of assets that are eligible for the target portfolio.</p>
          <select value={assetUniverse} onChange={(e) => setAssetUniverse(e.target.value)}>
            {assetUniverseSelectOptions}
          </select>
        </div>

        {numComputations > 10_000 ? <p style={{ marginTop: "5px" }} className={formStyles.label_subtext}>This backtest range + rebalance combination requires {numComputations.toLocaleString('en-US', { style: 'decimal' }).split('.')[0]} computations and may take up to {Math.floor(numComputations / 10000) * 10} seconds.</p> : null}

        {loading ? <img style={{ width: "40px", marginTop: "20px", marginLeft: "40px" }} src='loading.gif' /> : <button className={formStyles.backtest_btn} type="submit">Run Backtest</button>}

        <div className={formStyles.error_container}>
          <Error message={err} />
        </div>
      </form>
    </div>
  );
}

// responsive but not ready for mobile
function VerboseFormView({ props }: { props: FormViewProps }) {
  const {
    handleSubmit,
    factorName,
    // setFactorName,
    userID,
    factorExpression,
    setFactorExpression,
    updateName,
    maxDate,
    backtestStart,
    setBacktestStart,
    backtestEnd,
    setBacktestEnd,
    samplingIntervalUnit,
    setSamplingIntervalUnit,
    numSymbols,
    setNumSymbols,
    cash,
    setCash,
    assetUniverse,
    setAssetUniverse,
    assetUniverseSelectOptions,
    numComputations,
    loading,
    user,
    err,
    factorExpressionInput,
  } = props;

  const [clicked, setClicked] = useState(false);
  const navigate = useNavigate();

  const loadingIcon = <img
    style={{
      width: "40px",
      margin: "0px auto",
      display: "block",
    }}
    src='loading.gif'
    alt='loading...'
  />

  const onSubmit = (e: any) => {
    handleSubmit(e)
    setClicked(true);
  }

  useEffect(() => {
    if (!loading && clicked) {
      navigate("/backtest");
    }
  }, [loading, clicked])

  const buttons = <>
    <button
      className={`${formStyles.backtest_btn} ${formStyles.verbose_backtest_btn}`}
      type="submit">
      Run Backtest
    </button>
  </>
  return (
    <div className={`${appStyles.tile} ${formStyles.verbose_tile}`}>
      <div className={formStyles.verbose_heading_container}>
        <h2 style={{ marginBottom: "0px" }}>Factor Backtest</h2>
        <p className={formStyles.verbose_builder_subtitle}>Create and backtest factor-based investment strategies.</p>
      </div>

      <form onSubmit={onSubmit} style={{ display: "contents" }}>
        <Container>
          <Row>
            <Col md={6}>
              <div className={formStyles.verbose_inner_column_wrapper}>
                <div className={formStyles.form_element}>
                  <label className={formStyles.label}>Asset Universe</label>
                  <p className={formStyles.label_subtext}>The pool of assets that are eligible for the target portfolio.</p>
                  <select value={assetUniverse} onChange={(e) => setAssetUniverse(e.target.value)}>
                    {assetUniverseSelectOptions}
                  </select>
                </div>
                <div className={formStyles.form_element}>
                  <label className={formStyles.label}>Backtest Range</label>
                  <input
                    min={'2010-01-01'}
                    max={backtestEnd > maxDate ? maxDate : backtestEnd}
                    required
                    type="date"
                    value={backtestStart}
                    onChange={(e) => setBacktestStart(e.target.value)}
                  />
                  <p style={{ display: "inline" }}> to </p>
                  <input
                    max={maxDate}
                    required
                    type="date"
                    value={backtestEnd}
                    onChange={(e) => setBacktestEnd(e.target.value)}
                  />
                </div>
                <div className={formStyles.form_element}>
                  <label className={formStyles.label}>Rebalance Interval</label>
                  <p className={formStyles.label_subtext}>How frequently should we re-evaluate portfolio holdings.</p>
                  <select value={samplingIntervalUnit} onChange={(e) => setSamplingIntervalUnit(e.target.value)}>
                    <option value="daily">daily</option>
                    <option value="weekly">weekly</option>
                    <option value="monthly">monthly</option>
                    <option value="yearly">yearly</option>
                  </select>
                </div>
                <div className={formStyles.form_element}>
                  <label className={formStyles.label}>Number of Assets</label>
                  <p className={formStyles.label_subtext}>How many assets the target portfolio should hold at any time.</p>
                  <input
                    id="num-symbols"
                    // max={numAssetsInSelectedUniverse}
                    style={{ width: "80px" }}
                    value={numSymbols}
                    // min={3}
                    onChange={(e) => {
                      let x = e.target.value;
                      if (x.length === 0) {
                        x = "0";
                      }
                      if (!/[^0-9]/.test(x)) {
                        setNumSymbols(parseFloat(x))
                      }
                    }
                    }
                  />
                </div>

              </div>
            </Col>
            <Col md={6}>
              <div className={formStyles.verbose_inner_column_wrapper}>
                {/* <div className={formStyles.form_element}>
                  <label className={formStyles.label}>Strategy Name</label>
                  <input style={{ width: "250px" }} required
                    id="factor-name"
                    type="text"
                    value={factorName}
                    onChange={(e) =>
                      setFactorName(e.target.value)
                    }
                  />
                </div> */}
                <div className={formStyles.form_element}>
                  {factorExpressionInput}
                </div>
              </div>
            </Col>
          </Row>
        </Container>

        <div className={formStyles.verbose_button_container}>
          {numComputations > 10_000 ? <p style={{ marginTop: "5px" }} className={formStyles.label_subtext}>This backtest range + rebalance combination requires {numComputations.toLocaleString('en-US', { style: 'decimal' }).split('.')[0]} computations and may take up to {Math.floor(numComputations / 10000) * 10} seconds.</p> : null}

          {loading ? loadingIcon : buttons}

          <div className={formStyles.verbose_error_container}>
            <Error message={err} />
          </div>
        </div>

      </form>
    </div>
  );
}

export function Error({ message }: { message: string | null }) {
  return message === null ? null : <>
    <div className={formStyles.error_wrapper}>
      <h4 style={{ marginBottom: "0px", marginTop: "0px" }}>That's an error.</h4>
      <p>{message}</p>
    </div>
  </>
}

function jumpToAnchorOnSmallScreen(anchorId: string) {
  // Check if the screen width is less than 600 pixels
  if (window.innerWidth < 600) {
    // Get the element with the specified anchorId
    const anchorElement = document.getElementById(anchorId);

    // Check if the element exists
    if (anchorElement) {
      // Calculate the position to scroll to
      const offset = anchorElement.getBoundingClientRect().top + window.scrollY;

      // Scroll to the element smoothly
      window.scrollTo({
        top: offset,
        behavior: 'smooth'
      });
    }
  }
}

function BookmarkStrategy({ user, setUser, formProps }: {
  user: GoogleAuthUser | null;
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>;
  formProps: FormViewProps,
}) {
  const [bookmarked, setBookmarked] = useState(false);
  const toolTipMessage = `Bookmark strategy`;
  const [showBookmarkModal, setShowBookmarkModal] = useState(false);

  const getIsBookmarked: (user: GoogleAuthUser) => Promise<any> = async (user: GoogleAuthUser) => {
    const bookmarkRequest: BookmarkStrategyRequest = {
      expression: formProps.factorExpression,
      name: formProps.factorName,
      backtestStart: formProps.backtestStart,
      backtestEnd: formProps.backtestEnd,
      rebalanceInterval: formProps.samplingIntervalUnit,
      numAssets: formProps.numSymbols,
      assetUniverse: formProps.assetUniverse,
      bookmark: bookmarked, // this is ignored
    }
    try {
      const response = await fetch(endpoint + "/isStrategyBookmarked", {
        method: "POST",
        headers: {
          "Authorization": user ? "Bearer " + user.accessToken : ""
        },
        body: JSON.stringify(bookmarkRequest)
      });
      if (!response.ok) {
        const j = await response.json()
        // alert(j.error)
        console.error("Error submitting data:", response.status);
      } else {
        const j = await response.json()
        return j;
      }
    } catch (error) {
      // alert((error as Error).message)
      console.error("Error:", error);
    }
    return false;
  };

  const updateBookmarked = async (user: GoogleAuthUser, bookmark: boolean) => {
    setBookmarked(bookmark)
    const bookmarkRequest: BookmarkStrategyRequest = {
      expression: formProps.factorExpression,
      name: formProps.factorName,
      backtestStart: formProps.backtestStart,
      backtestEnd: formProps.backtestEnd,
      rebalanceInterval: formProps.samplingIntervalUnit,
      numAssets: formProps.numSymbols,
      assetUniverse: formProps.assetUniverse,
      bookmark,
    }
    try {
      const response = await fetch(endpoint + "/bookmarkStrategy", {
        method: "POST",
        headers: {
          "Authorization": user ? "Bearer " + user.accessToken : ""
        },
        body: JSON.stringify(bookmarkRequest)
      });
      if (!response.ok) {
        const j = await response.json()
        alert(j.error)
        console.error("Error submitting data:", response.status);
      }
    } catch (error) {
      alert((error as Error).message)
      console.error("Error:", error);
    }
  };

  const icon = bookmarked ? <FaBookmark size={20} style={{ cursor: "pointer" }} /> : <FaRegBookmark size={20} style={{ cursor: "pointer" }} />

  // todo - centralize this function
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

      figureOutNext(bookmarked, newUser)

    },
    onError: (error) => console.log('Login Failed:', error)
  });

  // they clicked when the state was X, so probably need to toggle
  async function figureOutNext(currentBookmarkState: boolean, user: GoogleAuthUser) {
    const response = await getIsBookmarked(user);
    const actualState = response.isBookmarked;
    const name = response.name;
    if (actualState !== true && actualState !== false) {
      // TODO - how the f
      return
    }
    // it wasn't saved before, they don't have it saved
    // pop open modal and go through confirmation process
    // the modal will need to update bookmarked, once
    // they add name etc
    if (!currentBookmarkState && !actualState) {
      setShowBookmarkModal(true)
      // formProps.setSelectedFactor(name)
      return
    }
    // if it wasn't flagged before, but they're actually
    // saved, just update it
    // it should be rare for it to be bookmarked
    // but not actually saved
    if (currentBookmarkState != actualState) {
      setBookmarked(actualState)
      if (actualState) {
        // formProps.setSelectedFactor(name)
      }
      return
    }
    // no drama - just remove the bookmark
    if (currentBookmarkState && actualState) {
      setBookmarked(false)
      updateBookmarked(user, false)
    }
  }

  const onClick = () => {
    if (!user) {
      // login calls updateBookmarked
      login()
    } else {
      // updateBookmarked(user, !bookmarked)
      figureOutNext(bookmarked, user)
    }
  }

  useEffect(() => {
    if (user) {
      // we might wanna change this - 
      // basically triggers every time the form
      // input changes to figure out if it's bookmarked
      getIsBookmarked(user).then(resp => {
        // console.log(resp)
        setBookmarked(resp.isBookmarked)
        // formProps.setSelectedFactor(resp.name)
      })
    } else {
      setBookmarked(false)
    }
  }, [user, formProps])

  return (
    <>
      <div
        className={formStyles.bookmark_container}
        data-tooltip-id="bookmark-tooltip"
        data-tooltip-content={toolTipMessage}
        data-tooltip-place="bottom"
        onClick={onClick}
      >
        {icon}
        <ReactTooltip id="bookmark-tooltip" />
      </div>
      <BookmarkModal
        show={showBookmarkModal}
        close={() => setShowBookmarkModal(false)}
        factorName={formProps.factorName}
        setFactorName={formProps.setFactorName}
        // updateName={formProps.updateName}
        bookmarkStategy={async () => {
          if (user) {
            setBookmarked(true)
            await updateBookmarked(user, true)
            await formProps.getStrategies();
            // console.log(fa)
            formProps.setSelectedFactor(formProps.factorName)
          } else {
            // should be impossible
            alert("must be logged in")
          }
        }}
      />
    </>
  )
}

function BookmarkModal({
  show,
  close,
  factorName,
  setFactorName,
  bookmarkStategy,
  // onSubmit,
}: {
  show: boolean;
  close: () => void;
  factorName: string,
  setFactorName: React.Dispatch<SetStateAction<string>>;
  bookmarkStategy: () => void;
  // user: GoogleAuthUser | null,
  // onSubmit: () => Promise<void>
}) {
  if (!show) return null;

  const handleOverlayClick = (e: any) => {
    if (e.target.id === "contact-modal") {
      close();
    }
  };

  return (
    <div id="contact-modal" className={modalsStyle.modal} onClick={handleOverlayClick}>
      <div className={modalsStyle.modal_content}>
        <span onClick={() => close()} className={modalsStyle.close} id="closeModalBtn">&times;</span>
        <h2 style={{ marginBottom: "40px" }}>Bookmark Strategy</h2>
        <form onSubmit={() => {
          bookmarkStategy();
          close();
        }}>
          <div>
            <label className={formStyles.label}>Strategy Name</label>
            <input
              type="text"
              value={factorName}
              className={modalsStyle.contact_form_email_input}
              onChange={(e) => {
                setFactorName(e.target.value)
              }}
            />
          </div>
          <button className={formStyles.backtest_btn} type='submit'>Submit</button>
        </form>
      </div>
    </div>
  );
}
