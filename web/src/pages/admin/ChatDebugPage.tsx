import React, { useState, useRef, useEffect } from 'react'
import { modelListProviders, modelChatStream, type ModelProvider } from '../../lib/api'

interface Message {
  role: 'user' | 'assistant' | 'system' | 'tool'
  content: string
  reasoning?: string
  streaming?: boolean
}

// ---- 高级参数状态 ----
interface AdvancedParams {
  temperature: string       // 空字符串 = 不传
  maxTokens: string
  topP: string
  stop: string              // 逗号分隔
  responseFormat: 'text' | 'json_object' | ''
  thinkingType: 'disabled' | 'enabled' | ''
  budgetTokens: string
  toolChoice: 'auto' | 'none' | 'required' | ''
  toolsJson: string         // JSON array string
  toolsJsonError: string
}

const DEFAULT_PARAMS: AdvancedParams = {
  temperature: '', maxTokens: '', topP: '', stop: '',
  responseFormat: '', thinkingType: '', budgetTokens: '',
  toolChoice: '', toolsJson: '', toolsJsonError: '',
}

function buildExtra(p: AdvancedParams): Record<string, unknown> {
  const extra: Record<string, unknown> = {}
  if (p.temperature !== '') extra.temperature = parseFloat(p.temperature)
  if (p.maxTokens !== '') extra.max_tokens = parseInt(p.maxTokens)
  if (p.topP !== '') extra.top_p = parseFloat(p.topP)
  if (p.stop !== '') extra.stop = p.stop.split(',').map(s => s.trim()).filter(Boolean)
  if (p.responseFormat) extra.response_format = { type: p.responseFormat }
  if (p.thinkingType) {
    const t: Record<string, unknown> = { type: p.thinkingType }
    if (p.thinkingType === 'enabled' && p.budgetTokens !== '') t.budget_tokens = parseInt(p.budgetTokens)
    extra.thinking = t
  }
  if (p.toolChoice) extra.tool_choice = p.toolChoice
  if (p.toolsJson.trim()) {
    try { extra.tools = JSON.parse(p.toolsJson) } catch { /* validated separately */ }
  }
  return extra
}

// ---- 样式常量 ----
const inputStyle: React.CSSProperties = {
  padding: '5px 8px', borderRadius: 6, border: '1px solid var(--border)',
  background: 'var(--bg-primary)', color: 'var(--text-primary)', fontSize: 12, width: '100%', boxSizing: 'border-box',
}
const labelStyle: React.CSSProperties = { fontSize: 11, color: 'var(--text-muted)', marginBottom: 2, display: 'block' }
const rowStyle: React.CSSProperties = { display: 'flex', gap: 10, marginBottom: 10, alignItems: 'flex-end' }

function AdvancedPanel({ params, setParams }: { params: AdvancedParams; setParams: React.Dispatch<React.SetStateAction<AdvancedParams>> }) {
  const set = (k: keyof AdvancedParams, v: string) => setParams(p => ({ ...p, [k]: v }))

  const validateTools = (v: string) => {
    let err = ''
    if (v.trim()) {
      try { JSON.parse(v) } catch { err = 'JSON 格式错误' }
    }
    setParams(p => ({ ...p, toolsJson: v, toolsJsonError: err }))
  }

  return (
    <div style={{ padding: '12px 14px', background: 'var(--bg-secondary)', borderRadius: 10, border: '1px solid var(--border)', marginBottom: 10, fontSize: 12 }}>

      {/* Row 1: sampling */}
      <div style={rowStyle}>
        <div style={{ flex: 1 }}>
          <label style={labelStyle}>Temperature</label>
          <input style={inputStyle} value={params.temperature} onChange={e => set('temperature', e.target.value)} placeholder="0~2（空=默认）" type="number" min="0" max="2" step="0.1" />
        </div>
        <div style={{ flex: 1 }}>
          <label style={labelStyle}>Max Tokens</label>
          <input style={inputStyle} value={params.maxTokens} onChange={e => set('maxTokens', e.target.value)} placeholder="空=默认" type="number" min="1" />
        </div>
        <div style={{ flex: 1 }}>
          <label style={labelStyle}>Top P</label>
          <input style={inputStyle} value={params.topP} onChange={e => set('topP', e.target.value)} placeholder="0~1（空=默认）" type="number" min="0" max="1" step="0.05" />
        </div>
        <div style={{ flex: 2 }}>
          <label style={labelStyle}>Stop（逗号分隔）</label>
          <input style={inputStyle} value={params.stop} onChange={e => set('stop', e.target.value)} placeholder="\n,###" />
        </div>
      </div>

      {/* Row 2: response_format + thinking */}
      <div style={rowStyle}>
        <div style={{ flex: 1 }}>
          <label style={labelStyle}>Response Format</label>
          <div style={{ display: 'flex', gap: 10, marginTop: 4, alignItems: 'center', flexWrap: 'wrap' }}>
            {(['', 'text', 'json_object'] as const).map(v => (
              <label key={v} style={{ display: 'flex', alignItems: 'center', gap: 4, cursor: 'pointer', color: 'var(--text-primary)', fontSize: 12 }}>
                <input type="radio" checked={params.responseFormat === v} onChange={() => set('responseFormat', v)} />
                {v === '' ? '默认' : v}
              </label>
            ))}
            {params.responseFormat === 'json_object' && (
              <span style={{ fontSize: 11, color: 'rgb(234,179,8)', background: 'rgba(234,179,8,0.1)', padding: '1px 7px', borderRadius: 4 }}>
                ⚠ 消息中需包含 "json" 字样
              </span>
            )}
          </div>
        </div>
        <div style={{ flex: 1 }}>
          <label style={labelStyle}>Thinking（DeepSeek）</label>
          <div style={{ display: 'flex', gap: 10, alignItems: 'center', marginTop: 4 }}>
            {(['', 'disabled', 'enabled'] as const).map(v => (
              <label key={v} style={{ display: 'flex', alignItems: 'center', gap: 4, cursor: 'pointer', color: 'var(--text-primary)', fontSize: 12 }}>
                <input type="radio" checked={params.thinkingType === v} onChange={() => set('thinkingType', v)} />
                {v === '' ? '默认' : v}
              </label>
            ))}
            {params.thinkingType === 'enabled' && (
              <input style={{ ...inputStyle, width: 90 }} value={params.budgetTokens} onChange={e => set('budgetTokens', e.target.value)}
                placeholder="budget_tokens" type="number" min="1" />
            )}
          </div>
        </div>
      </div>

      {/* Row 3: tool_choice */}
      <div style={{ marginBottom: 10 }}>
        <label style={labelStyle}>Tool Choice</label>
        <div style={{ display: 'flex', gap: 12, marginTop: 4 }}>
          {(['', 'auto', 'none', 'required'] as const).map(v => (
            <label key={v} style={{ display: 'flex', alignItems: 'center', gap: 4, cursor: 'pointer', color: 'var(--text-primary)', fontSize: 12 }}>
              <input type="radio" checked={params.toolChoice === v} onChange={() => set('toolChoice', v)} />
              {v === '' ? '默认' : v}
            </label>
          ))}
        </div>
      </div>

      {/* Row 4: tools JSON */}
      <div>
        <label style={labelStyle}>Tools JSON（数组格式）</label>
        <textarea
          value={params.toolsJson}
          onChange={e => validateTools(e.target.value)}
          rows={5}
          placeholder={'[\n  {\n    "type": "function",\n    "function": {\n      "name": "get_weather",\n      "description": "获取城市天气",\n      "parameters": {\n        "type": "object",\n        "properties": {"city": {"type": "string"}},\n        "required": ["city"]\n      }\n    }\n  }\n]'}
          style={{ ...inputStyle, resize: 'vertical', fontFamily: 'monospace', fontSize: 11 }}
        />
        {params.toolsJsonError && (
          <div style={{ color: 'rgb(239,68,68)', fontSize: 11, marginTop: 2 }}>⚠ {params.toolsJsonError}</div>
        )}
      </div>
    </div>
  )
}

// ---- Main Page ----

export default function ChatDebugPage() {
  const [providers, setProviders] = useState<ModelProvider[]>([])
  const [slug, setSlug] = useState('')
  const [systemPrompt, setSystemPrompt] = useState('')
  const [input, setInput] = useState('')
  const [messages, setMessages] = useState<Message[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [expandedReasoning, setExpandedReasoning] = useState<Set<number>>(new Set())
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [advancedParams, setAdvancedParams] = useState<AdvancedParams>(DEFAULT_PARAMS)
  const abortRef = useRef<AbortController | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    modelListProviders()
      .then((ps) => {
        const llms = ps.filter((p) => p.type === 'llm' && p.enabled)
        setProviders(llms)
        if (llms.length > 0) setSlug(llms[0].slug)
      })
      .catch(() => {})
  }, [])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const canSend = !advancedParams.toolsJsonError

  const send = async () => {
    if (!input.trim() || !slug || loading || !canSend) return

    const userMsg: Message = { role: 'user', content: input.trim() }
    const history = [...messages, userMsg]
    const assistantIdx = history.length

    setMessages([...history, { role: 'assistant', content: '', streaming: true }])
    setInput('')
    setLoading(true)
    setError(null)

    const toSend = (systemPrompt.trim()
      ? [{ role: 'system' as const, content: systemPrompt.trim() }, ...history]
      : history
    ).map((m) => ({ role: m.role, content: m.content }))

    const extra = buildExtra(advancedParams)
    abortRef.current = new AbortController()

    try {
      await modelChatStream(slug, toSend, (chunk) => {
        if (chunk.type === 'reasoning' && chunk.content) {
          setMessages((prev) => {
            const next = [...prev]
            const cur = next[assistantIdx]
            next[assistantIdx] = { ...cur, reasoning: (cur.reasoning ?? '') + chunk.content!, streaming: true }
            return next
          })
        } else if (chunk.type === 'content' && chunk.content) {
          setMessages((prev) => {
            const next = [...prev]
            const cur = next[assistantIdx]
            next[assistantIdx] = { ...cur, content: (cur.content ?? '') + chunk.content!, streaming: true }
            return next
          })
        } else if (chunk.type === 'done') {
          setMessages((prev) => {
            const next = [...prev]
            next[assistantIdx] = { ...next[assistantIdx], streaming: false }
            return next
          })
        } else if (chunk.type === 'error') {
          setError(chunk.message ?? '流式输出错误')
        }
      }, abortRef.current.signal, extra)

      setMessages((prev) => {
        const next = [...prev]
        if (next[assistantIdx]?.streaming) next[assistantIdx] = { ...next[assistantIdx], streaming: false }
        return next
      })
    } catch (e: any) {
      if (e?.name === 'AbortError') {
        setMessages((prev) => {
          const next = [...prev]
          if (next[assistantIdx]) next[assistantIdx] = { ...next[assistantIdx], streaming: false }
          return next
        })
        return
      }
      const msg: string = e?.message ?? ''
      setError(msg.includes('timeout') || msg.includes('ECONNABORTED') ? '请求超时，请重试' : msg || '请求失败')
      setMessages((prev) => {
        const next = [...prev]
        if (next[assistantIdx]) next[assistantIdx] = { ...next[assistantIdx], streaming: false }
        return next
      })
    } finally {
      setLoading(false)
      abortRef.current = null
    }
  }

  const stop = () => { abortRef.current?.abort(); setLoading(false) }
  const handleKey = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() }
  }
  const clear = () => { setMessages([]); setError(null) }
  const toggleReasoning = (i: number) => {
    setExpandedReasoning((prev) => {
      const next = new Set(prev)
      next.has(i) ? next.delete(i) : next.add(i)
      return next
    })
  }

  const bubbleStyle = (role: string): React.CSSProperties => ({
    maxWidth: '72%',
    padding: '10px 14px',
    borderRadius: role === 'user' ? '16px 16px 4px 16px' : '16px 16px 16px 4px',
    background: role === 'user' ? 'var(--accent)' : 'var(--bg-secondary)',
    color: role === 'user' ? '#fff' : 'var(--text-primary)',
    fontSize: 14,
    lineHeight: 1.6,
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word',
    border: role === 'assistant' ? '1px solid var(--border)' : 'none',
  })

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', maxWidth: 820, margin: '0 auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12, flexShrink: 0 }}>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700, color: 'var(--text-primary)', flex: 1 }}>
          Chat 调试
          <span style={{ marginLeft: 8, fontSize: 11, fontWeight: 400, color: 'var(--text-muted)', verticalAlign: 'middle' }}>SSE 流式</span>
        </h2>
        <select value={slug} onChange={(e) => setSlug(e.target.value)}
          style={{ padding: '6px 10px', borderRadius: 8, border: '1px solid var(--border)', background: 'var(--bg-secondary)', color: 'var(--text-primary)', fontSize: 13 }}>
          {providers.length === 0 && <option value="">（无可用 LLM 供应商）</option>}
          {providers.map((p) => (
            <option key={p.slug} value={p.slug}>{p.name} / {p.default_model || p.slug}</option>
          ))}
        </select>
        <button onClick={clear} style={{ padding: '6px 12px', borderRadius: 8, border: '1px solid var(--border)', background: 'transparent', color: 'var(--text-secondary)', fontSize: 12, cursor: 'pointer' }}>
          清空
        </button>
      </div>

      {/* System prompt */}
      <div style={{ marginBottom: 8, flexShrink: 0 }}>
        <textarea value={systemPrompt} onChange={(e) => setSystemPrompt(e.target.value)}
          placeholder="System prompt（可选）" rows={2}
          style={{ width: '100%', boxSizing: 'border-box', padding: '8px 12px', borderRadius: 8, border: '1px solid var(--border)', background: 'var(--bg-secondary)', color: 'var(--text-secondary)', fontSize: 12, resize: 'vertical', fontFamily: 'inherit' }} />
      </div>

      {/* Advanced params toggle */}
      <div style={{ flexShrink: 0, marginBottom: 8 }}>
        <button
          onClick={() => setShowAdvanced(v => !v)}
          style={{ background: 'transparent', border: 'none', color: 'var(--accent)', fontSize: 12, cursor: 'pointer', padding: 0, fontWeight: 500 }}
        >
          {showAdvanced ? '▼' : '▶'} 高级参数（response_format / tools / thinking…）
        </button>
        {showAdvanced && <AdvancedPanel params={advancedParams} setParams={setAdvancedParams} />}
      </div>

      {/* Messages */}
      <div style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 12, paddingBottom: 8 }}>
        {messages.length === 0 && !loading && (
          <div style={{ margin: 'auto', color: 'var(--text-muted)', fontSize: 13, textAlign: 'center', paddingTop: 40 }}>
            输入消息开始调试
          </div>
        )}
        {messages.map((m, i) => (
          <div key={i} style={{ display: 'flex', justifyContent: m.role === 'user' ? 'flex-end' : 'flex-start' }}>
            <div style={{ maxWidth: '80%' }}>
              <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 3, textAlign: m.role === 'user' ? 'right' : 'left' }}>
                {m.role === 'user' ? 'You' : slug}
                {m.streaming && <span style={{ marginLeft: 6, color: 'var(--accent)' }}>●</span>}
              </div>
              {m.role === 'assistant' && m.reasoning && (
                <div style={{ marginBottom: 6 }}>
                  <button onClick={() => toggleReasoning(i)} style={{
                    background: 'rgba(99,102,241,0.08)', color: 'rgb(99,102,241)',
                    border: '1px solid rgba(99,102,241,0.2)', borderRadius: 6,
                    padding: '2px 10px', fontSize: 11, cursor: 'pointer', fontWeight: 500,
                  }}>
                    {expandedReasoning.has(i) ? '▼' : '▶'} Thinking{m.streaming ? ' …' : ''}
                  </button>
                  {expandedReasoning.has(i) && (
                    <div style={{
                      marginTop: 4, padding: '8px 12px',
                      background: 'rgba(99,102,241,0.06)', borderRadius: 8,
                      border: '1px solid rgba(99,102,241,0.15)',
                      fontSize: 12, color: 'var(--text-secondary)',
                      whiteSpace: 'pre-wrap', wordBreak: 'break-word', lineHeight: 1.5,
                      maxHeight: 240, overflowY: 'auto',
                    }}>
                      {m.reasoning}
                    </div>
                  )}
                </div>
              )}
              <div style={bubbleStyle(m.role)}>
                {m.content || (m.streaming ? <span style={{ opacity: 0.4 }}>…</span> : '')}
              </div>
            </div>
          </div>
        ))}
        {error && (
          <div style={{ background: 'rgba(239,68,68,0.1)', color: 'rgb(239,68,68)', padding: '8px 14px', borderRadius: 8, fontSize: 13 }}>
            {error}
          </div>
        )}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <div style={{ display: 'flex', gap: 8, marginTop: 10, flexShrink: 0 }}>
        <textarea value={input} onChange={(e) => setInput(e.target.value)} onKeyDown={handleKey}
          placeholder="输入消息，Enter 发送，Shift+Enter 换行" rows={2} disabled={loading}
          style={{ flex: 1, padding: '10px 12px', borderRadius: 10, border: '1px solid var(--border)', background: 'var(--bg-secondary)', color: 'var(--text-primary)', fontSize: 14, resize: 'none', fontFamily: 'inherit', opacity: loading ? 0.6 : 1 }} />
        {loading ? (
          <button onClick={stop} style={{ padding: '10px 16px', borderRadius: 10, border: 'none', background: 'rgba(239,68,68,0.8)', color: '#fff', fontWeight: 600, fontSize: 13, cursor: 'pointer', alignSelf: 'flex-end' }}>
            停止
          </button>
        ) : (
          <button onClick={send} disabled={!input.trim() || !slug || !canSend}
            style={{ padding: '10px 20px', borderRadius: 10, border: 'none', background: 'var(--accent)', color: '#fff', fontWeight: 600, fontSize: 14, cursor: 'pointer', opacity: (!input.trim() || !slug || !canSend) ? 0.5 : 1, alignSelf: 'flex-end' }}>
            发送
          </button>
        )}
      </div>
    </div>
  )
}
