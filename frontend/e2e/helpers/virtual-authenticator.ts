import type { CDPSession, Page } from '@playwright/test'

export interface VirtualCredential {
  credentialId: string
  isResidentCredential: boolean
  rpId: string
  signCount: number
  userHandle?: string
  userName?: string
  userDisplayName?: string
  backupEligibility?: boolean
  backupState?: boolean
}

export interface VirtualAuthenticatorController {
  authenticatorId: string
  listCredentials(): Promise<VirtualCredential[]>
  clearCredentials(): Promise<void>
  removeCredential(credentialId: string): Promise<void>
  setUserVerified(isUserVerified: boolean): Promise<void>
  setAutomaticPresenceSimulation(enabled: boolean): Promise<void>
  remove(): Promise<void>
}

export async function attachVirtualAuthenticator(page: Page): Promise<VirtualAuthenticatorController> {
  const session = await page.context().newCDPSession(page)
  await session.send('WebAuthn.enable', { enableUI: false })

  const { authenticatorId } = await session.send('WebAuthn.addVirtualAuthenticator', {
    options: {
      protocol: 'ctap2',
      ctap2Version: 'ctap2_1',
      transport: 'internal',
      hasResidentKey: true,
      hasUserVerification: true,
      automaticPresenceSimulation: true,
      isUserVerified: true
    }
  }) as { authenticatorId: string }

  return createVirtualAuthenticatorController(session, authenticatorId)
}

function createVirtualAuthenticatorController(
  session: CDPSession,
  authenticatorId: string
): VirtualAuthenticatorController {
  return {
    authenticatorId,
    async listCredentials(): Promise<VirtualCredential[]> {
      const response = await session.send('WebAuthn.getCredentials', {
        authenticatorId
      }) as { credentials: VirtualCredential[] }

      return response.credentials
    },
    async clearCredentials(): Promise<void> {
      await session.send('WebAuthn.clearCredentials', { authenticatorId })
    },
    async removeCredential(credentialId: string): Promise<void> {
      await session.send('WebAuthn.removeCredential', {
        authenticatorId,
        credentialId
      })
    },
    async setUserVerified(isUserVerified: boolean): Promise<void> {
      await session.send('WebAuthn.setUserVerified', {
        authenticatorId,
        isUserVerified
      })
    },
    async setAutomaticPresenceSimulation(enabled: boolean): Promise<void> {
      await session.send('WebAuthn.setAutomaticPresenceSimulation', {
        authenticatorId,
        enabled
      })
    },
    async remove(): Promise<void> {
      await session.send('WebAuthn.removeVirtualAuthenticator', { authenticatorId })
    }
  }
}
