import React, { useState, useRef, useEffect } from 'react'
import { modelListProviders, modelChatStream, type ModelProvider } from '../../lib/api'

interface Message {
  role: 'user' | 'assistant' | 'system'
  content: string
  reasoning?: string
  streaming?: boolean
}

export default function ChatDebugPage() {
  const [providers, setProviders] = useState<ModelProvider[]>([])
  const [slug, setSlug] = useState('')
  const [systemPrompt, setSystemPrompt] = useState('')
  const [input, setInput] = useState('')
  const [messages, setMessages] = useState<Message[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [expandedReasoning, setExpandedReasoning] = useState<Set<number>>(new Set())
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

  const send = async () => {
    if (!input.trim() || !slug || loading) return

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
      }, abortRef.current.signal)

      // 确保 streaming 标记清除
      setMessages((prev) => {
        const next = [...prev]
        if (next[assistantIdx]?.streaming) {
          next[assistantIdx] = { ...next[assistantIdx], streaming: false }
        }
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
      setError(
        msg.includes('timeout') || msg.includes('ECONNABORTED') ? '请求超时，请重试' : msg || '请求失败'
      )
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
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', maxWidth: 760, margin: '0 auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16, flexShrink: 0 }}>
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
      <div style={{ marginBottom: 10, flexShrink: 0 }}>
        <textarea value={systemPrompt} onChange={(e) => setSystemPrompt(e.target.value)}
          placeholder="System prompt（可选）" rows={2}
          style={{ width: '100%', boxSizing: 'border-box', padding: '8px 12px', borderRadius: 8, border: '1px solid var(--border)', background: 'var(--bg-secondary)', color: 'var(--text-secondary)', fontSize: 12, resize: 'vertical', fontFamily: 'inherit' }} />
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

              {/* Thinking 折叠区 */}
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
          <button onClick={send} disabled={!input.trim() || !slug}
            style={{ padding: '10px 20px', borderRadius: 10, border: 'none', background: 'var(--accent)', color: '#fff', fontWeight: 600, fontSize: 14, cursor: 'pointer', opacity: (!input.trim() || !slug) ? 0.5 : 1, alignSelf: 'flex-end' }}>
            发送
          </button>
        )}
      </div>
    </div>
  )
}
