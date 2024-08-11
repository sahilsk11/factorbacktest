import React, { useContext, useState, useEffect } from "react"
import { getAuth, signInWithPhoneNumber, User, RecaptchaVerifier } from "@firebase/auth";
import { initializeApp } from '@firebase/app';

const AuthContext = React.createContext(null)

const auth = getAuth(initializeApp({
  apiKey: process.env.REACT_APP_FIREBASE_API_KEY,
  authDomain: process.env.REACT_APP_FIREBASE_AUTH_DOMAIN,
  databaseURL: process.env.REACT_APP_FIREBASE_DATABASE_URL,
  projectId: process.env.REACT_APP_FIREBASE_PROJECT_ID,
  storageBucket: process.env.REACT_APP_FIREBASE_STORAGE_BUCKET,
  messagingSenderId: process.env.REACT_APP_FIREBASE_MESSAGING_SENDER_ID,
  appId: process.env.REACT_APP_FIREBASE_APP_ID
}))



export function useAuth() {
  return useContext(AuthContext)
}

export function AuthProvider({ children }: {
  children: any
}) {
  const [currentUser, setCurrentUser] = useState<User | null>()
  const [loading, setLoading] = useState(true)

  function signup() {
    const applicationVerifier = new RecaptchaVerifier(
      auth, ''
    )

    return signInWithPhoneNumber(auth, "+14088870718", RecaptchaVerifier)
  }

  function login(email, password) {
    return auth.signInWithEmailAndPassword(email, password)
  }

  function logout() {
    return auth.signOut()
  }

  function resetPassword(email) {
    return auth.send (email)
  }

  function updateEmail(email) {
    return currentUser.updateEmail(email)
  }

  function updatePassword(password) {
    return currentUser.updatePassword(password)
  }

  useEffect(() => {
    const unsubscribe = auth.onAuthStateChanged(user => {
      setCurrentUser(user)
      setLoading(false)
    })

    return unsubscribe
  }, [])

  const value = {
    currentUser,
    login,
    signup,
    logout,
    resetPassword,
    updateEmail,
    updatePassword
  }

  return (
    <AuthContext.Provider value={value}>
      {!loading && children}
    </AuthContext.Provider>
  )
}