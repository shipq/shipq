// shipq-files.ts - File upload/download utility.
//
// This is the single source of truth for the shipq file upload client.
// It is used directly by the admin panel and embedded by the Go codegen
// into user projects (via go:embed → GenerateTypeScriptHelpers).
//
// Do NOT duplicate this logic elsewhere.

export interface ShipqFilesConfig {
  /** Base URL of the API server (e.g., "https://api.example.com"). Defaults to "" (same origin). */
  baseUrl?: string;
  /** Credentials mode for fetch. Defaults to "include" for cross-origin cookie auth. */
  credentials?: RequestCredentials;
}

let _baseUrl = "";
let _credentials: RequestCredentials = "include";

/** Call once at app startup to configure the API base URL and credentials. */
export function configure(config: ShipqFilesConfig): void {
  if (config.baseUrl !== undefined) {
    // Remove trailing slash
    _baseUrl = config.baseUrl.replace(/\/+$/, "");
  }
  if (config.credentials !== undefined) {
    _credentials = config.credentials;
  }
}

export interface UploadResult {
  fileId: string;
  name: string;
  contentType: string;
  sizeBytes: number;
}

export interface UploadProgress {
  loaded: number;
  total: number;
  percent: number;
}

interface UploadURLResponse {
  file_id: string;
  method: "PUT" | "MULTIPART";
  upload_url?: string;
  upload_id?: string;
  parts?: Array<{ part_number: number; upload_url: string }>;
}

interface CompleteResponse {
  file_id: string;
  name: string;
  content_type: string;
  size_bytes: number;
}

/** Result from an XHR PUT upload. */
interface XHRUploadResult {
  status: number;
  etag: string;
}

/**
 * Upload a file to the server using presigned URLs.
 * Automatically handles single-part and multipart uploads based on file size.
 */
export async function uploadFile(
  file: File,
  options?: {
    onProgress?: (progress: UploadProgress) => void;
    signal?: AbortSignal;
    visibility?: "private" | "organization" | "public" | "anonymous";
  }
): Promise<UploadResult> {
  const { onProgress, signal, visibility } = options ?? {};

  // Step 1: Request presigned upload URL(s)
  const uploadUrlResp = await fetch(`${_baseUrl}/files/upload-url`, {
    method: "POST",
    credentials: _credentials,
    headers: { "Content-Type": "application/json" },
    signal,
    body: JSON.stringify({
      name: file.name,
      content_type: file.type || "application/octet-stream",
      size_bytes: file.size,
      visibility: visibility ?? "private",
    }),
  });

  if (!uploadUrlResp.ok) {
    const errorBody = await uploadUrlResp.text();
    throw new Error(`Failed to get upload URL: ${uploadUrlResp.status} ${errorBody}`);
  }

  const uploadData: UploadURLResponse = await uploadUrlResp.json();

  // Step 2: Upload to storage
  if (uploadData.method === "PUT" && uploadData.upload_url) {
    // Single-part upload
    await xhrPut(
      uploadData.upload_url,
      file,
      file.type || "application/octet-stream",
      0,
      file.size,
      onProgress,
      signal
    );

    // Step 3: Complete the upload
    return await completeUpload(uploadData.file_id, undefined, signal);
  }

  if (uploadData.method === "MULTIPART" && uploadData.parts && uploadData.upload_id) {
    // Multipart upload
    const completedParts: Array<{ part_number: number; etag: string }> = [];
    let totalUploaded = 0;

    for (const part of uploadData.parts) {
      const partIndex = part.part_number - 1;
      const partSize = getPartSize(file.size, uploadData.parts.length);
      const start = partIndex * partSize;
      const end = Math.min(start + partSize, file.size);
      const blob = file.slice(start, end);

      const result = await xhrPut(
        part.upload_url,
        blob,
        file.type || "application/octet-stream",
        totalUploaded,
        file.size,
        onProgress,
        signal
      );

      completedParts.push({ part_number: part.part_number, etag: result.etag });
      totalUploaded += blob.size;
    }

    // Step 3: Complete multipart upload
    return await completeUpload(uploadData.file_id, completedParts, signal);
  }

  throw new Error("Unexpected upload response");
}

/** Get the download URL for a file. Opens a 302 redirect to S3. */
export function getDownloadUrl(fileId: string): string {
  return `${_baseUrl}/files/${encodeURIComponent(fileId)}/download`;
}

// --- Internal helpers ---

function getPartSize(totalSize: number, numParts: number): number {
  return Math.ceil(totalSize / numParts);
}

/**
 * PUT a blob to a URL using XMLHttpRequest for cross-browser progress tracking.
 * XHR's upload.onprogress works in all browsers (including Safari), unlike
 * fetch's ReadableStream body + duplex: "half" which is Chromium-only.
 */
function xhrPut(
  url: string,
  data: Blob,
  contentType: string,
  offsetBytes: number,
  totalBytes: number,
  onProgress?: (progress: UploadProgress) => void,
  signal?: AbortSignal
): Promise<XHRUploadResult> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", url);
    xhr.setRequestHeader("Content-Type", contentType);

    if (onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          const loaded = offsetBytes + e.loaded;
          onProgress({
            loaded,
            total: totalBytes,
            percent: Math.round((loaded / totalBytes) * 100),
          });
        }
      };
    }

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve({
          status: xhr.status,
          etag: xhr.getResponseHeader("ETag") ?? "",
        });
      } else {
        reject(new Error(`Upload failed: ${xhr.status}`));
      }
    };

    xhr.onerror = () => reject(new Error("Upload failed: network error"));
    xhr.ontimeout = () => reject(new Error("Upload failed: timeout"));

    // Wire up AbortSignal
    if (signal) {
      if (signal.aborted) {
        xhr.abort();
        reject(new DOMException("The operation was aborted.", "AbortError"));
        return;
      }
      signal.addEventListener("abort", () => xhr.abort(), { once: true });
      xhr.onabort = () =>
        reject(new DOMException("The operation was aborted.", "AbortError"));
    }

    xhr.send(data);
  });
}

async function completeUpload(
  fileId: string,
  parts?: Array<{ part_number: number; etag: string }>,
  signal?: AbortSignal
): Promise<UploadResult> {
  const body: Record<string, unknown> = {};
  if (parts) body.parts = parts;

  const resp = await fetch(`${_baseUrl}/files/${encodeURIComponent(fileId)}/complete`, {
    method: "POST",
    credentials: _credentials,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    signal,
  });

  if (!resp.ok) {
    const errorBody = await resp.text();
    throw new Error(`Failed to complete upload: ${resp.status} ${errorBody}`);
  }

  const data: CompleteResponse = await resp.json();
  return {
    fileId: data.file_id,
    name: data.name,
    contentType: data.content_type,
    sizeBytes: data.size_bytes,
  };
}
