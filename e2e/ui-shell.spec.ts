import { expect, test } from "@playwright/test";
import { goTo, loginAsAdminUi, openCopilot, openDeclare } from "./helpers";

test("UI shell: Esc closes the declare modal and focus returns to its opener", async ({
  page,
}) => {
  await loginAsAdminUi(page);
  await openDeclare(page);
  await expect(page.getByTestId("declare-incident-form")).toBeVisible();

  await page.keyboard.press("Escape");
  await expect(page.getByTestId("declare-incident-form")).toHaveCount(0);
  await expect(page.getByTestId("declare-incident-open")).toBeFocused();
});

test("UI shell: the copilot drawer keeps its state when closed and reopened", async ({
  page,
}) => {
  await loginAsAdminUi(page);
  await openCopilot(page);
  await expect(page.getByTestId("copilot-panel")).toBeVisible();
  await page.getByTestId("copilot-input").fill("borrador sin enviar");

  await page.getByRole("button", { name: "Cerrar" }).click();
  await expect(page.getByTestId("copilot-panel")).toBeHidden();

  await openCopilot(page);
  await expect(page.getByTestId("copilot-panel")).toBeVisible();
  await expect(page.getByTestId("copilot-input")).toHaveValue(
    "borrador sin enviar",
  );
});

test("UI shell: browser back returns to the previous view", async ({
  page,
}) => {
  await loginAsAdminUi(page);
  await expect(page.getByTestId("kpi-dashboard")).toBeVisible();
  await goTo(page, "services");
  await expect(page.getByTestId("services-list")).toBeVisible();

  await page.goBack();
  await expect(page.getByTestId("kpi-dashboard")).toBeVisible();
  await expect(page.getByTestId("services-list")).toHaveCount(0);
});

test("UI shell: an unknown hash shows the dashboard", async ({ page }) => {
  await loginAsAdminUi(page);
  await goTo(page, "services");
  await expect(page.getByTestId("services-list")).toBeVisible();

  await page.evaluate(() => {
    window.location.hash = "#/foo";
  });
  await expect(page.getByTestId("kpi-dashboard")).toBeVisible();
});
