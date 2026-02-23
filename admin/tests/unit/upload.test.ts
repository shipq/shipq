import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { uploadFile, getDownloadUrl } from "../../src/shipq-files.js";

// --- Fetch mock (for JSON API calls: upload-url, complete) ---

function mockFetch(
  responses: Array<{
    ok: boolean;
    status?: number;
    json?: unknown;
  }>
) {
  let callIndex = 0;
  return vi.fn().mockImplementation(() => {
    const resp = responses[callIndex] ?? responses[responses.length - 1];
    callIndex++;
    return Promise.resolve({
      ok: resp.ok,
      status: resp.status ?? (resp.ok ? 200 : 500),
      statusText: resp.ok ? "OK" : "Error",
      json: () => Promise.resolve(resp.json ?? {}),
      text: () => Promise.resolve(JSON.stringify(resp.json ?? {})),
      headers: new Headers(),
    });
  });
}

// --- XMLHttpRequest mock (for PUT uploads to S3) ---

interface MockXHRConfig {
  status?: number;
  etag?: string;
  shouldError?: boolean;
}

let xhrInstances: MockXHR[] = [];

class MockXHR {
  method = "";
  url = "";
  requestHeaders: Record<string, string> = {};
  responseHeaders: Record<string, string> = {};
  status = 200;
  sentBody: unknown = null;

  upload = { onprogress: null as ((e: unknown) => void) | null };
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  ontimeout: (() => void) | null = null;
  onabort: (() => void) | null = null;

  private _config: MockXHRConfig;

  constructor() {
    this._config = xhrConfigs.shift() ?? {};
    xhrInstances.push(this);
  }

  open(method: string, url: string) {
    this.method = method;
    this.url = url;
  }

  setRequestHeader(key: string, value: string) {
    this.requestHeaders[key] = value;
  }

  getResponseHeader(key: string): string | null {
    return this.responseHeaders[key] ?? null;
  }

  abort() {
    // Triggers onabort if set
    setTimeout(() => this.onabort?.(), 0);
  }

  send(body: unknown) {
    this.sentBody = body;
    const config = this._config;

    setTimeout(() => {
      if (config.shouldError) {
        this.onerror?.();
        return;
      }

      this.status = config.status ?? 200;
      this.responseHeaders["ETag"] = config.etag ?? "";

      // Fire a progress event if handler is set
      if (this.upload.onprogress && body instanceof Blob) {
        this.upload.onprogress({
          lengthComputable: true,
          loaded: body.size,
          total: body.size,
        });
      }

      this.onload?.();
    }, 0);
  }
}

let xhrConfigs: MockXHRConfig[] = [];

function setXHRConfigs(configs: MockXHRConfig[]) {
  xhrConfigs = [...configs];
}

function createTestFile(
  name: string,
  content: string,
  type = "text/plain"
): File {
  return new File([content], name, { type });
}

describe("shipq-files", () => {
  const origXHR = globalThis.XMLHttpRequest;

  beforeEach(() => {
    vi.restoreAllMocks();
    xhrInstances = [];
    xhrConfigs = [];
    // @ts-ignore - mock constructor
    globalThis.XMLHttpRequest = MockXHR;
  });

  afterEach(() => {
    globalThis.XMLHttpRequest = origXHR;
  });

  describe("uploadFile", () => {
    it("handles single-part PUT upload", async () => {
      const fetchMock = mockFetch([
        // 1. upload-url response
        {
          ok: true,
          json: {
            file_id: "file-123",
            method: "PUT",
            upload_url: "https://s3.example.com/presigned-put",
          },
        },
        // 2. complete response (fetch call #2)
        {
          ok: true,
          json: {
            file_id: "file-123",
            name: "test.txt",
            content_type: "text/plain",
            size_bytes: 11,
          },
        },
      ]);
      globalThis.fetch = fetchMock;

      // XHR config for the S3 PUT
      setXHRConfigs([{ status: 200 }]);

      const file = createTestFile("test.txt", "hello world");
      const result = await uploadFile(file);

      expect(result.fileId).toBe("file-123");
      expect(result.name).toBe("test.txt");
      expect(result.contentType).toBe("text/plain");
      expect(result.sizeBytes).toBe(11);

      // fetch: upload-url + complete = 2 calls
      expect(fetchMock).toHaveBeenCalledTimes(2);
      expect(fetchMock.mock.calls[0][0]).toContain("/files/upload-url");
      expect(fetchMock.mock.calls[1][0]).toContain("/files/complete");

      // XHR: 1 PUT to S3
      expect(xhrInstances).toHaveLength(1);
      expect(xhrInstances[0].method).toBe("PUT");
      expect(xhrInstances[0].url).toBe("https://s3.example.com/presigned-put");
    });

    it("handles multipart upload", async () => {
      const fetchMock = mockFetch([
        // 1. upload-url response (multipart)
        {
          ok: true,
          json: {
            file_id: "file-456",
            method: "MULTIPART",
            upload_id: "mpu-abc",
            parts: [
              { part_number: 1, upload_url: "https://s3.example.com/part1" },
              { part_number: 2, upload_url: "https://s3.example.com/part2" },
            ],
          },
        },
        // 2. complete
        {
          ok: true,
          json: {
            file_id: "file-456",
            name: "big.bin",
            content_type: "application/octet-stream",
            size_bytes: 1024,
          },
        },
      ]);
      globalThis.fetch = fetchMock;

      // XHR configs for 2 part uploads
      setXHRConfigs([
        { status: 200, etag: '"etag1"' },
        { status: 200, etag: '"etag2"' },
      ]);

      const file = createTestFile(
        "big.bin",
        "x".repeat(1024),
        "application/octet-stream"
      );
      const result = await uploadFile(file);

      expect(result.fileId).toBe("file-456");

      // fetch: upload-url + complete = 2
      expect(fetchMock).toHaveBeenCalledTimes(2);

      // XHR: 2 PUTs
      expect(xhrInstances).toHaveLength(2);
      expect(xhrInstances[0].url).toBe("https://s3.example.com/part1");
      expect(xhrInstances[1].url).toBe("https://s3.example.com/part2");

      // Complete call should include upload_id and parts with ETags
      const completeCall = fetchMock.mock.calls[1];
      const completeBody = JSON.parse(completeCall[1].body);
      expect(completeBody.upload_id).toBe("mpu-abc");
      expect(completeBody.parts).toEqual([
        { part_number: 1, etag: '"etag1"' },
        { part_number: 2, etag: '"etag2"' },
      ]);
    });

    it("throws on upload-url failure", async () => {
      globalThis.fetch = mockFetch([
        { ok: false, status: 500, json: { error: "internal error" } },
      ]);

      const file = createTestFile("test.txt", "hello");
      await expect(uploadFile(file)).rejects.toThrow(
        "Failed to get upload URL"
      );
    });

    it("throws on S3 PUT failure", async () => {
      globalThis.fetch = mockFetch([
        {
          ok: true,
          json: {
            file_id: "file-789",
            method: "PUT",
            upload_url: "https://s3.example.com/presigned",
          },
        },
      ]);

      setXHRConfigs([{ status: 403 }]);

      const file = createTestFile("test.txt", "hello");
      await expect(uploadFile(file)).rejects.toThrow("Upload failed: 403");
    });

    it("throws on XHR network error", async () => {
      globalThis.fetch = mockFetch([
        {
          ok: true,
          json: {
            file_id: "file-err",
            method: "PUT",
            upload_url: "https://s3.example.com/presigned",
          },
        },
      ]);

      setXHRConfigs([{ shouldError: true }]);

      const file = createTestFile("test.txt", "hello");
      await expect(uploadFile(file)).rejects.toThrow(
        "Upload failed: network error"
      );
    });

    it("calls onProgress during upload", async () => {
      const fetchMock = mockFetch([
        {
          ok: true,
          json: {
            file_id: "file-prog",
            method: "PUT",
            upload_url: "https://s3.example.com/presigned",
          },
        },
        {
          ok: true,
          json: {
            file_id: "file-prog",
            name: "test.txt",
            content_type: "text/plain",
            size_bytes: 5,
          },
        },
      ]);
      globalThis.fetch = fetchMock;

      setXHRConfigs([{ status: 200 }]);

      const progressCalls: UploadProgress[] = [];
      const file = createTestFile("test.txt", "hello");

      type UploadProgress = { loaded: number; total: number; percent: number };
      await uploadFile(file, {
        onProgress: (p) => progressCalls.push(p),
      });

      // XHR mock fires one progress event with loaded=total
      expect(progressCalls.length).toBeGreaterThanOrEqual(1);
      expect(progressCalls[progressCalls.length - 1].percent).toBe(100);
    });
  });

  describe("getDownloadUrl", () => {
    it("returns the correct download URL", () => {
      expect(getDownloadUrl("file-123")).toBe("/files/file-123/download");
    });

    it("encodes special characters in file ID", () => {
      expect(getDownloadUrl("file/with spaces")).toBe(
        "/files/file%2Fwith%20spaces/download"
      );
    });
  });
});
