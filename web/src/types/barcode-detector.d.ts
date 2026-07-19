// The Barcode Detection API isn't yet in TypeScript's built-in DOM lib.
// Supported in Chrome/Edge/Android WebView; not in Safari or Firefox.
interface BarcodeDetectorOptions {
  formats?: string[]
}

interface DetectedBarcode {
  rawValue: string
  format: string
}

declare class BarcodeDetector {
  static getSupportedFormats(): Promise<string[]>
  constructor(options?: BarcodeDetectorOptions)
  detect(source: CanvasImageSource): Promise<DetectedBarcode[]>
}
