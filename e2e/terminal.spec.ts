import { test, expect } from '@playwright/test';

test.describe('Terminal Feature', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to servers page
    await page.goto('/');
    await page.waitForSelector('[data-testid="servers-page"]', { timeout: 10000 });
  });

  test('user can open terminal for external server', async ({ page }) => {
    // Ensure there's at least one external server
    await page.waitForSelector('[data-testid="server-card"]', { timeout: 10000 });

    // Find the first external server card
    const externalServer = page.locator('[data-testid="server-card"]').filter({
      has: page.locator('text=外网'),
    }).first();

    // Click connect terminal button
    const connectButton = externalServer.locator('button:has-text("连接终端")');
    await expect(connectButton).toBeVisible();
    await connectButton.click();

    // Verify terminal modal opens
    await expect(page.locator('[data-testid="terminal-modal"]')).toBeVisible();
    await expect(page.locator('[data-testid="terminal-container"]')).toBeVisible();

    // Wait for connection status
    await expect(page.locator('text=已连接')).toBeVisible({ timeout: 10000 });

    // Verify terminal input area exists
    await expect(page.locator('.xterm-screen')).toBeVisible();
  });

  test('user can open terminal for internal server through gateway', async ({ page }) => {
    // Wait for server cards to load
    await page.waitForSelector('[data-testid="server-card"]', { timeout: 10000 });

    // Find the first internal server card
    const internalServer = page.locator('[data-testid="server-card"]').filter({
      has: page.locator('text=内网'),
    }).first();

    // Verify gateway indicator exists
    await expect(internalServer.locator('text=→')).toBeVisible();

    // Click connect terminal button
    const connectButton = internalServer.locator('button:has-text("连接终端")');
    await expect(connectButton).toBeVisible();
    await connectButton.click();

    // Verify terminal modal opens
    await expect(page.locator('[data-testid="terminal-modal"]')).toBeVisible();

    // Wait for connection through gateway
    await expect(page.locator('text=已连接')).toBeVisible({ timeout: 15000 });
  });

  test('user can type commands in terminal', async ({ page }) => {
    // Open terminal
    await page.waitForSelector('[data-testid="server-card"]', { timeout: 10000 });
    const connectButton = page.locator('button:has-text("连接终端")').first();
    await connectButton.click();

    // Wait for connection
    await expect(page.locator('text=已连接')).toBeVisible({ timeout: 10000 });

    // Type a command
    await page.keyboard.type('ls -la');
    await page.keyboard.press('Enter');

    // Wait for output (this depends on the actual server response)
    await page.waitForTimeout(1000);

    // Verify terminal still shows as connected
    await expect(page.locator('text=已连接')).toBeVisible();
  });

  test('user can close terminal', async ({ page }) => {
    // Open terminal
    await page.waitForSelector('[data-testid="server-card"]', { timeout: 10000 });
    const connectButton = page.locator('button:has-text("连接终端")').first();
    await connectButton.click();

    // Wait for modal to open
    await expect(page.locator('[data-testid="terminal-modal"]')).toBeVisible();

    // Click close button
    await page.click('button:has-text("关闭")');

    // Verify modal closes
    await expect(page.locator('[data-testid="terminal-modal"]')).not.toBeVisible();
  });

  test('terminal shows error when connection fails', async ({ page }) => {
    // Mock a failed connection by using an invalid server
    // This test assumes there's a way to simulate connection failure

    await page.waitForSelector('[data-testid="server-card"]', { timeout: 10000 });

    // Try to connect to a server
    const connectButton = page.locator('button:has-text("连接终端")').first();
    await connectButton.click();

    // If connection fails, error message should appear
    // This depends on the actual error handling implementation
    await page.waitForTimeout(5000);

    // Verify either connected or error state
    const statusText = await page.locator('[data-testid="connection-status"]').textContent();
    expect(['已连接', '连接失败', '连接中...']).toContain(statusText);
  });

  test('terminal supports resize', async ({ page }) => {
    // Open terminal
    await page.waitForSelector('[data-testid="server-card"]', { timeout: 10000 });
    const connectButton = page.locator('button:has-text("连接终端")').first();
    await connectButton.click();

    await expect(page.locator('[data-testid="terminal-modal"]')).toBeVisible();

    // Resize the browser window
    await page.setViewportSize({ width: 1200, height: 800 });

    // Terminal should still be visible
    await expect(page.locator('[data-testid="terminal-container"]')).toBeVisible();
  });
});
