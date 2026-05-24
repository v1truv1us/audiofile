import { supabase } from './supabase'

export function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message) return error.message
  return fallback
}

export async function registerCurrentUserPasskey(friendlyName: string) {
  const { data, error } = await supabase.auth.registerPasskey()
  if (error) return { error }

  const name = friendlyName.trim()
  if (name && data?.id) {
    const { error: updateError } = await supabase.auth.passkey.update({
      passkeyId: data.id,
      friendlyName: name,
    })
    if (updateError) return { error: updateError }
  }

  return { error: null }
}

export async function signInWithPasskey() {
  const { error } = await supabase.auth.signInWithPasskey()
  return { error }
}
