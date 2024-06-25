import { useState } from "react";
import "./modals.css";
import { ContactRequest } from "./models";
import { endpoint } from "./App";

export function Modal({ userID, show, close }: {
  userID: string;
  show: boolean;
  close: () => void;
}) {
  console.log(show)
  if (!show) return null;

  const handleOverlayClick = (e: any) => {
    if (e.target.id === "modal") {
      close();
    }
  };

  return (
    <div id="modal" className="modal" onClick={handleOverlayClick}>
      <div className="modal-content">
        <span onClick={() => close()} className="close" id="closeModalBtn">&times;</span>
        <h2 style={{marginBottom: "40px"}}>Contact</h2>
        <ContactForm userID={userID} />
      </div>
    </div>
  );
}

function ContactForm({userID}: {
  userID: string;
}) {
  const [replyEmail, setReplyEmail] = useState<string | null>(null);
  const [content, setMessageContent] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [submitted, setSubmitted] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    try {
      const response = await fetch(endpoint+"/contact", {
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
          value={replyEmail ? replyEmail: ""}
          style={{width: "300px"}}
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
        style={{width: "400px", height: "100px"}}
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
    <div style={{margin: "0px auto", marginTop: "30px"}} className='error-container'>
      <h4 style={{ marginBottom: "0px", marginTop: "0px" }}>That's an error.</h4>
      <p>{message}</p>
    </div>
  </>
}

export default ContactForm;
