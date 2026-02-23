/**
 * Playwright E2E smoke tests for the admin panel.
 *
 * These run against the mock server (auto-started via playwright.config.ts).
 * No Go server needed.
 *
 * Run with: npm run admin:e2e
 */

import { test, expect } from "@playwright/test";

test.describe("Admin Panel", () => {
  test("shows login page on initial load", async ({ page }) => {
    await page.goto("/admin");
    await expect(page.locator(".login-box h1")).toHaveText("Admin Login");
    await expect(page.locator("#admin-email")).toBeVisible();
    await expect(page.locator("#admin-password")).toBeVisible();
    await expect(page.locator("#admin-login-btn")).toBeVisible();
  });

  test("shows error for invalid credentials", async ({ page }) => {
    await page.goto("/admin#/login");
    await page.fill("#admin-email", "bad@test.com");
    await page.fill("#admin-password", "wrong");
    await page.click("#admin-login-btn");

    await expect(page.locator(".login-box .error")).toBeVisible();
  });

  test("rejects non-GLOBAL_OWNER users", async ({ page }) => {
    await page.goto("/admin#/login");
    await page.fill("#admin-email", "user@test.com");
    await page.fill("#admin-password", "password");
    await page.click("#admin-login-btn");

    // Should show "Access denied" and stay on login page
    await expect(page.locator(".login-box .error")).toContainText(
      "GLOBAL_OWNER"
    );
  });

  test("shows table list after successful login", async ({ page }) => {
    await page.goto("/admin#/login");
    await page.fill("#admin-email", "test@test.com");
    await page.fill("#admin-password", "password");
    await page.click("#admin-login-btn");

    // Wait for navigation to tables page
    await expect(page.locator(".admin-sidebar")).toBeVisible({ timeout: 5000 });
    await expect(page.locator(".admin-main")).toBeVisible();

    // Should show table links in sidebar
    await expect(page.locator(".admin-sidebar a")).toHaveCount(5); // All Tables + 4 resources
  });

  test("navigates to spreadsheet view when clicking a table", async ({
    page,
  }) => {
    // Login first
    await page.goto("/admin#/login");
    await page.fill("#admin-email", "test@test.com");
    await page.fill("#admin-password", "password");
    await page.click("#admin-login-btn");
    await expect(page.locator(".admin-sidebar")).toBeVisible({ timeout: 5000 });

    // Click "posts" in sidebar
    await page.locator(".admin-sidebar a", { hasText: "posts" }).click();

    // Should show spreadsheet with data (2 active + 1 deleted via admin list)
    await expect(page.locator("table.spreadsheet")).toBeVisible({
      timeout: 5000,
    });
    await expect(page.locator("table.spreadsheet tbody tr")).toHaveCount(3);
  });

  test("can click a cell to edit it", async ({ page }) => {
    // Login and navigate to posts
    await page.goto("/admin#/login");
    await page.fill("#admin-email", "test@test.com");
    await page.fill("#admin-password", "password");
    await page.click("#admin-login-btn");
    await expect(page.locator(".admin-sidebar")).toBeVisible({ timeout: 5000 });

    await page.goto("/admin#/tables/posts");
    await expect(page.locator("table.spreadsheet")).toBeVisible({
      timeout: 5000,
    });

    // Find an editable cell (title column, first row) and click it
    const titleCell = page
      .locator("table.spreadsheet tbody tr")
      .first()
      .locator("td")
      .nth(1); // title is second column
    await titleCell.locator(".cell-display").click();

    // Should turn into an input
    const input = titleCell.locator("input");
    await expect(input).toBeVisible();
    await expect(input).toHaveValue("Hello World");
  });

  test("shows + New Row button and can add a row", async ({ page }) => {
    await page.goto("/admin#/login");
    await page.fill("#admin-email", "test@test.com");
    await page.fill("#admin-password", "password");
    await page.click("#admin-login-btn");
    await expect(page.locator(".admin-sidebar")).toBeVisible({ timeout: 5000 });

    await page.goto("/admin#/tables/posts");
    await expect(page.locator("table.spreadsheet")).toBeVisible({
      timeout: 5000,
    });

    const rowsBefore = await page
      .locator("table.spreadsheet tbody tr")
      .count();

    // Click + New Row
    await page.locator(".btn-add").click();

    const rowsAfter = await page
      .locator("table.spreadsheet tbody tr")
      .count();
    expect(rowsAfter).toBe(rowsBefore + 1);

    // New row should have new-row class
    const newRow = page.locator("table.spreadsheet tbody tr.new-row");
    await expect(newRow).toBeVisible();
  });

  test("shows deleted rows with strikethrough styling", async ({ page }) => {
    await page.goto("/admin#/login");
    await page.fill("#admin-email", "test@test.com");
    await page.fill("#admin-password", "password");
    await page.click("#admin-login-btn");
    await expect(page.locator(".admin-sidebar")).toBeVisible({ timeout: 5000 });

    await page.goto("/admin#/tables/posts");
    await expect(page.locator("table.spreadsheet")).toBeVisible({
      timeout: 5000,
    });

    // Should have a deleted row (the soft-deleted post)
    const deletedRow = page.locator("table.spreadsheet tbody tr.deleted");
    await expect(deletedRow).toBeVisible();

    // Should show Restore button on deleted row
    await expect(deletedRow.locator(".btn-restore")).toBeVisible();
  });

  test("managed_files table shows Upload File button", async ({ page }) => {
    await page.goto("/admin#/login");
    await page.fill("#admin-email", "test@test.com");
    await page.fill("#admin-password", "password");
    await page.click("#admin-login-btn");
    await expect(page.locator(".admin-sidebar")).toBeVisible({ timeout: 5000 });

    // Navigate to managed_files table
    await page
      .locator(".admin-sidebar a", { hasText: "managed_files" })
      .click();

    await expect(page.locator("table.spreadsheet")).toBeVisible({
      timeout: 5000,
    });

    // Should show Upload File button
    const uploadBtn = page.locator("button", { hasText: "Upload File" });
    await expect(uploadBtn).toBeVisible();
  });

  test("managed_files table shows download links for uploaded files", async ({
    page,
  }) => {
    await page.goto("/admin#/login");
    await page.fill("#admin-email", "test@test.com");
    await page.fill("#admin-password", "password");
    await page.click("#admin-login-btn");
    await expect(page.locator(".admin-sidebar")).toBeVisible({ timeout: 5000 });

    // Navigate to managed_files table
    await page
      .locator(".admin-sidebar a", { hasText: "managed_files" })
      .click();

    await expect(page.locator("table.spreadsheet")).toBeVisible({
      timeout: 5000,
    });

    // Should have download links for the uploaded files
    const downloadLinks = page.locator(".btn-download");
    await expect(downloadLinks.first()).toBeVisible({ timeout: 5000 });

    // Download link should point to the file download endpoint
    const href = await downloadLinks.first().getAttribute("href");
    expect(href).toContain("/files/");
    expect(href).toContain("/download");
  });

  test("can upload a file via managed_files table", async ({ page }) => {
    await page.goto("/admin#/login");
    await page.fill("#admin-email", "test@test.com");
    await page.fill("#admin-password", "password");
    await page.click("#admin-login-btn");
    await expect(page.locator(".admin-sidebar")).toBeVisible({ timeout: 5000 });

    // Navigate to managed_files table
    await page
      .locator(".admin-sidebar a", { hasText: "managed_files" })
      .click();

    await expect(page.locator("table.spreadsheet")).toBeVisible({
      timeout: 5000,
    });

    const rowsBefore = await page
      .locator("table.spreadsheet tbody tr")
      .count();

    // Trigger file upload via the file input
    // We need to use page.setInputFiles on the hidden file input
    // The Upload File button creates a dynamic input, so we use a filechooser
    const [fileChooser] = await Promise.all([
      page.waitForEvent("filechooser"),
      page.locator("button", { hasText: "Upload File" }).click(),
    ]);

    // Create a test file
    await fileChooser.setFiles({
      name: "test-upload.txt",
      mimeType: "text/plain",
      buffer: Buffer.from("Hello from Playwright E2E test!"),
    });

    // Wait for the upload to complete and the table to refresh
    // The table should have one more row now
    await expect(page.locator("table.spreadsheet tbody tr")).toHaveCount(
      rowsBefore + 1,
      { timeout: 10000 }
    );
  });

  test("logout returns to login page", async ({ page }) => {
    await page.goto("/admin#/login");
    await page.fill("#admin-email", "test@test.com");
    await page.fill("#admin-password", "password");
    await page.click("#admin-login-btn");
    await expect(page.locator(".admin-sidebar")).toBeVisible({ timeout: 5000 });

    // Click logout
    await page.locator("button", { hasText: "Logout" }).click();

    // Should be back on login page
    await expect(page.locator(".login-box")).toBeVisible({ timeout: 5000 });
  });
});
