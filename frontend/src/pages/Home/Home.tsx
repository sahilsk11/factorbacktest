import StatsFooter from "common/Footer";
import { Nav } from "common/Nav";
import { GetPublishedStrategiesResponse, GoogleAuthUser } from "models";
import { useEffect, useState } from "react";
import appStyles from "../../App.module.css";
import homeStyles from "./Home.module.css";
import { Card, Container, ListGroup, Row } from "react-bootstrap";
import { useNavigate } from "react-router-dom";
import { endpoint } from "App";

export function Home({
  user,
  setUser,
}: {
  user: GoogleAuthUser | null,
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>;
}) {
  const [publishedStrategies, setPublishedStrategies] = useState<GetPublishedStrategiesResponse[]>([]);
  const [showHelpModal, setShowHelpModal] = useState(false);
  const [showContactModal, setShowContactModal] = useState(false);

  const navigate = useNavigate();

  async function getPublishedStrategies(): Promise<GetPublishedStrategiesResponse[]> {
    try {
      const response = await fetch(endpoint + "/publishedStrategies", {
        headers: {
          "Authorization": user ? "Bearer " + user.accessToken : ""
        }
      });
      if (!response.ok) {
        const j = await response.json()
        alert(j.error)
        console.error("Error submitting data:", response.status);
      } else {
        const j = await response.json() as GetPublishedStrategiesResponse[];
        return j
      }
    } catch (error) {
      alert((error as Error).message)
      console.error("Error:", error);
    }
    return []
  }

  useEffect(() => {
    (async () => {
      setPublishedStrategies(await getPublishedStrategies())
    })();

  }, []);

  const cards = publishedStrategies.map(ps => <StrategyCard data={ps} />)

  return <>
    <Nav loggedIn={user !== null} setUser={setUser} showLinks={false} setShowHelpModal={setShowHelpModal} setShowContactModal={setShowContactModal} />

    <div className={`${appStyles.tile} ${homeStyles.container}`}>
      <div className={homeStyles.title_container}>
        <h2 style={{ marginBottom: "0px" }}>Factor Backtest</h2>
        <p className={homeStyles.verbose_builder_subtitle}>Create and backtest factor-based investment strategies.</p>
      </div>

      <div className={homeStyles.btn_container}>
        <button
          className={`fb_btn ${homeStyles.new_btn}`}
          onClick={() => navigate("/backtest")}
        >Create Strategy +</button>
      </div>

      <Container className={homeStyles.card_container}>
        <Row style={{ display: "flex", justifyContent: "center" }}>
          {cards}
        </Row>
      </Container>
    </div>
  </>
}

function StrategyCard({
  data
}: {
  data: GetPublishedStrategiesResponse
}) {
  const weights: Record<string, number> = {};
  // stats.holdings.forEach(h => {
  //   weights[h.symbol] = h.marketValue / stats.currentValue
  // })

  const navigate = useNavigate();

  return (
    <>
      <Card
        className={`${homeStyles.card}`}
        onClick={() => navigate("/backtest?id=" + data.savedStrategyID)}
      >
        <div style={{
          width: "50%",
          margin: "0px auto",
          display: "block",
          marginTop: "10px",
        }}
        >
          <p style={{ textAlign: "center", marginTop: "65px", marginBottom: "50px" }} className={appStyles.subtext}>no data yet</p>
        </div>
        <Card.Body>
          <Card.Text style={{ textAlign: "center" }}>{data.strategyName}</Card.Text>
          {/* <Card.Text>
            Some quick example text to build on the card title and make up the
            bulk of the card's content.
          </Card.Text> */}
        </Card.Body>
        <ListGroup className="list-group-flush">
          <ListGroup.Item>1Y Return: {data.oneYearReturn}%</ListGroup.Item>
          <ListGroup.Item>Rebalances: {data.rebalanceInterval}</ListGroup.Item>
          <ListGroup.Item>Sharpe Ratio: {data.sharpeRatio}</ListGroup.Item>
        </ListGroup>
      </Card>
    </>
  )
}


