import { FactorData, BenchmarkData, endpoint } from "App";
import { GoogleAuthUser, LatestHoldings, GetSavedStrategiesResponse, BacktestInputs, GetPublishedStrategiesResponse, PerformanceMetrics } from "models";
import { useState, useEffect } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { formatDate, minMaxDates } from "../../util";
import BacktestChart from "./BacktestChart";
import BenchmarkManager from "./BenchmarkSelector";
import Inspector from "./FactorSnapshot";
import FactorForm, { getStrategies } from "./Form";
import styles from 'App.module.css'
import { InvestInStrategy } from "./InvestInStrategy";
import { Col, Container, Row, Table } from "react-bootstrap";
import "./Backtest.module.css";
import backtestStyles from "./Backtest.module.css";
import formStyles from "./Form.module.css";
import { useAuth } from "auth";


export default function FactorBacktestMain({ userID, user, setUser }: {
  userID: string
  user: GoogleAuthUser | null,
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>,
}) {
  const [factorData, updateFactorData] = useState<FactorData[]>([]);
  const [benchmarkData, updateBenchmarkData] = useState<BenchmarkData[]>([]);
  const [inspectFactorDataIndex, updateInspectFactorDataIndex] = useState<number | null>(null);
  const [inspectFactorDataDate, updateInspectFactorDataDate] = useState<string | null>(null);
  const [latestHoldings, setLatestHoldings] = useState<LatestHoldings | null>(null);
  const [metrics, setMetrics] = useState<PerformanceMetrics | null>(null);
  const [assetUniverse, setAssetUniverse] = useState<string>("--");

  const [bookmarked, setBookmarked] = useState(false);
  const [savedStrategies, setSavedStrategies] = useState<GetSavedStrategiesResponse[]>([]);
  const [lastStrategyID, setLastStrategyID] = useState<string | null>(null);


  // everything related to inputs pmuch
  const [numSymbols, setNumSymbols] = useState(10);
  const [factorExpression, setFactorExpression] = useState(`pricePercentChange(
  nDaysAgo(7),
  currentDate
)`);
  const [factorName, setFactorName] = useState("7_day_momentum_weekly");
  const [backtestStart, setBacktestStart] = useState(threeYearsAgoAsString());
  const [backtestEnd, setBacktestEnd] = useState(todayAsString());
  const [samplingIntervalUnit, setSamplingIntervalUnit] = useState("monthly");
  const [selectedFactor, setSelectedFactor] = useState("momentum");
  // super hacky until we can refactor
  const [runBacktestToggle, setRunBacktestToggle] = useState(false);

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
    // navigate("/")
  }

  const { session } = useAuth()

  const [searchParams, setSearchParams] = useSearchParams();

  async function getStrategy(id: string): Promise<GetPublishedStrategiesResponse | null> {
    try {
      const response = await fetch(endpoint + "/publishedStrategies", {
        headers: {
          "Authorization": session ? "Bearer " + session.access_token : ""
        }
      });
      if (!response.ok) {
        const j = await response.json()
        alert(j.error)
        console.error("Error submitting data:", response.status);
      } else {
        const j = await response.json() as GetPublishedStrategiesResponse[];
        return j.find(e => e.strategyID === id) || null
      }
    } catch (error) {
      alert((error as Error).message)
      console.error("Error:", error);
    }
    return null;
  }

  async function setFromUrl(id: string) {
    const strat = await getStrategy(id)
    if (!strat) {
      return
    }
    setFactorName(strat.strategyName)
    setNumSymbols(strat.numAssets)
    setFactorExpression(strat.factorExpression)
    setAssetUniverse(strat.assetUniverse)
    // Optional URL override for backtest start date: /backtest?id=<strategyID>&start=YYYY-MM-DD
    const startParam = searchParams.get("start")
    if (startParam) {
      // Validate the date; fall back to existing state if invalid.
      const parsed = new Date(startParam)
      if (!isNaN(parsed.getTime())) {
        setBacktestStart(formatDate(parsed))
      }
    }
    setSamplingIntervalUnit(strat.rebalanceInterval)
    setSelectedFactor(strat.strategyName)

    await new Promise(f => setTimeout(f, 500));
    setRunBacktestToggle(true);
  }


  useEffect(() => {
    const id = searchParams.get("id")
    if (id) {
      setFromUrl(id)
    }
  }, [searchParams])

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

  const handleClear = () => {
    // Clear backtest results and related view state, but keep user inputs.
    updateFactorData([]);
    updateBenchmarkData([]);
    updateInspectFactorDataIndex(null);
    updateInspectFactorDataDate(null);
    setLatestHoldings(null);
    setMetrics(null);
    setBookmarked(false);
    setLastStrategyID(null);
    // Clear any URL params like id/start so the next load behaves like a fresh page.
    setSearchParams(new URLSearchParams());
  };

  return (
    <>
      <div className={styles.my_container}>
        <div className={`${styles.column} ${styles.form_wrapper}`}>
          <FactorForm
            // set this to the benchmark names that are already in used
            user={user}
            userID={userID}
            takenNames={takenNames}
            setMetrics={setMetrics}
            appendFactorData={(newFactorData: FactorData) => {
              updateFactorData([...factorData, newFactorData])
            }}
            runBacktestToggle={runBacktestToggle}
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
            setLastStrategyID={setLastStrategyID}
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
          <div
            id="backtest-chart"
            className={`${styles.column} ${styles.backtest_chart_container}`}
            style={{ position: "relative" }}
          >
            <BacktestChart
              benchmarkData={benchmarkData}
              factorData={factorData}
              updateInspectFactorDataIndex={updateFdIndex}
              updateInspectFactorDataDate={updateInspectFactorDataDate}
            />
            <div
              style={{
                position: "absolute",
                top: -15,
                right: 24,
                zIndex: 10,
              }}
            >
              <button
                className={formStyles.backtest_btn}
                style={{
                  width: "54px",
                  height: "22px",
                  padding: "4px 12px",
                  fontSize: "10px",
                  boxShadow: "none",
                }}
                onClick={handleClear}
              >
                Clear
              </button>
            </div>
            {lastStrategyID ? <Container style={{
              width: "93%",
              margin: "0px auto",
              marginTop: "20px"
            }}>
              <Row>
                <Col sm={6}>
                  <InvestInStrategy
                    user={user}
                    setUser={setUser}
                    bookmarked={bookmarked}
                    setBookmarked={setBookmarked}
                    backtestInputs={backtestInputs}
                    setFactorName={setFactorName}
                    setSelectedFactor={setSelectedFactor}
                    setSavedStrategies={setSavedStrategies}
                    strategyID={lastStrategyID}
                  />
                </Col>
                <Col sm={6}>
                  <Stats metrics={metrics} />
                </Col>
              </Row>
            </Container> : null}
            {/* <div > */}
            {/* </div> */}
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
              setUser={setUser}
            />
          </div> : null}
      </div >
    </>
  )


  // return (
  //   <div className={styles.my_container}>
  //     {useVerboseBuilder ? formComponent : classicView}
  //   </div >
  // );
}

function Stats({
  metrics
}: {
  metrics: PerformanceMetrics | null
}) {
  let returnsText = "n/a";
  if (metrics?.annualizedReturn) {
    returnsText = (100 * metrics.annualizedReturn).toFixed(2).toString() + "%"
  }
  let stdevText = "n/a";
  if (metrics?.annualizedStandardDeviation) {
    stdevText = (100 * metrics.annualizedStandardDeviation).toFixed(2).toString() + "%"
  }
  let sharpeText = "n/a";
  if (metrics?.sharpeRatio) {
    sharpeText = metrics.sharpeRatio.toFixed(2).toString()
  }
  return (
    <div className={`${backtestStyles.flex_container} ${styles.tile}`}>
      <p className={backtestStyles.flex_container_title}>Performance History</p>
      {/* <p className={`${styles.subtext} ${backtestStyles.flex_container_subtext}`}>From 2023-01-01 to 2023-01-01</p> */}
      <div style={{ paddingBottom: "0px", marginTop: "10px" }}>
        <Table>
          <tbody>
            <tr style={{ borderTop: "1px solid #DFE2E6" }}>
              <th className={backtestStyles.stats_table_header}>Annualized Return</th>
              <td className={backtestStyles.stats_table_value}>{returnsText}</td>
            </tr>
            <tr>
              <th className={backtestStyles.stats_table_header}>Sharpe Ratio</th>
              <td className={backtestStyles.stats_table_value}>{sharpeText}</td>
            </tr>
            <tr>
              <th className={backtestStyles.stats_table_header}>Annualized Volatilty (stdev)</th>
              <td className={backtestStyles.stats_table_value}>{stdevText}</td>
            </tr>
          </tbody>
        </Table>
      </div>
    </div>
  )
}

function todayAsString() {
  const today = new Date();
  const year = today.getFullYear();
  const month = String(today.getMonth() + 1).padStart(2, '0'); // Months are 0-based, so add 1
  const day = String(today.getDate()).padStart(2, '0');

  return `${year}-${month}-${day}`;
}

function threeYearsAgoAsString() {
  const today = new Date();
  const year = today.getFullYear() - 3;
  const month = String(today.getMonth() + 1).padStart(2, '0'); // Months are 0-based, so add 1
  const day = String(today.getDate()).padStart(2, '0');

  return `${year}-${month}-${day}`;
}