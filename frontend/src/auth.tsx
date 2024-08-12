import React, { createContext, useContext, useEffect, useState } from 'react';
import { createClient, Session, SupabaseClient, User } from '@supabase/supabase-js';

const supabaseUrl = process.env.REACT_APP_SUPABASE_URL || "";
const supabaseKey = process.env.REACT_APP_SUPABASE_ANON_KEY || "";

const supabase: SupabaseClient = createClient(supabaseUrl, supabaseKey);

interface AuthProviderProps {
  children: React.ReactNode
}

type AuthContextType = {
  loading: boolean,
  session: Session | null,
  user: User | null
}

const AuthContext = createContext<AuthContextType>({
  loading: true,
  session: null,
  user: null
})

const AuthProvider = (props: AuthProviderProps) => {
  const [user, setUser] = useState<User | null>(null)
  const [session, setSession] = useState<Session | null>(null)
  const [loading, setLoading] = useState<boolean>(true)

  useEffect(() => {
    const { data: listener } = supabase.auth.onAuthStateChange((_event, session) => {
      setSession(session)
      setUser(session?.user || null)
      setLoading(false)
    })

    const setData = async () => {
      const { data: { session }, error } = await supabase.auth.getSession()
      if (error) {
        throw error
      }

      setSession(session)
      setUser(session?.user || null)
      setLoading(false)
    }

    setData()

    return () => {
      listener?.subscription.unsubscribe()
    }
  }, [])

  const value = {
    loading,
    session,
    user
  }

  return (
    <AuthContext.Provider value={value}>
      {props.children}
    </AuthContext.Provider>
  )
}

export const useAuth = () => {
  return useContext(AuthContext)
}

export default AuthProvider