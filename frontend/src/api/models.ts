/**
 * Models API endpoints
 * Handles fetching available models for the user
 */

import { apiClient } from './client'

export interface Model {
  id: string
  type: string
  display_name: string
  created_at: string
}

export interface ModelsResponse {
  object: string
  data: Model[]
}

/**
 * Get available models using an API key
 * @param apiKey - The API key to use for authentication
 * @returns List of available models
 */
export async function getAvailableModels(apiKey: string): Promise<ModelsResponse> {
  const { data } = await apiClient.get<ModelsResponse>('/v1/models', {
    headers: {
      'x-api-key': apiKey
    }
  })
  return data
}

export const modelsAPI = {
  getAvailableModels
}

export default modelsAPI
