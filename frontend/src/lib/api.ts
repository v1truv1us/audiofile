// Shared API configuration — reads from window at runtime so builds work anywhere.
const API_BASE: string =
	typeof window !== "undefined"
		? (window as any).__API_BASE__ || "http://localhost:8080"
		: "http://localhost:8080";

export function apiUrl(path: string): string {
	return `${API_BASE}${path}`;
}
