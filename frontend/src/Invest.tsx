import { GoogleAuthUser, GetSavedStrategiesResponse } from "./models";
import investStyles from "./Invest.module.css";
import appStyles from "./App.module.css";
import { Nav } from "./Nav";
import { useEffect, useState } from "react";
import { ContactModal, HelpModal } from "./Modals";
import { Table } from "react-bootstrap";
import { endpoint } from "./App";
import { formatDate } from "./util";
import { useNavigate, useSearchParams } from "react-router-dom";

export default function Invest({
  user,
  setUser,
}: {
  user: GoogleAuthUser | null,
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>;
}) {
  const [showHelpModal, setShowHelpModal] = useState(false);
  const [showContactModal, setShowContactModal] = useState(false);

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
        const j = await response.json()
        setSavedStrategies(j)
      }
    } catch (error) {
      alert((error as Error).message)
      console.error("Error:", error);
    }
  }

  useEffect(() => {
    if (user) {
      getStrategies()
    }
  }, [user]);

  const [searchParams, setSearchParams] = useSearchParams();
  const [selectedRow, setSelectedRow] = useState(-1);

  return <>
    <Nav loggedIn={user !== null} setUser={setUser} showLinks={false} setShowHelpModal={setShowHelpModal} setShowContactModal={setShowContactModal} />

    <div className={`${appStyles.tile} ${investStyles.container}`}>
      <h2 style={{ marginBottom: "0px" }}>Invest in Strategy</h2>
      <p className={appStyles.subtext}>Deposit funds into any strategy you've previously tested.</p>

      <Table hover>
        <thead>
          <tr>
            <th>Name</th>
            <th>Rebalance Interval</th>
            <th>Bookmarked</th>
            <th>Creation Date</th>
          </tr>
        </thead>
        <tbody>
          {savedStrategies.map((s, i) => {
            return <tr key={i} onClick={() => {
              setSearchParams([["id", s.savedStrategyID]])
              setSelectedRow(i)
            }}
              className={i === selectedRow ? "table-active" : ""}
              style={{ cursor: "pointer" }}>
              <td>{s.strategyName}</td>
              <td>{s.rebalanceInterval}</td>
              <td>{s.bookmarked ? "true" : "false"}</td>
              <td>{s.createdAt.substring(0, s.createdAt.indexOf("T"))}</td>
            </tr>
          })}
        </tbody>
      </Table>

      <ConfirmInvestment />

    </div>


    <ContactModal user={user} userID={""} show={showContactModal} close={() => setShowContactModal(false)} />
    <HelpModal show={showHelpModal} close={() => setShowHelpModal(false)} />
  </>
}


function ConfirmInvestment() {
  return <>
  <form onSubmit={(e) => {e.preventDefault()}}>
    <label>Amount</label>
    <input />
    <button type="submit">Confirm</button>
    </form>
  </>
}