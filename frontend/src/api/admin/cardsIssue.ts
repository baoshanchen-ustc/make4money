import { apiClient } from '../client'

export interface CardsIssueAdminConfig {
  enabled: boolean
  response_template: string
  key_exists: boolean
  masked_key: string
}

export interface UpdateCardsIssueConfigRequest {
  enabled: boolean
  response_template: string
}

export async function getConfig(): Promise<CardsIssueAdminConfig> {
  const { data } = await apiClient.get<CardsIssueAdminConfig>('/admin/cards-issue/config')
  return data
}

export async function updateConfig(
  payload: UpdateCardsIssueConfigRequest,
): Promise<CardsIssueAdminConfig> {
  const { data } = await apiClient.put<CardsIssueAdminConfig>('/admin/cards-issue/config', payload)
  return data
}

export async function regenerateKey(): Promise<{ key: string }> {
  const { data } = await apiClient.post<{ key: string }>('/admin/cards-issue/key/regenerate')
  return data
}

export async function deleteKey(): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>('/admin/cards-issue/key')
  return data
}

export const cardsIssueAPI = {
  getConfig,
  updateConfig,
  regenerateKey,
  deleteKey,
}

export default cardsIssueAPI
