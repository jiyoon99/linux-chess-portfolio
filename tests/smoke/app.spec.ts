import { expect, test } from "@playwright/test";

test("registers, starts an AI game, and runs analysis", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { name: "Linux Chess" })).toBeVisible();
  await expect(page.getByText("Connected")).toBeVisible();
  await expect(page.getByText("API").locator("..")).toContainText("online");

  await page.getByRole("button", { name: "Register" }).click();
  await page.getByPlaceholder("username").fill(`smoke_${Date.now()}`);
  await page.getByPlaceholder("password").fill("correct-password");
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(page.getByText("Signed in as")).toBeVisible();
  await page.getByRole("button", { name: "Play AI" }).click();

  await expect(page.locator(".matchPanel")).toContainText("Playing");
  await page.getByRole("button", { name: "Run Analysis" }).click();
  await expect(page.getByText("Best move")).toBeVisible();
  await expect(page.getByText(/heuristic|stockfish/).first()).toBeVisible();
});

test("creates a private room and lets another browser join", async ({ browser }) => {
  const owner = await browser.newPage();
  const guest = await browser.newPage();

  await owner.goto("/");
  await guest.goto("/");
  await expect(owner.getByText("Connected")).toBeVisible();
  await expect(guest.getByText("Connected")).toBeVisible();

  await owner.getByRole("button", { name: "Create Room" }).click();
  await expect(owner.getByText(/Room [A-Z0-9]{6} waiting/)).toBeVisible();
  const roomText = await owner.locator(".metric").filter({ hasText: "Room" }).locator("strong").innerText();

  await guest.getByPlaceholder("ABC123").fill(roomText);
  await guest.getByRole("button", { name: "Join" }).click();

  await expect(owner.locator(".matchPanel")).toContainText("Playing");
  await expect(guest.locator(".matchPanel")).toContainText("Playing");

  await owner.close();
  await guest.close();
});

test("resigns an AI game and loads saved game detail", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("Connected")).toBeVisible();

  await page.getByRole("button", { name: "Register" }).click();
  await page.getByPlaceholder("username").fill(`history_${Date.now()}`);
  await page.getByPlaceholder("password").fill("correct-password");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByText("Signed in as")).toBeVisible();

  await page.getByRole("button", { name: "Play AI" }).click();
  await expect(page.locator(".matchPanel")).toContainText("Playing");
  await page.getByRole("button", { name: "Resign" }).click();

  await expect(page.getByText("Resignation").first()).toBeVisible();

  const recent = await page.evaluate(async () => {
    const response = await fetch("/games/recent", { credentials: "include" });
    return response.json();
  });
  expect(recent.games.length).toBeGreaterThan(0);

  const detail = await page.evaluate(async (gameId) => {
    const response = await fetch(`/games/detail?id=${encodeURIComponent(gameId)}`, { credentials: "include" });
    return response.json();
  }, recent.games[0].id);

  expect(detail.id).toBe(recent.games[0].id);
  expect(detail.method).toBe("Resignation");
});
