import { useState } from "react";
import modalsStyle from "./Modals.module.css";
import formStyles from "pages/Backtest/Form.module.css";
import { ContactRequest, GoogleAuthUser } from "../models";
import { endpoint } from "../App";
import { useAuth } from "auth";

export function HelpModal({ show, close }: {
  show: boolean;
  close: () => void;
}) {
  if (!show) return null;

  const handleOverlayClick = (e: any) => {
    if (e.target.id === "help-modal") {
      close();
    }
  };

  return (
    <div id="help-modal" className={modalsStyle.modal} onClick={handleOverlayClick}>
      <div className={`${modalsStyle.modal_content} ${modalsStyle.help_modal}`}>
        <span onClick={() => close()} className={modalsStyle.close} id="closeModalBtn">&times;</span>
        <h2 className={modalsStyle.modal_title}>Welcome!</h2>
        <div className={modalsStyle.help_text_container}>
          <ul>
            <li>
              <p className={modalsStyle.help_text}>FactorBacktest.net allows you to rapidly test factor-based investment strategies.</p>
            </li>
            <li>
              <p className={modalsStyle.help_text}>Define your strategy equation, set backtest parameters, and narrow your asset selection pool. Hit the <button className={modalsStyle.demo_backtest_btn}>Run Backtest</button> button on the left to test your first strategy!</p>
            </li>
            <li>
              <p className={modalsStyle.help_text}>The performance chart shows adjusted returns, accounting for splits, dividends, etc. Click on any datapoint to view what the strategy did on that day.</p>
              <img className={modalsStyle.help_gif} style={{ width: "100%" }} src="./dots.gif" />
            </li>
            <li>
              <p className={modalsStyle.help_text}>To view this message again, hit "User Guide" on the top right.</p>
            </li>
          </ul>
        </div>

        <button onClick={() => close()} className={modalsStyle.mobile_close_btn}>close</button>
      </div>
    </div>
  );
}

export function ContactModal({ show, close }: {
  show: boolean;
  close: () => void;
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
        <h2 style={{ marginBottom: "40px" }}>Contact</h2>
        <ContactForm  />
      </div>
    </div>
  );
}

function ContactForm() {
  const [replyEmail, setReplyEmail] = useState<string | null>(null);
  const [content, setMessageContent] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [submitted, setSubmitted] = useState(false);

  const { session } = useAuth()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    try {
      const response = await fetch(endpoint + "/contact", {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          "Authorization": session ? "Bearer " + session.access_token : ""
        },
        body: JSON.stringify({
          // userID,
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
        <label className={formStyles.label} htmlFor="replyEmail">Reply Email (optional)</label>
        <input
          type="text"
          id="replyEmail"
          name="replyEmail"
          value={replyEmail ? replyEmail : ""}
          className={modalsStyle.contact_form_email_input}
          onChange={(e) => {
            setSubmitted(false);
            setError(null);
            setReplyEmail(e.target.value)
          }}
        />
      </div>
      <div>
        <label className={formStyles.label} htmlFor="content">Message</label>
        <textarea
          className={modalsStyle.contact_form_message_input}
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
      <button className={modalsStyle.contact_btn} type="submit">Submit</button>
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
