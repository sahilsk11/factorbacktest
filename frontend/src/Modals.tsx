import { useState } from "react";
import "./modals.css";
import { ContactRequest } from "./models";
import { endpoint } from "./App";

export function HelpModal({ show, close }: {
  show: boolean;
  close: () => void;
}) {
  console.log(show)
  if (!show) return null;

  const handleOverlayClick = (e: any) => {
    if (e.target.id === "help-modal") {
      close();
    }
  };

  return (
    <div id="help-modal" className="modal" onClick={handleOverlayClick}>
      <div className="modal-content help-modal">
        <span onClick={() => close()} className="close" id="closeModalBtn">&times;</span>
        <h2 style={{ marginBottom: "40px" }}>Welcome!</h2>
        <div className="help-text-container">
          <ul>
            <li>
              <p className="help-text">FactorBacktest.net allows you to rapidly test factor-based investment strategies.</p>
            </li>
            <li>
              <p className="help-text">Define your strategy equation, set backtest parameters, and narrow your asset selection pool. Hit the <button className="demo-backtest-btn">Run Backtest</button> button on the left to test your first strategy!</p>
            </li>
            <li>
              <p className="help-text">The performance chart shows adjusted returns, accounting for splits, dividends, etc. Click on any datapoint to view what the strategy did on that day.</p>
              <img className="help-gif" style={{ width: "100%" }} src="./dots.gif" />
            </li>
            <li>
              <p className="help-text">To view this message again, hit "User Guide" on the top left.</p>
            </li>
          </ul>

        </div>
      </div>
    </div>
  );
}

export function ContactModal({ userID, show, close }: {
  userID: string;
  show: boolean;
  close: () => void;
}) {
  console.log(show)
  if (!show) return null;

  const handleOverlayClick = (e: any) => {
    if (e.target.id === "contact-modal") {
      close();
    }
  };

  return (
    <div id="contact-modal" className="modal" onClick={handleOverlayClick}>
      <div className="modal-content">
        <span onClick={() => close()} className="close" id="closeModalBtn">&times;</span>
        <h2 style={{ marginBottom: "40px" }}>Contact</h2>
        <ContactForm userID={userID} />
      </div>
    </div>
  );
}

function ContactForm({ userID }: {
  userID: string;
}) {
  const [replyEmail, setReplyEmail] = useState<string | null>(null);
  const [content, setMessageContent] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [submitted, setSubmitted] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    try {
      const response = await fetch(endpoint + "/contact", {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          userID,
          replyEmail,
          content
        } as ContactRequest),
      });

      if (response.ok) {
        setSubmitted(true);
      } else {
        setError((await response.json()).error)
      }
    } catch (error) {
      setError((error as Error).message)
      console.error('Network error:', error);
    }
  };

  return (
    <form onSubmit={handleSubmit}>
      <div>
        <label htmlFor="replyEmail">Reply Email (optional)</label>
        <input
          type="text"
          id="replyEmail"
          name="replyEmail"
          value={replyEmail ? replyEmail : ""}
          style={{ width: "300px" }}
          onChange={(e) => {
            setSubmitted(false);
            setError(null);
            setReplyEmail(e.target.value)
          }}
        />
      </div>
      <div>
        <label htmlFor="content">Message</label>
        <textarea
          style={{ width: "400px", height: "100px" }}
          id="content"
          name="content"
          value={content}
          onChange={(e) => {
            setSubmitted(false);
            setError(null)
            setMessageContent(e.target.value)
          }}
        />
      </div>
      <button className="contact-btn" type="submit">Submit</button>
      {submitted ? <p>Thanks! If needed, I'll be in touch shortly.</p> : null}
      <Error message={error} />
    </form>
  );
};

function Error({ message }: { message: string | null }) {
  return message === null ? null : <>
    <div style={{ margin: "0px auto", marginTop: "30px" }} className='error-container'>
      <h4 style={{ marginBottom: "0px", marginTop: "0px" }}>That's an error.</h4>
      <p>{message}</p>
    </div>
  </>
}

export default ContactForm;
