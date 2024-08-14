import { GoogleAuthUser, GetSavedStrategiesResponse, InvestInStrategyRequest, GetInvestmentsResponse } from "../../models";
import investStyles from "./Invest.module.css";
import appStyles from "../../App.module.css";
import { Nav } from "common/Nav"
import { Dispatch, SetStateAction, useEffect, useState } from "react";
import { ContactModal, HelpModal } from "common/Modals";
import { Card, ListGroup, Row, Table } from "react-bootstrap";
import { endpoint } from "App";
import { formatDate } from "../../util";
import { useNavigate, useSearchParams } from "react-router-dom";

import { AssetBreakdown } from "../Backtest/FactorSnapshot";
import { useAuth } from "auth";
import LoginModal from "common/AuthModals";

export default function Invest({
  user,
  setUser,
}: {
  user: GoogleAuthUser | null,
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>;
}) {
  const [showHelpModal, setShowHelpModal] = useState(false);
  const [showContactModal, setShowContactModal] = useState(false);
  const [activeInvestments, setActiveInvestments] = useState<GetInvestmentsResponse[]>([]);
  const [showLoginModal, setShowLoginModal] = useState(false);

  const { session } = useAuth();

  useEffect(() => {
    if (session) {
      getInvestments()
    } else {
      setActiveInvestments([]);
      setShowLoginModal(true);
    }
  }, [session])

  const navigate = useNavigate();

  async function getInvestments() {
    try {
      const response = await fetch(endpoint + "/activeInvestments", {
        headers: {
          "Authorization": session ? "Bearer " + session.access_token : ""
        }
      });
      if (!response.ok) {
        const j = await response.json()
        alert(j.error)
        console.error("Error submitting data:", response.status);
      } else {
        const j = await response.json()
        setActiveInvestments(j)
      }
    } catch (error) {
      alert((error as Error).message)
      console.error("Error:", error);
    }
  }

  const tiles = activeInvestments.map((e) => <InvestmentTile stats={e} />)

  return <>
    <Nav loggedIn={user !== null} setUser={setUser} showLinks={false} />

    {/* <InvestInStrategy user={user} setCheckForNewInvestments={setCheckForNewInvestments} /> */}
    {/* <ActiveInvestments user={user} checkForNewInvestments={checkForNewInvestments} setCheckForNewInvestments={setCheckForNewInvestments} /> */}

    {/* {activeInvestments} */}

    {showLoginModal ? <LoginModal show={showLoginModal} close={() => {
      setShowLoginModal(false);
      if (!session) {
        navigate("/")
      }
    }} /> : null}

    <div className={`${appStyles.tile} ${investStyles.container}`}>
      <h2 style={{ marginBottom: "0px" }}>Active Investments</h2>
      <p className={appStyles.subtext}>Deposit funds into any strategy you've previously tested.</p>
      <Row>
        {tiles}
      </Row>

    </div>
  </>
}

function InvestmentTile({
  stats
}: {
  stats: GetInvestmentsResponse
}) {
  const weights: Record<string, number> = {};
  stats.holdings.forEach(h => {
    weights[h.symbol] = h.marketValue / stats.currentValue
  })

  const latestTrade = stats.completedTrades.length > 0 ? formatDate(new Date(stats.completedTrades?.reduce((latest, trade) => {
    return trade.filledAt > latest.filledAt ? trade : latest;
  })?.filledAt)) : "n/a";

  return (
    <>
      <Card style={{ width: '18rem', marginRight: "20px" }} className="col-sm-6 mb-3 mb-sm-0">
        <div style={{
          width: "50%",
          margin: "0px auto",
          display: "block",
          marginTop: "10px"
        }} >
          {stats.completedTrades.length > 0 ?
            <AssetBreakdown assetWeights={weights} /> : <p style={{ textAlign: "center", marginTop: "65px", marginBottom: "50px" }} className={appStyles.subtext}>no data yet</p>}
        </div>
        <Card.Body>
          <Card.Text style={{ textAlign: "center" }}>{stats.strategy.strategyName}</Card.Text>
          {/* <Card.Text>
            Some quick example text to build on the card title and make up the
            bulk of the card's content.
          </Card.Text> */}
        </Card.Body>
        <ListGroup className="list-group-flush">
          <ListGroup.Item>Total Return: {stats.percentReturnFraction.toFixed(2)}%</ListGroup.Item>
          <ListGroup.Item>Current Value: ${stats.currentValue.toFixed(2)}</ListGroup.Item>
          <ListGroup.Item>Inception: {stats.startDate}</ListGroup.Item>
          <ListGroup.Item>Last Trade: {latestTrade}</ListGroup.Item>
        </ListGroup>
      </Card>
    </>
  )
}


