import { describe, expect, it } from 'vitest'

import { parseOpenAIRequestOverrides } from '@/utils/openaiRequestOverrides'

describe('parseOpenAIRequestOverrides', () => {
  it('rejects top-level model overrides', () => {
    expect(parseOpenAIRequestOverrides('{"model":"gpt-5.4"}')).toEqual({
      value: null,
      error: 'model_not_allowed'
    })
  })

  it('allows non-model overrides', () => {
    expect(parseOpenAIRequestOverrides('{"service_tier":"fast"}')).toEqual({
      value: {
        service_tier: 'fast'
      },
      error: null
    })
  })
})
