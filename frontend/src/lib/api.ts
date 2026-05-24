// Shared API configuration — reads from window at runtime so builds work anywhere.
const API_BASE: string =
	typeof window !== "undefined"
		? (window as any).__API_BASE__ || "http://localhost:8080"
		: "http://localhost:8080";

export function apiUrl(path: string): string {
	return `${API_BASE}${path}`;
}

/**
 * Authenticated fetch wrapper. Attaches the current Supabase access token
 * as a Bearer Authorization header. Handles token refresh automatically.
 */
export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
	const { supabase } = await import('./supabase');
	const { data: { session } } = await supabase.auth.getSession();

	const headers = new Headers(init.headers || {});
	if (session?.access_token) {
		headers.set('Authorization', `Bearer ${session.access_token}`);
	}
	if (!headers.has('Content-Type') && init.body && typeof init.body === 'string') {
		headers.set('Content-Type', 'application/json');
	}

	const res = await fetch(apiUrl(path), { ...init, headers });

	if (res.status === 401) {
		// Session expired — redirect to login
		window.location.href = '/login';
		throw new Error('Session expired');
	}

	return res;
}
