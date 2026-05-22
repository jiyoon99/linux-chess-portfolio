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

  await expect(page.getByText("Playing").first()).toBeVisible();
  await page.getByRole("button", { name: "Run Analysis" }).click();
  await expect(page.getByText("Best move")).toBeVisible();
  await expect(page.getByText(/heuristic|stockfish/).first()).toBeVisible();
});
