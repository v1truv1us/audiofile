/** Strip trailing slashes so `/wishlist/` matches `/wishlist`. */
export function normalizePath(path: string): string {
	if (path.length > 1 && path.endsWith('/')) {
		return path.slice(0, -1);
	}
	return path;
}

/** True when viewing someone else's shared collection or wishlist. */
export function isPublicShareView(
	location: Pick<Location, 'pathname' | 'search'> = window.location,
): boolean {
	const path = normalizePath(location.pathname);
	if (path !== '/collection' && path !== '/wishlist') {
		return false;
	}
	return new URLSearchParams(location.search).has('share');
}

const PUBLIC_PAGES = new Set(['/login', '/signup', '/terms', '/privacy', '/refund', '/pricing']);

/** Pages that do not require a signed-in session. */
export function isPublicPage(
	location: Pick<Location, 'pathname' | 'search'> = window.location,
): boolean {
	const path = normalizePath(location.pathname);
	return PUBLIC_PAGES.has(path) || isPublicShareView(location);
}
