import { describe, it, expect } from "vitest";
import { parseHash } from "../../src/router.js";

describe("parseHash", () => {
  it("returns login for empty hash", () => {
    expect(parseHash("")).toEqual({ page: "login" });
  });

  it("returns login for #/login", () => {
    expect(parseHash("#/login")).toEqual({ page: "login" });
  });

  it("returns login for #login (without leading slash)", () => {
    expect(parseHash("#login")).toEqual({ page: "login" });
  });

  it("returns tables for #/tables", () => {
    expect(parseHash("#/tables")).toEqual({ page: "tables" });
  });

  it("returns spreadsheet with resource for #/tables/posts", () => {
    expect(parseHash("#/tables/posts")).toEqual({
      page: "spreadsheet",
      resource: "posts",
    });
  });

  it("handles encoded resource names", () => {
    expect(parseHash("#/tables/my%20table")).toEqual({
      page: "spreadsheet",
      resource: "my table",
    });
  });

  it("returns login for unknown routes", () => {
    expect(parseHash("#/unknown")).toEqual({ page: "login" });
  });

  it("handles hash with just #", () => {
    expect(parseHash("#")).toEqual({ page: "login" });
  });

  it("handles #/ (just slash)", () => {
    expect(parseHash("#/")).toEqual({ page: "login" });
  });
});
