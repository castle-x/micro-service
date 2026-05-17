import { expect, test, type Page } from '@playwright/test'

const login = async (page: Page) => {
  await page.route('**/api/v1/auth/login', async (route) => {
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

  await page.goto('/login')
  await page.getByPlaceholder('Email').fill('admin@platform.com')
  await page.getByPlaceholder('Password').fill('Admin@1234')
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
}

test('llm setup page creates provider and model, toggles them, and displays provider test summaries', async ({ page }) => {
  let providerEnabled = true
  let modelEnabled = true
  let createdProvider = false
  let createdModel = false
  let createdDefaultModel = false
  let updatedProvider = false
  let updatedModel = false
  let deletedProvider = false
  let deletedModel = false
  let testedProviderDefaultModel = false
  let testedModel = false
  let legacyProviderTestCalled = false
  let providerName = 'Fake Provider'
  let defaultModelRef = 'fake/fake-chat'
  let modelDisplayName = 'Fake Chat'
  let modelCapabilities = ['chat', 'stream']

  await page.route('**/api/v1/admin/llm/providers', async (route) => {
    if (route.request().method() === 'POST') {
      const body = route.request().postDataJSON() as Record<string, unknown>
      expect(body).toMatchObject({
        name: 'Fake Provider',
        slug: 'fake',
        vendor: 'openai_compatible',
        base_url: 'http://127.0.0.1:39090',
        api_key: 'fake-key',
        default_model_ref: 'fake/fake-chat',
      })
      createdProvider = true
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ code: 0, data: { id: 'provider-new' } }),
      })
      return
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        code: 0,
        data: [
          {
            id: 'provider-1',
            name: providerName,
            slug: 'fake',
            vendor: 'openai_compatible',
            base_url: 'http://127.0.0.1:39090',
            default_model_ref: defaultModelRef,
            enabled: providerEnabled,
          },
        ],
      }),
    })
  })

  await page.route('**/api/v1/admin/llm/providers/provider-1', async (route) => {
    if (route.request().method() === 'PUT') {
      const body = route.request().postDataJSON() as Record<string, unknown>
      expect(body).toMatchObject({
        name: 'Fake Provider Updated',
        vendor: 'openai_compatible',
        base_url: 'http://127.0.0.1:39090',
        default_model_ref: 'fake/fake-chat-updated',
      })
      providerName = String(body.name)
      defaultModelRef = String(body.default_model_ref)
      updatedProvider = true
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ code: 0, data: { id: 'provider-1', name: providerName } }),
      })
      return
    }
    if (route.request().method() === 'DELETE') {
      expect(deletedModel).toBe(true)
      deletedProvider = true
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { ok: true } }) })
      return
    }
    await route.fallback()
  })

  await page.route('**/api/v1/admin/llm/providers/provider-1/api-key', async (route) => {
    expect(route.request().method()).toBe('PATCH')
    expect(route.request().postDataJSON()).toEqual({ api_key: 'new-fake-key' })
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: {} }) })
  })

  await page.route('**/api/v1/admin/llm/providers/provider-1/enabled', async (route) => {
    expect(route.request().method()).toBe('PATCH')
    const body = route.request().postDataJSON() as { enabled: boolean }
    providerEnabled = body.enabled
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: {} }) })
  })

  await page.route('**/api/v1/admin/llm/providers/provider-1/test', async (route) => {
    legacyProviderTestCalled = true
    await route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({
        code: 500,
        message: 'legacy provider test endpoint should not be used by the UI',
      }),
    })
  })

  await page.route('**/api/v1/admin/llm/models**', async (route) => {
    if (route.request().method() === 'POST') {
      const body = route.request().postDataJSON() as Record<string, unknown>
      if (body.model === 'fake-chat-updated') {
        expect(body).toMatchObject({
          provider_slug: 'fake',
          model: 'fake-chat-updated',
          display_name: 'fake-chat-updated',
          capabilities: ['chat', 'stream'],
          enabled: true,
        })
        createdDefaultModel = true
      } else {
        expect(body).toMatchObject({
          provider_slug: 'fake',
          model: 'fake-chat',
          display_name: 'Fake Chat',
          capabilities: ['chat', 'stream'],
          enabled: true,
        })
        createdModel = true
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ code: 0, data: { id: 'model-new', model_ref: `fake/${String(body.model)}` } }),
      })
      return
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        code: 0,
        data: deletedModel ? [] : [
          {
            id: 'model-1',
            provider_id: 'provider-1',
            provider_slug: 'fake',
            model: 'fake-chat',
            model_ref: 'fake/fake-chat',
            display_name: modelDisplayName,
            capabilities: modelCapabilities,
            context_window: 8192,
            max_output_tokens: 1024,
            enabled: modelEnabled,
          },
          ...(createdDefaultModel ? [{
            id: 'model-default',
            provider_id: 'provider-1',
            provider_slug: 'fake',
            model: 'fake-chat-updated',
            model_ref: 'fake/fake-chat-updated',
            display_name: 'fake-chat-updated',
            capabilities: ['chat', 'stream'],
            enabled: true,
          }] : []),
        ],
      }),
    })
  })

  await page.route('**/api/v1/admin/llm/models/model-1', async (route) => {
    if (route.request().method() === 'PUT') {
      const body = route.request().postDataJSON() as Record<string, unknown>
      expect(body).toMatchObject({
        display_name: 'Fake Chat Updated',
        capabilities: ['chat', 'stream', 'tool_calling'],
        context_window: 32768,
        max_output_tokens: 2048,
      })
      modelDisplayName = String(body.display_name)
      modelCapabilities = body.capabilities as string[]
      updatedModel = true
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ code: 0, data: { id: 'model-1', display_name: modelDisplayName } }),
      })
      return
    }
    if (route.request().method() === 'DELETE') {
      deletedModel = true
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { ok: true } }) })
      return
    }
    await route.fallback()
  })

  await page.route('**/api/v1/admin/llm/models/model-1/enabled', async (route) => {
    expect(route.request().method()).toBe('PATCH')
    const body = route.request().postDataJSON() as { enabled: boolean }
    modelEnabled = body.enabled
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: {} }) })
  })

  await page.route('**/api/v1/admin/llm/generate', async (route) => {
    expect(route.request().method()).toBe('POST')
    const body = route.request().postDataJSON() as Record<string, unknown>
    expect(body).toMatchObject({
      model_ref: 'fake/fake-chat',
      messages: [{ role: 'user', content: 'ping' }],
      max_tokens: 32,
    })
    if (!testedProviderDefaultModel) {
      testedProviderDefaultModel = true
    } else {
      testedModel = true
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        code: 0,
        data: {
          request_id: 'req-model-test',
          assistant: { role: 'assistant', content: 'pong' },
          usage: { prompt_tokens: 1, completion_tokens: 1, total_tokens: 2 },
          finish_reason: 'stop',
          model_ref: 'fake/fake-chat',
        },
      }),
    })
  })

  await login(page)
  await page.goto('/admin/llm')

  await expect(page.getByRole('heading', { name: 'AI 模型供应商' })).toBeVisible()
  await expect(page.getByText('Fake Provider')).toBeVisible()
  await expect(page.getByRole('cell', { name: 'fake/fake-chat' }).first()).toBeVisible()

  await page.getByRole('button', { name: '+ 新增供应商' }).click()
  await page.getByLabel('名称 *').fill('Fake Provider')
  await page.getByLabel('Slug *').fill('fake')
  await page.getByLabel('Vendor *').fill('openai_compatible')
  await page.getByLabel('Base URL *').fill('http://127.0.0.1:39090')
  await page.getByLabel('API Key').fill('fake-key')
  await page.getByLabel('默认模型引用').fill('fake/fake-chat')
  await page.getByRole('button', { name: '创建' }).click()
  await expect.poll(() => createdProvider).toBe(true)

  await page.getByRole('button', { name: '更新 Key' }).click()
  await page.getByPlaceholder('新的 API Key').fill('new-fake-key')
  await page.getByRole('button', { name: '保存' }).click()

  await page.getByRole('button', { name: '已启用' }).first().click()
  await expect(page.getByRole('button', { name: '已禁用' }).first()).toBeVisible()

  const providerRowForGenerateTest = page.getByRole('row').filter({ hasText: 'Fake Provider' })
  await providerRowForGenerateTest.getByRole('button', { name: '测试' }).click()
  await expect.poll(() => testedProviderDefaultModel).toBe(true)
  await expect.poll(() => legacyProviderTestCalled).toBe(false)
  await expect(providerRowForGenerateTest.getByText('OK pong')).toBeVisible()

  await page.getByRole('button', { name: '+ 新增模型' }).click()
  await page.getByLabel('Provider Slug *').fill('fake')
  await page.getByLabel('上游模型 *').fill('fake-chat')
  await page.getByLabel('显示名称').fill('Fake Chat')
  await page.getByLabel('Capabilities *').fill('chat, stream')
  await page.getByLabel('Context Window').fill('8192')
  await page.getByLabel('Max Output Tokens').fill('1024')
  await page.getByRole('button', { name: '创建模型' }).click()
  await expect.poll(() => createdModel).toBe(true)

  const createdModelRow = page.getByRole('row', { name: /fake fake\/fake-chat Fake Chat/ })
  await createdModelRow.getByRole('button', { name: '测试' }).click()
  await expect.poll(() => testedModel).toBe(true)
  await expect(createdModelRow.getByText('OK pong')).toBeVisible()

  await page.getByRole('button', { name: '已启用' }).last().click()
  await expect(page.getByRole('button', { name: '已禁用' }).last()).toBeVisible()

  const providerRow = page.getByRole('row').filter({ hasText: 'Fake Provider' })
  await providerRow.getByRole('button', { name: '编辑' }).click()
  await page.getByLabel('名称 *').fill('Fake Provider Updated')
  await page.getByLabel('默认模型引用').fill('fake/fake-chat-updated')
  await page.getByRole('button', { name: '保存' }).click()
  await expect.poll(() => updatedProvider).toBe(true)
  await expect.poll(() => createdDefaultModel).toBe(true)
  await expect(page.getByText('Fake Provider Updated')).toBeVisible()
  await expect(page.getByRole('row', { name: /fake fake\/fake-chat-updated/ })).toBeVisible()

  const modelRow = page.getByRole('row').filter({ hasText: 'Fake Chat' })
  await modelRow.getByRole('button', { name: '编辑' }).click()
  await page.getByLabel('显示名称').fill('Fake Chat Updated')
  await page.getByLabel('Capabilities *').fill('chat, stream, tool_calling')
  await page.getByLabel('Context Window').fill('32768')
  await page.getByLabel('Max Output Tokens').fill('2048')
  await page.getByRole('button', { name: '保存' }).click()
  await expect.poll(() => updatedModel).toBe(true)
  await expect(page.getByText('Fake Chat Updated')).toBeVisible()

  page.once('dialog', async (dialog) => {
    expect(dialog.message()).toContain('Fake Chat Updated')
    await dialog.accept()
  })
  await page.getByRole('row').filter({ hasText: 'Fake Chat Updated' }).getByRole('button', { name: '删除' }).click()
  await expect.poll(() => deletedModel).toBe(true)
  await expect(page.getByText('暂无模型')).toBeVisible()

  page.once('dialog', async (dialog) => {
    expect(dialog.message()).toContain('Fake Provider Updated')
    await dialog.accept()
  })
  await page.getByRole('row').filter({ hasText: 'Fake Provider Updated' }).getByRole('button', { name: '删除' }).click()
  await expect.poll(() => deletedProvider).toBe(true)
})

test('llm setup page uses current DeepSeek defaults for provider and model forms', async ({ page }) => {
  await page.route('**/api/v1/admin/llm/providers', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        code: 0,
        data: [
          {
            id: 'provider-deepseek',
            name: 'DeepSeek',
            slug: 'deepseek',
            vendor: 'openai_compatible',
            base_url: 'https://api.deepseek.com',
            default_model_ref: 'deepseek/deepseek-chat',
            enabled: true,
          },
        ],
      }),
    })
  })

  await page.route('**/api/v1/admin/llm/models**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ code: 0, data: [] }),
    })
  })

  await login(page)
  await page.goto('/admin/llm')

  await page.getByRole('button', { name: '+ 新增供应商' }).click()
  await expect(page.getByLabel('默认模型引用')).toHaveValue('deepseek/deepseek-v4-flash')
  await page.getByRole('button', { name: '取消' }).click()

  await page.getByRole('button', { name: '+ 新增模型' }).click()
  await expect(page.getByLabel('Provider Slug *')).toHaveValue('deepseek')
  await expect(page.getByLabel('上游模型 *')).toHaveValue('deepseek-v4-flash')
  await expect(page.getByLabel('显示名称')).toHaveValue('DeepSeek V4 Flash')
})
