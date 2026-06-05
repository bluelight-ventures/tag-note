import { expect, test } from '@playwright/test';

// Tests run in parallel (see playwright.config.ts). They share the seeded
// test@test.com account and the same database, so every test must scope its
// data to a globally unique suffix to avoid cross-test interference.
function uniqueSuffix() {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

let adminToken = '';

async function getAdminToken(request) {
  if (adminToken) return adminToken;
  const loginResponse = await request.post('/api/v1/auth/login', {
    data: {
      email: 'test@test.com',
      password: 'testpass123',
    },
  });
  expect(loginResponse.ok()).toBeTruthy();
  const body = await loginResponse.json();
  expect(body.token).toBeTruthy();
  adminToken = body.token;
  return adminToken;
}

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

  await setFocusEditorContent(page, content);
  await page.locator('#focus-tag-input').fill(tag);
  await page.keyboard.press('Enter');

  const saveStatus = page.locator('#focus-save-status');
  try {
    await expect(saveStatus).toHaveText('Saved', { timeout: 2_000 });
    await page.locator('#focus-close').click();
  } catch {
    await page.getByTestId('save-note-button').click();
  }

  await expect(page.locator('#focus-overlay')).toBeHidden();
  const card = page.getByTestId('note-card').filter({ hasText: content });
  await expect(card).toBeVisible();
  return card;
}

async function setFocusEditorContent(page, content: string) {
  await expect(page.locator('#focus-overlay .CodeMirror')).toBeVisible();
  await page.waitForFunction(() => {
    const el = document.querySelector('#focus-overlay .CodeMirror') as HTMLElement & {
      CodeMirror?: { getValue: () => string };
    } | null;
    return !!el?.CodeMirror;
  });
  await page.evaluate((value) => {
    const codeMirror = (document.querySelector('#focus-overlay .CodeMirror') as HTMLElement & {
      CodeMirror?: { setValue: (next: string) => void; focus: () => void };
    } | null)?.CodeMirror;
    if (codeMirror) {
      codeMirror.setValue(value);
      codeMirror.focus();
      return;
    }
    const textarea = document.querySelector<HTMLTextAreaElement>('#focus-content');
    if (textarea) {
      textarea.value = value;
      textarea.dispatchEvent(new Event('input', { bubbles: true }));
    }
  }, content);
  await page.waitForFunction((value) => {
    const codeMirror = (document.querySelector('#focus-overlay .CodeMirror') as HTMLElement & {
      CodeMirror?: { getValue: () => string };
    } | null)?.CodeMirror;
    return codeMirror ? codeMirror.getValue() === value : false;
  }, content);
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
  const suffix = uniqueSuffix();
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
  const suffix = uniqueSuffix();
  const content = `Guest E2E note ${suffix}`;
  const tag = `guest-${suffix}`;

  await page.goto('/app');
  await page.getByTestId('guest-mode-button').click();
  await expect(page.getByTestId('new-note-button')).toBeVisible();

  await createNote(page, content, tag);
  await expect(page.getByTestId('note-card').filter({ hasText: content })).toBeVisible();
});

test('test user note creation autosaves and closes without discard warning', async ({ page }) => {
  const suffix = uniqueSuffix();
  const content = `Autosaved E2E note ${suffix}`;
  const tag = `autosave-${suffix}`;

  await login(page);
  await page.getByTestId('new-note-button').click();
  await expect(page.locator('#focus-overlay')).toBeVisible();

  await setFocusEditorContent(page, content);
  await page.locator('#focus-tag-input').fill(tag);
  await page.keyboard.press('Enter');

  await expect(page.locator('#focus-save-status')).toHaveText('Saved');
  await page.locator('#focus-close').click();
  await expect(page.locator('#modal-overlay')).toBeHidden();
  await expect(page.locator('#focus-overlay')).toBeHidden();
  await expect(page.getByTestId('note-card').filter({ hasText: content })).toBeVisible();
});

test('test user note edits autosave without clicking save', async ({ page }) => {
  const suffix = uniqueSuffix();
  const originalContent = `Autosave edit original ${suffix}`;
  const editedContent = `Autosave edit updated ${suffix}`;
  const tag = `autosave-edit-${suffix}`;

  await login(page);
  const card = await createNote(page, originalContent, tag);
  await card.getByTestId('edit-note-button').click();
  await expect(page.locator('#focus-overlay')).toBeVisible();

  await setFocusEditorContent(page, editedContent);
  await expect(page.locator('#focus-save-status')).toHaveText('Saved');
  await page.locator('#focus-close').click();

  await expect(page.locator('#focus-overlay')).toBeHidden();
  await expect(page.getByTestId('note-card').filter({ hasText: editedContent })).toBeVisible();
});

test('guest mode note creation autosaves locally', async ({ page }) => {
  const suffix = uniqueSuffix();
  const content = `Guest autosaved note ${suffix}`;
  const tag = `guest-auto-${suffix}`;

  await page.goto('/app');
  await page.getByTestId('guest-mode-button').click();
  await expect(page.getByTestId('new-note-button')).toBeVisible();

  await page.getByTestId('new-note-button').click();
  await expect(page.locator('#focus-overlay')).toBeVisible();
  await setFocusEditorContent(page, content);
  await page.locator('#focus-tag-input').fill(tag);
  await page.keyboard.press('Enter');

  await expect(page.locator('#focus-save-status')).toHaveText('Saved');
  await page.locator('#focus-close').click();
  await expect(page.locator('#focus-overlay')).toBeHidden();
  await expect(page.getByTestId('note-card').filter({ hasText: content })).toBeVisible();
});

test('account deletion removes uploaded image files', async ({ request }) => {
  const suffix = uniqueSuffix();
  const email = `delete-upload-${suffix}@example.com`;
  const password = 'testpass123';

  const registerResponse = await request.post('/api/v1/auth/register', {
    data: {
      email,
      password,
      display_name: 'Delete Upload E2E',
    },
  });
  expect(registerResponse.status()).toBe(201);
  const registerBody = await registerResponse.json();
  expect(registerBody.token).toBeTruthy();

  const uploadResponse = await request.post('/api/v1/images', {
    headers: {
      Authorization: `Bearer ${registerBody.token}`,
    },
    multipart: {
      file: {
        name: 'account-delete.png',
        mimeType: 'image/png',
        buffer: Buffer.from('png-bytes'),
      },
    },
  });
  expect(uploadResponse.status()).toBe(201);
  const uploadBody = await uploadResponse.json();
  const filePath = uploadBody.data?.filePath;
  expect(filePath).toMatch(/^\/uploads\/[a-z0-9]+\.png$/);

  const uploadedBeforeDelete = await request.get(filePath);
  expect(uploadedBeforeDelete.status()).toBe(200);

  const deleteResponse = await request.delete('/api/v1/auth/account', {
    headers: {
      Authorization: `Bearer ${registerBody.token}`,
    },
  });
  expect(deleteResponse.status()).toBe(204);

  // The file is unlinked synchronously, but fasthttp's static file-handle cache
  // can keep serving the descriptor opened by the GET above for a short window
  // (shortened in test mode). Poll until the cache evicts and the file 404s.
  await expect
    .poll(async () => (await request.get(filePath)).status(), {
      timeout: 15_000,
      intervals: [2_000],
    })
    .toBe(404);
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

test('admin jwt can access operational status', async ({ request }) => {
  const token = await getAdminToken(request);

  const statusResponse = await request.get('/status', {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
  expect(statusResponse.status()).toBe(200);
  await expect(statusResponse).toBeOK();
});

test('app page injects only the web Google client ID, not the audience list', async ({ request }) => {
  // GOOGLE_CLIENT_ID may be a comma-separated list of accepted token audiences
  // (web, iOS, ...) for backend verification. The browser must be initialized
  // with a single, valid client ID (the web one) — injecting the whole list
  // makes Google reject sign-in with "invalid_client". (Regression guard.)
  const res = await request.get('/app');
  expect(res.ok()).toBeTruthy();
  const html = await res.text();

  const match = html.match(/window\.GOOGLE_CLIENT_ID\s*=\s*"([^"]*)"/);
  test.skip(!match, 'Google Sign-In is not configured on this server');

  const clientId = match![1];
  expect(clientId).not.toContain(',');
  expect(clientId).toMatch(/\.apps\.googleusercontent\.com$/);
  // The CI e2e server is configured with web-e2e,... so the web id is first.
  expect(clientId).toBe('web-e2e.apps.googleusercontent.com');
});

test('app page injects the Apple Services ID when web Apple sign-in is configured', async ({ request }) => {
  // The web Apple flow is initialized with the Services ID (APPLE_WEB_CLIENT_ID).
  const res = await request.get('/app');
  expect(res.ok()).toBeTruthy();
  const html = await res.text();

  const match = html.match(/window\.APPLE_CLIENT_ID\s*=\s*"([^"]*)"/);
  test.skip(!match, 'Sign in with Apple (web) is not configured on this server');

  // The CI e2e server is configured with the web Services ID.
  expect(match![1]).toBe('com.tag-note.web-e2e');
});

test('admin dashboard displays protected metrics', async ({ page, request }) => {
  const token = await getAdminToken(request);

  await page.addInitScript((token) => {
    localStorage.setItem('tagnote_token', token);
  }, token);

  await page.goto('/admin');
  await expect(page.getByRole('heading', { name: 'Metrics' })).toBeVisible();
  await expect(page.locator('#metrics-output')).toContainText('app_uptime_seconds');
});
