import { GoogleAuthUser, GetSavedStrategiesResponse, InvestInStrategyRequest, GetInvestmentsResponse } from "./models";
import investStyles from "./Invest.module.css";
import appStyles from "./l3_service.module.css";
import { Nav } from "./Nav";
import { Dispatch, SetStateAction, useEffect, useState } from "react";
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
  const [checkForNewInvestments, setCheckForNewInvestments] = useState(true);


  return <>
    <Nav loggedIn={user !== null} setUser={setUser} showLinks={false} setShowHelpModal={setShowHelpModal} setShowContactModal={setShowContactModal} />

    <InvestInStrategy user={user} setCheckForNewInvestments={setCheckForNewInvestments} />
    <ActiveInvestments user={user} checkForNewInvestments={checkForNewInvestments} setCheckForNewInvestments={setCheckForNewInvestments} />


    <ContactModal user={user} userID={""} show={showContactModal} close={() => setShowContactModal(false)} />
    <HelpModal show={showHelpModal} close={() => setShowHelpModal(false)} />
  </>
}

function InvestInStrategy({
  user,
  setCheckForNewInvestments,
}: {
  user: GoogleAuthUser | null,
  setCheckForNewInvestments: Dispatch<SetStateAction<boolean>>;
}) {
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

  const [selectedStrategyID, setSelectedStrategyID] = useState<string | null>(null);


  return <div className={`${appStyles.tile} ${investStyles.container}`}>
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
            setSelectedStrategyID(s.savedStrategyID);
          }}
            className={s.savedStrategyID === selectedStrategyID ? "table-active" : ""}
            style={{ cursor: "pointer" }}>
            <td>{s.strategyName}</td>
            <td>{s.rebalanceInterval}</td>
            <td>{s.bookmarked ? "true" : "false"}</td>
            <td>{s.createdAt.substring(0, s.createdAt.indexOf("T"))}</td>
          </tr>;
        })}
      </tbody>
    </Table>

    {selectedStrategyID ? <ConfirmInvestment savedStrategyID={selectedStrategyID} user={user} setCheckForNewInvestments={setCheckForNewInvestments} /> : null}

  </div>;
}

function ConfirmInvestment({
  user,
  savedStrategyID,
  setCheckForNewInvestments,
}: {
  user: GoogleAuthUser | null;
  savedStrategyID: string;
  setCheckForNewInvestments: Dispatch<SetStateAction<boolean>>;
}) {
  const [amount, setAmount] = useState("")
  async function invest() {
    try {
      const response = await fetch(endpoint + "/investInStrategy", {
        method: "POST",
        headers: {
          "Authorization": user ? "Bearer " + user.accessToken : ""
        },
        body: JSON.stringify({
          amountDollars: parseInt(amount),
          savedStrategyID,
        } as InvestInStrategyRequest)
      });
      if (!response.ok) {
        const j = await response.json()
        alert(j.error)
        console.error("Error submitting data:", response.status);
      } else {
        setCheckForNewInvestments(true)
      }
    } catch (error) {
      alert((error as Error).message)
      console.error("Error:", error);
    }
  }
  return <>
    <form onSubmit={(e) => { e.preventDefault(); invest() }}>
      <label>Amount: $</label>
      <input value={amount} onChange={e => setAmount(e.target.value)} />
      <button type="submit">Confirm</button>
    </form>
  </>
}

function ActiveInvestments({
  user,
  checkForNewInvestments,
  setCheckForNewInvestments,
}: {
  user: GoogleAuthUser | null,
  checkForNewInvestments: boolean,
  setCheckForNewInvestments: Dispatch<SetStateAction<boolean>>;
}) {
  const [activeInvestments, setActiveInvestments] = useState<GetInvestmentsResponse[]>([]);

  async function getInvestments() {
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
        setCheckForNewInvestments(false)
      }
    } catch (error) {
      alert((error as Error).message)
      console.error("Error:", error);
    }
  }

  useEffect(() => {
    if (user && checkForNewInvestments) {
      getInvestments()
    }
  }, [user, checkForNewInvestments]);

  if (activeInvestments.length === 0) {
    return null;
  }

  return <>
    <div className={`${appStyles.tile} ${investStyles.container}`}>
      <h2 style={{ marginBottom: "0px" }}>Active Investments</h2>
      <p className={appStyles.subtext}>Deposit funds into any strategy you've previously tested.</p>
      <Table hover>
      <thead>
        <tr>
          <th>ID</th>
          <th>Amount</th>
          <th>Start Date</th>
        </tr>
      </thead>
      <tbody>
        {activeInvestments.map((s, i) => {
          return <tr key={i} style={{ cursor: "pointer" }}>
            <td>{s.savedStrategyID}</td>
            <td>{s.amountDollars}</td>
            <td>{s.startDate}</td>
          </tr>;
        })}
      </tbody>
    </Table>
    </div>
  </>
}