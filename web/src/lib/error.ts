type ErrorRecord = {
  message?: unknown
  response?: {
    status?: unknown
    data?: {
      message?: unknown
    }
  }
}

export function getErrorMessage(err: unknown, fallback = 'Request failed'): string {
  if (!err || typeof err !== 'object') return fallback
  const e = err as ErrorRecord
  const responseMessage = e.response?.data?.message
  if (typeof responseMessage === 'string' && responseMessage) return responseMessage
  if (typeof e.message === 'string' && e.message) return e.message
  return fallback
}

export function getResponseStatus(err: unknown): number | undefined {
  if (!err || typeof err !== 'object') return undefined
  const status = (err as ErrorRecord).response?.status
  return typeof status === 'number' ? status : undefined
}
