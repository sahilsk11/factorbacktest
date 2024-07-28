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

  useEffect(() => {
    if (user) {
      console.log("got user")
      getInvestments()
    }
  }, [user])

  async function getInvestments() {
    if (!user) {
      alert("fhruei")
    }
    console.log(endpoint + "/activeInvestments")
    try {
      const response = await fetch(endpoint + "/activeInvestments", {
        headers: {
          "Authorization": user ? "Bearer " + user.accessToken : ""
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
      console.log("def this fuck")
      console.error("Error:", error);
    }
  }

  const tiles = activeInvestments.map((e) => <InvestmentTile stats={e} />)

  return <>
    <Nav loggedIn={user !== null} setUser={setUser} showLinks={false} setShowHelpModal={setShowHelpModal} setShowContactModal={setShowContactModal} />

    {/* <InvestInStrategy user={user} setCheckForNewInvestments={setCheckForNewInvestments} /> */}
    {/* <ActiveInvestments user={user} checkForNewInvestments={checkForNewInvestments} setCheckForNewInvestments={setCheckForNewInvestments} /> */}

    {/* {activeInvestments} */}

    <div className={`${appStyles.tile} ${investStyles.container}`}>
      <h2 style={{ marginBottom: "0px" }}>Active Investments</h2>
      <p className={appStyles.subtext}>Deposit funds into any strategy you've previously tested.</p>
      <Row>
        {tiles}
      </Row>

    </div>


    <ContactModal user={user} userID={""} show={showContactModal} close={() => setShowContactModal(false)} />
    <HelpModal show={showHelpModal} close={() => setShowHelpModal(false)} />
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
      <Card style={{ width: '18rem', marginRight:"20px" }} className="col-sm-6 mb-3 mb-sm-0">
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
          <Card.Text style={{ textAlign: "center" }}>{stats.savedStrategy.strategyName}</Card.Text>
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


