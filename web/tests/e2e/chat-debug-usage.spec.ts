import { expect, test } from '@playwright/test'

test('chat debug shows token usage for one streamed answer', async ({ page }) => {
  await page.route('**/api/v1/auth/login', async (route) => {
    const body = route.request().postDataJSON() as { email: string; password: string }
    expect(body).toEqual({ email: 'admin@platform.com', password: 'Admin@1234' })

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        code: 0,
        data: {
          access_token: 'e2e-access-token',
          refresh_token: 'e2e-refresh-token',
          expires_at: Date.now() + 3_600_000,
          user_id: 'admin-user',
        },
      }),
    })
  })

  await page.route('**/api/v1/user/me', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        code: 0,
        data: {
          user_id: 'admin-user',
          email: 'admin@platform.com',
          name: 'Platform Admin',
          avatar_url: '',
          role: 'admin',
        },
      }),
    })
  })

  await page.route('**/api/v1/admin/models/providers', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        code: 0,
        data: [
          {
            id: 'mock-provider',
            name: 'Mock LLM',
            slug: 'mock-llm',
            type: 'llm',
            base_url: 'http://mock.local',
            default_model: 'mock-chat',
            enabled: true,
          },
        ],
      }),
    })
  })

  await page.route('**/api/v1/admin/models/chat/stream', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'text/event-stream; charset=utf-8',
      headers: {
        'Cache-Control': 'no-cache',
        Connection: 'keep-alive',
        'X-Accel-Buffering': 'no',
      },
      body: [
        'data: {"type":"content","content":"hello"}',
        '',
        'data: {"type":"content","content":" from mock"}',
        '',
        'data: {"type":"done","prompt_tokens":7,"completion_tokens":3,"total_tokens":10}',
        '',
        '',
      ].join('\n'),
    })
  })

  await page.goto('/login')
  await page.getByPlaceholder('Email').fill('admin@platform.com')
  await page.getByPlaceholder('Password').fill('Admin@1234')
  await page.getByRole('button', { name: 'Sign in' }).click()

  await expect(page).toHaveURL(/\/dashboard$/)

  await page.goto('/admin/chat-debug')
  await expect(page.getByRole('heading', { name: 'Chat 调试' })).toBeVisible()
  await expect(page.locator('select')).toHaveValue('mock-llm')

  await page.getByPlaceholder('输入消息，Enter 发送，Shift+Enter 换行').fill('hello')
  await page.getByRole('button', { name: '发送' }).click()

  await expect(page.getByText('hello from mock')).toBeVisible()
  await expect(page.getByText('↑ 7 prompt')).toBeVisible()
  await expect(page.getByText('↓ 3 completion')).toBeVisible()
  await expect(page.getByText('= 10 tokens')).toBeVisible()
})
