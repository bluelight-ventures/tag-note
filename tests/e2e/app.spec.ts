import { expect, test } from '@playwright/test';

async function login(page) {
  await page.goto('/app');
  await expect(page.getByTestId('login-button')).toBeVisible();
  await page.locator('#auth-email').fill('test@test.com');
  await page.locator('#auth-password').fill('testpass123');
  await page.getByTestId('login-button').click();
  await expect(page.getByTestId('new-note-button')).toBeVisible();
}

async function createNote(page, content: string, tag: string) {
  await page.getByTestId('new-note-button').click();
  await expect(page.locator('#focus-overlay')).toBeVisible();

  await page.evaluate((value) => {
    const codeMirror = (document.querySelector('.CodeMirror') as HTMLElement & {
      CodeMirror?: { setValue: (next: string) => void };
    } | null)?.CodeMirror;
    if (codeMirror) {
      codeMirror.setValue(value);
      return;
    }
    const textarea = document.querySelector<HTMLTextAreaElement>('#focus-content');
    if (textarea) textarea.value = value;
  }, content);

  await page.locator('#focus-tag-input').fill(tag);
  await page.keyboard.press('Enter');
  await page.getByTestId('save-note-button').click();

  const card = page.getByTestId('note-card').filter({ hasText: content });
  await expect(card).toBeVisible();
  return card;
}

test('landing page and app shell load', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: /Tag your thinking/i })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Open App' })).toBeVisible();

  await page.goto('/app');
  await expect(page.getByTestId('login-button')).toBeVisible();
  await expect(page.getByTestId('guest-mode-button')).toBeVisible();
});

test('public health endpoint exposes only minimal liveness', async ({ request }) => {
  const response = await request.get('/healthz');
  expect(response.status()).toBe(200);
  await expect(response).toBeOK();
  expect(await response.json()).toEqual({ status: 'ok' });
});

test('test user can create, filter, and delete notes', async ({ page }) => {
  const suffix = Date.now().toString(36);
  const primaryContent = `E2E primary note ${suffix}`;
  const secondaryContent = `E2E secondary note ${suffix}`;
  const primaryTag = `e2e-${suffix}`;
  const secondaryTag = `other-${suffix}`;

  await login(page);

  const primaryCard = await createNote(page, primaryContent, primaryTag);
  await createNote(page, secondaryContent, secondaryTag);

  await primaryCard.getByTestId('note-tag').filter({ hasText: `#${primaryTag}` }).click();
  await expect(page.getByTestId('note-card').filter({ hasText: primaryContent })).toBeVisible();
  await expect(page.getByTestId('note-card').filter({ hasText: secondaryContent })).toHaveCount(0);

  await page.getByTestId('note-card').filter({ hasText: primaryContent }).getByTestId('delete-note-button').click();
  await expect(page.getByTestId('note-card').filter({ hasText: primaryContent })).toHaveCount(0);
});

test('guest mode can create a local note', async ({ page }) => {
  const suffix = Date.now().toString(36);
  const content = `Guest E2E note ${suffix}`;
  const tag = `guest-${suffix}`;

  await page.goto('/app');
  await page.getByTestId('guest-mode-button').click();
  await expect(page.getByTestId('new-note-button')).toBeVisible();

  await createNote(page, content, tag);
  await expect(page.getByTestId('note-card').filter({ hasText: content })).toBeVisible();
});

test('operational endpoints require explicit credentials', async ({ request }) => {
  for (const path of ['/status', '/metrics']) {
    const directResponse = await request.get(path);
    expect(directResponse.status(), `${path} should reject unauthenticated direct traffic`).toBe(401);

    const publicResponse = await request.get(path, {
      headers: { 'X-Forwarded-For': '203.0.113.10' },
    });
    expect(publicResponse.status(), `${path} should reject public unauthenticated traffic`).toBe(401);

    const tokenResponse = await request.get(path, {
      headers: {
        'X-Forwarded-For': '203.0.113.10',
        Authorization: 'Bearer e2e-operational-token',
      },
    });
    expect(tokenResponse.status(), `${path} should accept the operational bearer token`).toBe(200);
  }
});

test('admin jwt can access operational status from public proxy traffic', async ({ request }) => {
  const loginResponse = await request.post('/api/v1/auth/login', {
    data: {
      email: 'test@test.com',
      password: 'testpass123',
    },
  });
  expect(loginResponse.ok()).toBeTruthy();
  const body = await loginResponse.json();
  expect(body.token).toBeTruthy();

  const statusResponse = await request.get('/status', {
    headers: {
      'X-Forwarded-For': '203.0.113.10',
      Authorization: `Bearer ${body.token}`,
    },
  });
  expect(statusResponse.status()).toBe(200);
  await expect(statusResponse).toBeOK();
});
