import { FactorData, BenchmarkData, endpoint } from "App";
import { GoogleAuthUser, LatestHoldings, GetSavedStrategiesResponse, BacktestInputs } from "models";
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

  const [searchParams, setSearchParams] = useSearchParams();
  
  async function getStrategy(id:string): Promise<GetSavedStrategiesResponse | null> {
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
        return j.find(e => e.savedStrategyID === id) || null
      }
    } catch (error) {
      alert((error as Error).message)
      console.error("Error:", error);
    }
    return null;
  }

  async function setFromUrl(id:string) {
    const strat = await getStrategy(id)
    console.log(strat)
    if (!strat) {
      return
    }
    setFactorName(strat.strategyName)
    setNumSymbols(strat.numAssets)
    setFactorExpression(strat.factorExpression)
    setAssetUniverse(strat.assetUniverse)
    setBacktestStart(formatDate(new Date(strat.backtestStart)))
    setBacktestEnd(formatDate(new Date(strat.backtestEnd)))
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
            <Container style={{
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
                  />
                </Col>
                <Col sm={6}>
                  <Stats />
                </Col>
              </Row>
            </Container>
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

function Stats() {
  return (
    <div className={`${backtestStyles.flex_container} ${styles.tile}`}>
      <p className={backtestStyles.flex_container_title}>Performance History</p>
      <p className={`${styles.subtext} ${backtestStyles.flex_container_subtext}`}>From 2023-01-01 to 2023-01-01</p>
      <div style={{ paddingBottom: "0px" }}>
        <Table>
          <tbody>
            <tr style={{ borderTop: "1px solid #DFE2E6" }}>
              <th className={backtestStyles.stats_table_header}>Return</th>
              <td className={backtestStyles.stats_table_value}>20%</td>
            </tr>
            <tr>
              <th className={backtestStyles.stats_table_header}>Sharpe Ratio</th>
              <td className={backtestStyles.stats_table_value}>1.5</td>
            </tr>
            <tr>
              <th className={backtestStyles.stats_table_header}>Volatilty (stdev)</th>
              <td className={backtestStyles.stats_table_value}>20%</td>
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

function twoYearsAgoAsString() {
  const today = new Date();
  const year = today.getFullYear() - 2;
  const month = String(today.getMonth() + 1).padStart(2, '0'); // Months are 0-based, so add 1
  const day = String(today.getDate()).padStart(2, '0');

  return `${year}-${month}-${day}`;
}