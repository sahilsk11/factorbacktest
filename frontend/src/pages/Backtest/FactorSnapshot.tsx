import { endpoint, FactorData } from "../../App";
import { BacktestInputs, BacktestSnapshot, GetSavedStrategiesResponse, GoogleAuthUser, InvestInStrategyRequest, LatestHoldings } from "../../models";
import { Doughnut } from 'react-chartjs-2';
import {
  Chart as ChartJS,
  ArcElement,
  Tooltip,
  Legend,
  ChartOptions
} from 'chart.js';
import appStyles from "../../App.module.css";
import factorSnapshotStyles from "./FactorSnapshot.module.css";
import { AiOutlineQuestionCircle } from 'react-icons/ai';
import { Tooltip as ReactTooltip } from 'react-tooltip';
import 'react-tooltip/dist/react-tooltip.css'
import { Col, Container, Nav, Pagination, Row } from "react-bootstrap";
import { Dispatch, SetStateAction, useEffect, useState } from "react";
import formStyles from "./Form.module.css";
import { parseDateString } from "../../util";
import { getStrategies, updateBookmarked } from "./Form";
import modalsStyle from "common/Modals.module.css";
import { useGoogleLogin } from "@react-oauth/google";
import ConfettiExplosion from "react-confetti-explosion";
import { useNavigate } from "react-router-dom";

export default function Inspector({
  fdIndex,
  fdDate,
  factorData,
  updateInspectFactorDataIndex,
  updateInspectFactorDataDate,
  user,
  latestHoldings,
  bookmarked,
  setBookmarked,
  backtestInputs,
  setFactorName,
  setSavedStrategies,
  setSelectedFactor,
  setUser,
}: {
  fdIndex: number | null;
  fdDate: string | null;
  factorData: FactorData[];
  updateInspectFactorDataIndex: (newVal: number) => void;
  updateInspectFactorDataDate: Dispatch<SetStateAction<string | null>>;
  user: GoogleAuthUser | null,
  latestHoldings: LatestHoldings | null,
  bookmarked: boolean,
  setBookmarked: Dispatch<SetStateAction<boolean>>,
  backtestInputs: BacktestInputs,
  setFactorName: Dispatch<SetStateAction<string>>,
  setSavedStrategies: Dispatch<SetStateAction<GetSavedStrategiesResponse[]>>,
  setSelectedFactor: Dispatch<SetStateAction<string>>,
  setUser: Dispatch<SetStateAction<GoogleAuthUser | null>>,
}) {
  const [selectedTab, setSelectedTab] = useState<string>("holdings");

  if (fdIndex === null || fdDate === null || factorData.length === 0) {
    return null;
  }

  const selectedComponent = {
    "holdings": <InspectFactorData
      fdIndex={fdIndex}
      fdDate={fdDate}
      factorData={factorData}
      updateInspectFactorDataIndex={updateInspectFactorDataIndex}
      updateInspectFactorDataDate={updateInspectFactorDataDate}
    />,
    "metrics": <p>coming soon!</p>,
    "invest": <Invest
      user={user}
      setUser={setUser}
      fdIndex={fdIndex}
      factorData={factorData}
      updateInspectFactorDataIndex={updateInspectFactorDataIndex}
      latestHoldings={latestHoldings}
      bookmarked={bookmarked}
      setBookmarked={setBookmarked}
      backtestInputs={backtestInputs}
      setFactorName={setFactorName}
      setSavedStrategies={setSavedStrategies}
      setSelectedFactor={setSelectedFactor}
    />,
  }[selectedTab] || null;

  // return <div className={`${appStyles.tile} ${factorSnapshotStyles.fs_container}`}>{selectedComponent}</div>

  return (
    <>
      <div className={`${appStyles.tile} ${factorSnapshotStyles.fs_container}`}>
        <Nav variant="tabs" activeKey={selectedTab}>
          <Nav.Item>
            <Nav.Link className={`
              ${factorSnapshotStyles.tab_title} 
              ${selectedTab === "holdings" ? factorSnapshotStyles.tab_title_active : ""}
            `} onClick={() => setSelectedTab("holdings")} eventKey="holdings">Holdings History</Nav.Link>
          </Nav.Item>
          <Nav.Item>
            <Nav.Link className={`
              ${factorSnapshotStyles.tab_title} 
              ${selectedTab === "invest" ? factorSnapshotStyles.tab_title_active : ""}
            `} onClick={() => setSelectedTab("invest")} eventKey="invest">Invest</Nav.Link>
          </Nav.Item>
          {/* <Nav.Item>
            <Nav.Link onClick={() => setSelectedTab("metrics")} eventKey="metrics">Performance Metrics</Nav.Link>
          </Nav.Item> */}
        </Nav>
        {selectedComponent}
      </div>
    </>
  )
}

function Invest({
  user,
  setUser,
  fdIndex,
  updateInspectFactorDataIndex,
  factorData,
  latestHoldings,
  bookmarked,
  setBookmarked,
  backtestInputs,
  setFactorName,
  setSelectedFactor,
  setSavedStrategies
}: {
  user: GoogleAuthUser | null,
  setUser: Dispatch<SetStateAction<GoogleAuthUser | null>>,
  fdIndex: number,
  updateInspectFactorDataIndex: (newVal: number) => void,
  factorData: FactorData[],
  latestHoldings: LatestHoldings | null,
  bookmarked: boolean,
  setBookmarked: Dispatch<SetStateAction<boolean>>,
  backtestInputs: BacktestInputs,
  setFactorName: Dispatch<SetStateAction<string>>,
  setSelectedFactor: Dispatch<SetStateAction<string>>,
  setSavedStrategies: Dispatch<SetStateAction<GetSavedStrategiesResponse[]>>,
}) {
  const [depositAmount, setDepositAmount] = useState(10);
  const [showInvestModal, setShowInvestModal] = useState(false);

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

      setShowInvestModal(true)

    },
    onError: (error) => console.log('Login Failed:', error)
  });

  function deposit(e: any) {
    e.preventDefault()
    if (user) {
      // maybe bookmark strategy
      setShowInvestModal(true)
    } else {
      login()
    }

  }

  if (!latestHoldings) {
    return null;
  }

  const selector = factorData.length > 1 ? <StrategyNamesSelector fdIndex={fdIndex} updateInspectFactorDataIndex={updateInspectFactorDataIndex} factorData={factorData} /> : null;
  const sortedSymbols = Object.keys(latestHoldings.assets).sort((a, b) => latestHoldings.assets[b].assetWeight - latestHoldings.assets[a].assetWeight);

  const {
    factorName,
    factorExpression,
    assetUniverse,
  } = backtestInputs;

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

  return (
    <>
      <Container style={{ paddingTop: "10px" }}>
        {selector}

        <Row style={{ marginTop: "10px" }}>
          <Col md={6} className={factorSnapshotStyles.latest_holdings_container}>
            <p className={factorSnapshotStyles.invest_title}>Invest in Strategy</p>
            <p className={`${appStyles.subtext} ${factorSnapshotStyles.subtext}`}>Paper trade or deposit real funds</p>


            <form onSubmit={deposit}>
              <input
                // id="cash"
                className={factorSnapshotStyles.deposit_input}
                value={"$ " + depositAmount.toLocaleString()}
                style={{ paddingLeft: "5px" }}
                onChange={(e) => updateDepositAmount(e)}
              />
              <button className={`${formStyles.backtest_btn} ${factorSnapshotStyles.deposit_btn}`} type="submit">Start</button>
            </form>

            {/* <i><p style={{textAlign:"center", fontSize:"10px"}} className={appStyles.subtext}>Past performance is not indicative of future results.<br/>May lose value.</p></i> */}

          </Col>
          <Col md={6} style={{ paddingTop: "10px" }}>
            <p className={factorSnapshotStyles.invest_title}>Latest Holdings</p>
            <p className={`${appStyles.subtext} ${factorSnapshotStyles.subtext}`}>Based on market data from {parseDateString(latestHoldings.date)}</p>

            <table className={factorSnapshotStyles.table}>
              <thead>
                <tr>
                  <th>Symbol</th>
                  <th>Factor Score</th>
                  <th>Portfolio Allocation</th>

                </tr>
              </thead>
              <tbody>
                {sortedSymbols.map((symbol, i) => <tr key={i}>
                  <td>{symbol}</td>
                  <td>{latestHoldings.assets[symbol].factorScore < 1e-2 ? latestHoldings.assets[symbol].factorScore.toExponential(2) : latestHoldings.assets[symbol].factorScore.toFixed(2)}</td>
                  <td>{(100 * latestHoldings.assets[symbol].assetWeight).toFixed(2)}%</td>
                </tr>)}
              </tbody>
            </table >
          </Col>
        </Row>
      </Container>
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
            className={modalsStyle.contact_form_email_input}
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
          You're all set. Track your investments <a href="" onClick={() => navigate("/investments")}>here</a>.
        </div>
        {/* <button className={formStyles.backtest_btn} type='submit'>Submit</button> */}
      </>),
      onComplete: () => { },
      canProceed: false,
    },
  ]

  return (
    <div id="invest-modal" className={modalsStyle.modal} onClick={handleOverlayClick}>
      <div className={modalsStyle.modal_content}>
        <span onClick={() => closeWrapper()} className={modalsStyle.close} id="closeInvestModalBtn">&times;</span>
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


function InspectFactorData({
  fdIndex,
  fdDate,
  factorData,
  updateInspectFactorDataIndex,
  updateInspectFactorDataDate,
}: {
  fdIndex: number;
  fdDate: string;
  factorData: FactorData[];
  updateInspectFactorDataIndex: (newVal: number) => void;
  updateInspectFactorDataDate: Dispatch<SetStateAction<string | null>>;
}) {

  const strategyNamesSelector = <StrategyNamesSelector fdIndex={fdIndex} updateInspectFactorDataIndex={updateInspectFactorDataIndex} factorData={factorData} />

  const dateSelector = <select value={fdDate} onChange={e => updateInspectFactorDataDate(e.target.value)}>
    {Object.keys(factorData[fdIndex].data).map((dateStr, i) =>
      <option value={dateStr} key={i}>{dateStr}</option>
    )}
  </select>

  const fdDetails = factorData[fdIndex];
  const fdData = fdDetails.data[fdDate];
  // TODO - make this a one-liner
  const snapshotToAssetWeight = (snapshot: BacktestSnapshot): Record<string, number> => {
    let out: Record<string, number> = {}
    Object.keys(snapshot.assetMetrics).forEach(symbol => {
      out[symbol] = snapshot.assetMetrics[symbol].assetWeight
    })
    return out;
  };

  return <>
    <div style={{ margin: "0px auto", display: "block" }}>
      {/* <h3 style={{ marginBottom: "0px", marginTop: "0px" }}>Factor Snapshot</h3> */}
      <i><p className={appStyles.subtext}>What did {strategyNamesSelector} hold on {dateSelector} ?</p></i>
      <div className={appStyles.my_container} style={{ marginTop: "30px", width: "100%", minHeight: "0px", alignItems: "center" }}>
        <div className={appStyles.column} style={{ "flexGrow": 5, maxWidth: "600px" }}>
          <AssetAllocationTable snapshot={fdData} />
        </div>
        <div className={appStyles.column} style={{ "flexGrow": 2 }}>
          <div className={factorSnapshotStyles.chart_container}>
            <AssetBreakdown assetWeights={snapshotToAssetWeight(fdData)} />
            <h5 style={{ textAlign: "center" }}>Asset Allocation Breakdown</h5>

          </div>
        </div>
      </div>
    </div>

  </>
}

const AssetAllocationTable = ({ snapshot }: { snapshot: BacktestSnapshot }) => {
  const sortedSymbols = Object.keys(snapshot.assetMetrics).sort((a, b) => snapshot.assetMetrics[b].assetWeight - snapshot.assetMetrics[a].assetWeight);
  const toolTipMessage = `Indicates asset performance (% return) between the current date (${snapshot.date}) and the next rebalance.`
  return (
    <table className={factorSnapshotStyles.table}>
      <thead>
        <tr>
          <th>Symbol</th>
          <th>Factor Score</th>
          <th>Portfolio Allocation</th>
          <th>
            Price Change til Next Rebalance
            <a
              data-tooltip-id="my-tooltip"
              data-tooltip-content={toolTipMessage}
              data-tooltip-place="bottom"
              style={{
                paddingLeft: "5px",
                marginTop: "2px",
                height: "100%",
                // "border": "1px solid red"
              }}
            >
              <AiOutlineQuestionCircle className="question-icon" />
            </a>
            <ReactTooltip id="my-tooltip" />
          </th>

        </tr>
      </thead>
      <tbody>
        {sortedSymbols.map((symbol, i) => <tr key={i}>
          <td>{symbol}</td>
          <td>{snapshot.assetMetrics[symbol].factorScore < 1e-2 ? snapshot.assetMetrics[symbol].factorScore.toExponential(2) : snapshot.assetMetrics[symbol].factorScore.toFixed(2)}</td>
          <td>{(100 * snapshot.assetMetrics[symbol].assetWeight).toFixed(2)}%</td>
          <td>{snapshot.assetMetrics[symbol].priceChangeTilNextResampling?.toFixed(2)}%</td>
        </tr>)}
      </tbody>
    </table >
  );
};

ChartJS.register(ArcElement, Tooltip, Legend);

export const AssetBreakdown = ({
  assetWeights
}: {
  assetWeights: Record<string, number>
}) => {
  const assetData = Object.keys(assetWeights).map((key) => ({
    asset: key,
    allocation: assetWeights[key] * 100,
  }));

  // Sort the assetData array by allocation (percentage) in descending order
  assetData.sort((a, b) => b.allocation - a.allocation);

  // Extract the sorted keys and data values
  const labels = assetData.map((item) => item.asset);
  const dataValues = assetData.map((item) => item.allocation);

  const options: ChartOptions<"doughnut"> = {
    plugins: {
      legend: {
        display: false,
        position: "right"
      },
    },
  };


  const data = {
    labels,
    datasets: [
      {
        label: '% Allocation',
        data: dataValues,
        borderWidth: 1,
      },
    ],
  };

  return <Doughnut data={data} options={options} />;
}
function StrategyNamesSelector({
  fdIndex,
  updateInspectFactorDataIndex,
  factorData,
}:
  {
    fdIndex: number,
    updateInspectFactorDataIndex: (newVal: number) => void,
    factorData: FactorData[]
  }) {
  return <select value={fdIndex} onChange={(e) => updateInspectFactorDataIndex(Number(e.target.value))}>
    {factorData.map((fd, i) => <option value={i} key={i}>
      {fd.name}
    </option>)}
  </select>;
}

