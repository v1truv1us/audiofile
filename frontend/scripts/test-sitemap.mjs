import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const xml = await readFile(new URL('../dist/sitemap-0.xml', import.meta.url), 'utf8');
assert(!xml.includes('https://audiofile.app/login/'), 'The noindex login page must not be in the sitemap');
assert(xml.includes('https://audiofile.app/pricing/'), 'Public marketing pages must remain in the sitemap');

const loginHtml = await readFile(new URL('../dist/login/index.html', import.meta.url), 'utf8');
assert(loginHtml.includes('<meta name="robots" content="noindex, nofollow">'), 'Login must be noindex');
console.log('Verified sitemap and login indexing rules.');
