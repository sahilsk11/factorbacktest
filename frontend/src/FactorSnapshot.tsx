import { FactorData } from "./App";
import { BacktestInputs, BacktestSnapshot, GetSavedStrategiesResponse, GoogleAuthUser, LatestHoldings } from "./models";
import { Doughnut } from 'react-chartjs-2';
import {
  Chart as ChartJS,
  ArcElement,
  Tooltip,
  Legend,
  ChartOptions
} from 'chart.js';
import appStyles from "./App.module.css";
import factorSnapshotStyles from "./FactorSnapshot.module.css";
import { AiOutlineQuestionCircle } from 'react-icons/ai';
import { Tooltip as ReactTooltip } from 'react-tooltip';
import 'react-tooltip/dist/react-tooltip.css'
import { Col, Container, Nav, Row } from "react-bootstrap";
import { Dispatch, SetStateAction, useState } from "react";
import formStyles from "./Form.module.css";
import { parseDateString } from "./util";
import { getStrategies, updateBookmarked } from "./Form";
import modalsStyle from "./Modals.module.css";

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
  function deposit(e: any) {
    // maybe bookmark strategy
    e.preventDefault()
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

  return (
    <>
      <Container style={{ paddingTop: "10px" }}>
        {selector}

        <Row style={{ marginTop: "10px" }}>
          <Col md={6} className={factorSnapshotStyles.latest_holdings_container}>
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
          <Col md={6} style={{ paddingTop: "10px" }}>
            <p className={factorSnapshotStyles.invest_title}>Invest in Strategy</p>
            <p className={`${appStyles.subtext} ${factorSnapshotStyles.subtext}`}>Paper trade or deposit real funds</p>


            <form onSubmit={deposit}>
              <input
                // id="cash"
                className={factorSnapshotStyles.deposit_input}
                value={"$ " + depositAmount.toLocaleString()}
                style={{ paddingLeft: "5px" }}
                onChange={(e) => {
                  let x = e.target.value.replace(/,/g, '')
                  x = x.replace(/\$ /g, '')
                  if (x.length === 0) {
                    x = "0";
                  }
                  if (!/[^0-9]/.test(x) && x.length < 12) {
                    setDepositAmount(parseFloat(x))
                  }
                }}
              />
              <button className={`${formStyles.backtest_btn} ${factorSnapshotStyles.deposit_btn}`} type="submit">Start</button>
            </form>
          </Col>
        </Row>
      </Container>
      <InvestModal
        show={true}
        close={() => { }}
        factorName={factorName}
        setFactorName={setFactorName}
        bookmarkStategy={async () => {
          if (user) {
            setBookmarked(true)
            await updateBookmarked(user, true, backtestInputs)
            await getStrategies(user, setSavedStrategies);
            // console.log(fa)
            setSelectedFactor(factorName)
          } else {
            // should be impossible
            alert("must be logged in")
          }
        }}
      />
    </>
  )
}

function InvestModal({
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

const AssetBreakdown = ({
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

