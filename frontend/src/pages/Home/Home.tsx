import StatsFooter from "common/Footer";
import { Nav } from "common/Nav";
import { GoogleAuthUser } from "models";
import { useState } from "react";
import appStyles from "../../App.module.css";
import homeStyles from "./Home.module.css";
import { Card, Container, ListGroup, Row } from "react-bootstrap";

export function Home({
  user,
  setUser,
}: {
  user: GoogleAuthUser | null,
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>;
}) {
  const [showHelpModal, setShowHelpModal] = useState(false);
  const [showContactModal, setShowContactModal] = useState(false);

  return <>
    <Nav loggedIn={user !== null} setUser={setUser} showLinks={false} setShowHelpModal={setShowHelpModal} setShowContactModal={setShowContactModal} />

    <div className={`${appStyles.tile} ${homeStyles.container}`}>
      <h2 style={{ marginBottom: "0px" }}>Factor Backtest</h2>
      <p className={homeStyles.verbose_builder_subtitle}>Create and backtest factor-based investment strategies.</p>

      <button className={`fb_btn ${homeStyles.new_btn}`}>Create Strategy +</button>

      <Container>
        <Row>
          <StrategyCard />
          <StrategyCard />
          <StrategyCard />
          <StrategyCard />
          <StrategyCard />
          <StrategyCard />
          <StrategyCard />
          <StrategyCard />
          <StrategyCard />
        </Row>
      </Container>
    </div>
  </>
}

function StrategyCard({

}: {

  }) {
  const weights: Record<string, number> = {};
  // stats.holdings.forEach(h => {
  //   weights[h.symbol] = h.marketValue / stats.currentValue
  // })



  return (
    <>
      <Card style={{ width: '18rem', marginRight: "20px" }} className="col-sm-6 mb-3 mb-sm-0">
        <div style={{
          width: "50%",
          margin: "0px auto",
          display: "block",
          marginTop: "10px"
        }} >
          <p style={{ textAlign: "center", marginTop: "65px", marginBottom: "50px" }} className={appStyles.subtext}>no data yet</p>
        </div>
        <Card.Body>
          <Card.Text style={{ textAlign: "center" }}>name</Card.Text>
          {/* <Card.Text>
            Some quick example text to build on the card title and make up the
            bulk of the card's content.
          </Card.Text> */}
        </Card.Body>
        <ListGroup className="list-group-flush">
          <ListGroup.Item>Total Return: 10%</ListGroup.Item>
          <ListGroup.Item>Current Value: ${10}</ListGroup.Item>
          <ListGroup.Item>Inception: 2020-01-01</ListGroup.Item>
          <ListGroup.Item>Last Trade: heu</ListGroup.Item>
        </ListGroup>
      </Card>
    </>
  )
}

