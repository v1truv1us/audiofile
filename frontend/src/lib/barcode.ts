import { apiFetch } from './api';

type BarcodeDetectorClass = new (options?: { formats?: string[] }) => {
	detect(image: ImageBitmap): Promise<Array<{ rawValue: string }>>;
};

interface BarcodeDetectorWindow extends Window {
	BarcodeDetector?: BarcodeDetectorClass;
}

export interface ScanResult {
	barcode: string;
	results: { barcode: string; results: ReleaseSearchResult[] };
}

export interface ReleaseSearchResult {
	mbid: string;
	title: string;
	artist: string;
	year?: number | null;
	label?: string;
	coverUrl?: string;
}

export async function readBarcodeFromImage(file: File): Promise<string> {
	const Detector = (window as unknown as BarcodeDetectorWindow).BarcodeDetector;
	if (!Detector) throw new Error('Barcode scanning is not supported in this browser. Enter the barcode manually.');
	const detector = new Detector({ formats: ['ean_13', 'upc_a', 'upc_e'] });
	const image = await createImageBitmap(file);
	try {
		const codes = await detector.detect(image);
		return codes[0]?.rawValue ?? '';
	} finally {
		image.close();
	}
}

export async function scanBarcodeApi(barcode: string): Promise<{ barcode: string; results: ReleaseSearchResult[] }> {
	const res = await apiFetch('/api/releases/scan', {
		public: true,
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ barcode }),
	});
	if (!res.ok) throw new Error(await res.text());
	return res.json();
}
