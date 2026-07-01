import { expect, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import path from "node:path";
import { installDesignQaFixtures } from "./design-qa.fixtures";

type Theme = "light" | "dark";

type Surface = {
  name: string;
  path: string;
  expectText: RegExp;
  minImages: number;
  minImageCoverage?: number;
  skipOcclusion?: boolean;
  setup?: (page: Page) => Promise<void>;
};

const screenshotRoot = path.join(process.cwd(), "test-results", "design-qa");

const surfaces: Surface[] = [
  { name: "gallery", path: "/", expectText: /Gallery|Recently gathered|Red Room Study/, minImages: 4 },
  { name: "folios", path: "/folios", expectText: /Folios|Reference Walls|New folio/, minImages: 3 },
  { name: "folio-detail", path: "/folios/1", expectText: /Reference Walls|piece|Add pieces/, minImages: 4 },
  {
    name: "viewer",
    path: "/",
    expectText: /Red Room Study|Mara Vale|Favorite/,
    minImages: 1,
    minImageCoverage: 0.08,
    skipOcclusion: true,
    setup: async (page) => {
      await page.locator("figure").filter({ has: page.locator("img") }).first().click();
      await expect(page.getByLabel("Close")).toBeVisible();
    },
  },
  { name: "inbox", path: "/inbox", expectText: /Inbox|Keep|Dismiss|Add to folio/, minImages: 2, minImageCoverage: 0.005, skipOcclusion: true },
  { name: "streams", path: "/streams", expectText: /Streams|Telegram|Web Gallery/, minImages: 0 },
  { name: "settings", path: "/settings", expectText: /Settings|Preferences|Theme/, minImages: 0 },
  {
    name: "add-piece",
    path: "/",
    expectText: /Add Piece|Title|Source|Notes/,
    minImages: 4,
    setup: async (page) => {
      await page.getByRole("button", { name: /^Add Piece$/i }).first().click();
      await expect(page.getByRole("dialog", { name: "Add a piece" })).toBeVisible();
    },
  },
];

for (const theme of ["light", "dark"] as Theme[]) {
  test.describe(`design QA ${theme}`, () => {
    test.beforeEach(async ({ page }) => {
      await installDesignQaFixtures(page);
      await page.addInitScript((themeName) => {
        window.localStorage.setItem("okfolio-theme", themeName);
        window.localStorage.setItem("okfolio-info-panel-mode", "remember");
      }, theme);
    });

    for (const surface of surfaces) {
      test(`${surface.name} renders without obvious visual regressions`, async ({ page }, testInfo) => {
        await page.goto(surface.path);
        await expect(page.locator("body")).toContainText(surface.expectText);
        await surface.setup?.(page);
        await page.evaluate(() => document.fonts.ready);
        await page.waitForLoadState("networkidle");

        await assertFirstPaintDensity(page, surface.minImages, surface.minImageCoverage);
        await assertNoBrokenImages(page);
        await assertNoHorizontalOverflow(page);
        if (!surface.skipOcclusion) {
          await assertNoCenterOcclusion(page);
        }

        const outDir = path.join(screenshotRoot, testInfo.project.name, theme);
        mkdirSync(outDir, { recursive: true });
        await page.screenshot({
          path: path.join(outDir, `${surface.name}.png`),
          fullPage: true,
          animations: "disabled",
        });
      });
    }

    test("folio add pieces sends a serialized batch and one summary toast", async ({ page }) => {
      const postStarts: number[] = [];
      let inFlight = 0;
      let maxInFlight = 0;

      await page.route("**/api/v1/folios/1/pieces", async (route) => {
        if (route.request().method() !== "POST") {
          await route.fallback();
          return;
        }
        postStarts.push(Date.now());
        inFlight += 1;
        maxInFlight = Math.max(maxInFlight, inFlight);
        await new Promise((resolve) => setTimeout(resolve, 40));
        inFlight -= 1;
        const body = route.request().postDataJSON() as { photo_id?: number };
        await route.fulfill({
          status: 201,
          contentType: "application/json",
          body: JSON.stringify(body.photo_id === 108 ? { added: false, duplicate: true } : { added: true }),
        });
      });

      await page.goto("/folios/1");
      await page.getByRole("button", { name: "Add pieces" }).first().click();
      await page.getByRole("button", { name: /Catalog Fragment/i }).click();
      await page.getByRole("button", { name: /Evening Proof/i }).click();
      await page.getByRole("button", { name: /Studio Shelf/i }).click();
      await page.getByRole("button", { name: /^Add (3 pieces|pieces)$/ }).last().click();

      await expect(page.getByRole("status").filter({ hasText: /Added 2 pieces to folio/ })).toHaveCount(1);
      await expect(page.getByRole("status").filter({ hasText: /2 added · 1 already in folio/ })).toHaveCount(1);
      await expect.poll(() => postStarts.length).toBe(3);
      expect(maxInFlight, "bulk add requests should be serialized").toBe(1);
      await expect(page.getByRole("status").filter({ hasText: /Adding piece to folio|Couldn’t add piece to folio/ })).toHaveCount(0);
    });
  });
}

async function assertFirstPaintDensity(page: Page, minImages: number, minImageCoverage = 0.08) {
  const density = await page.evaluate(() => {
    const viewportArea = window.innerWidth * window.innerHeight;
    const visibleImages = Array.from(document.images).filter((image) => {
      const rect = image.getBoundingClientRect();
      return rect.width > 24 && rect.height > 24 && rect.bottom > 0 && rect.right > 0 && rect.top < window.innerHeight && rect.left < window.innerWidth;
    });
    const imageArea = visibleImages.reduce((sum, image) => {
      const rect = image.getBoundingClientRect();
      const visibleWidth = Math.max(0, Math.min(rect.right, window.innerWidth) - Math.max(rect.left, 0));
      const visibleHeight = Math.max(0, Math.min(rect.bottom, window.innerHeight) - Math.max(rect.top, 0));
      return sum + visibleWidth * visibleHeight;
    }, 0);
    const visibleText = (document.body.innerText || "").replace(/\s+/g, " ").trim();
    return {
      imageCount: visibleImages.length,
      imageCoverage: viewportArea > 0 ? imageArea / viewportArea : 0,
      textLength: visibleText.length,
    };
  });

  expect(density.textLength, "surface should not first-paint blank").toBeGreaterThan(80);
  if (minImages > 0) {
    expect(density.imageCount, "image-first surfaces should show fixture artwork on first paint").toBeGreaterThanOrEqual(minImages);
    expect(density.imageCoverage, "image-first surfaces should reserve meaningful first-paint artwork area").toBeGreaterThan(minImageCoverage);
  }
}

async function assertNoBrokenImages(page: Page) {
  const broken = await page.evaluate(() =>
    Array.from(document.images)
      .filter((image) => image.complete && image.naturalWidth === 0)
      .map((image) => image.getAttribute("src") || image.getAttribute("alt") || "unknown"),
  );
  expect(broken, "images should decode instead of falling back to broken placeholders").toEqual([]);
}

async function assertNoHorizontalOverflow(page: Page) {
  const overflow = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
  expect(overflow.scrollWidth, "page should not overflow horizontally").toBeLessThanOrEqual(overflow.clientWidth + 2);
}

async function assertNoCenterOcclusion(page: Page) {
  const occluded = await page.evaluate(() => {
    const selector = "button, a, input, select, textarea, [role='button'], [role='tab'], [role='switch'], h1, h2";
    const visibleDialogs = Array.from(document.querySelectorAll<HTMLElement>("[role='dialog']"))
      .filter((dialog) => {
        const rect = dialog.getBoundingClientRect();
        const style = window.getComputedStyle(dialog);
        return style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0;
      });
    const root: ParentNode = visibleDialogs.at(-1) ?? document;
    return Array.from(root.querySelectorAll<HTMLElement>(selector))
      .filter((element) => {
        const style = window.getComputedStyle(element);
        const rect = element.getBoundingClientRect();
        return style.visibility !== "hidden" && style.display !== "none" && rect.width >= 12 && rect.height >= 12 && rect.bottom > 0 && rect.right > 0 && rect.top < window.innerHeight && rect.left < window.innerWidth;
      })
      .map((element) => {
        const rect = element.getBoundingClientRect();
        const x = Math.min(Math.max(rect.left + rect.width / 2, 1), window.innerWidth - 1);
        const y = Math.min(Math.max(rect.top + rect.height / 2, 1), window.innerHeight - 1);
        if (window.innerWidth <= 700 && y > window.innerHeight - 92) return null;
        const top = document.elementFromPoint(x, y);
        const nearestControl = top?.closest(selector);
        const sameVisibleText = top?.textContent?.trim() === element.textContent?.trim();
        return top && (top === element || element.contains(top) || nearestControl === element || sameVisibleText)
          ? null
          : {
              tag: element.tagName.toLowerCase(),
              label: element.getAttribute("aria-label") || element.textContent?.trim().slice(0, 80) || "",
              coveredBy: top?.tagName.toLowerCase() || "none",
            };
      })
      .filter(Boolean);
  });

  expect(occluded, "interactive controls and headings should not be center-occluded").toEqual([]);
}
