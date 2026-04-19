<template>
  <AuthLayout>
    <ThirdPartyAuthCallbackFlow
      :provider-label="providerLabel"
      @success="handleSuccess"
      @error="handleError"
      @pending-session="handlePendingSession"
      @totp-required="handleTotpRequired"
      @create-account="handleCreateAccount"
      @adopt-existing-user="handleAdoptExistingUser"
      @bind-current-user="handleBindCurrentUser"
      @adoption-decision="handleAdoptionDecision"
    />
    <TotpLoginModal
      :show="show2FAModal"
      :temp-token="totpTempToken"
      :user-email-masked="totpUserEmailMasked"
      @verify="handle2FAVerify"
      @close="handle2FACancel"
    />
  </AuthLayout>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";

import ThirdPartyAuthCallbackFlow from "@/components/auth/ThirdPartyAuthCallbackFlow.vue";
import TotpLoginModal from "@/components/auth/TotpLoginModal.vue";
import { AuthLayout } from "@/components/layout";
import {
  createOAuthAccount,
  getPendingAuthSessionAdoptionDecision,
  getPublicSettings,
  inheritPendingAuthSessionAdoptionDecision,
  persistOAuthTokenPair,
  sanitizeAuthRedirectPath,
  withPendingAuthSessionAdoptionDecision,
} from "@/api/auth";
import { userAPI } from "@/api/user";
import { useAuthStore, useAppStore } from "@/stores";
import type {
  OAuthProvider,
  PendingAuthIntent,
  PendingAuthSessionSummary,
} from "@/types";

interface CallbackPendingSession {
  authResult: "pending_session";
  pendingAuthToken: string;
  provider: OAuthProvider;
  intent: PendingAuthIntent;
  redirect: string;
  adoptionRequired: boolean;
  suggestedDisplayName: string | null;
  suggestedAvatarUrl: string | null;
}

interface CallbackSuccessPayload {
  accessToken: string;
  refreshToken: string | null;
  expiresIn: number | null;
  tokenType: string | null;
  provider: OAuthProvider | null;
  intent: PendingAuthIntent | null;
  redirect: string;
  adoptionRequired: boolean;
  suggestedDisplayName: string | null;
  suggestedAvatarUrl: string | null;
}

interface CallbackTotpPayload {
  requires2FA: true;
  tempToken: string;
  userEmailMasked: string | null;
  provider: OAuthProvider | null;
  intent: PendingAuthIntent | null;
  redirect: string;
  pendingAuthToken?: string | null;
}

interface AdoptionDecisionPayload {
  adoptDisplayName: boolean;
  adoptAvatar: boolean;
  context: CallbackSuccessPayload | CallbackPendingSession;
}

const router = useRouter();
const { t } = useI18n();

const authStore = useAuthStore();
const appStore = useAppStore();

const isHandlingAction = ref(false);
const deferredSuccessPayload = ref<CallbackSuccessPayload | null>(null);
const show2FAModal = ref(false);
const totpTempToken = ref("");
const totpUserEmailMasked = ref("");
const pendingAuthTokenFor2FA = ref("");
const totpRedirectPath = ref("/dashboard");
const providerName = ref("OIDC");
const providerLabel = computed(
  () => providerName.value || t("profile.bindings.providers.oidc"),
);

function normalizePendingSession(
  summary: CallbackPendingSession,
): PendingAuthSessionSummary {
  return {
    token: summary.pendingAuthToken,
    provider: summary.provider,
    intent: summary.intent,
    auth_result: "pending_session",
    redirect: sanitizeAuthRedirectPath(summary.redirect),
    adoption_required: summary.adoptionRequired,
    suggested_display_name: summary.suggestedDisplayName,
    suggested_avatar_url: summary.suggestedAvatarUrl,
  };
}

function persistPendingSession(
  summary: CallbackPendingSession,
): PendingAuthSessionSummary {
  const normalized = normalizePendingSession(summary);
  const existing = authStore.pendingAuthSession;

  if (
    existing &&
    existing.token === normalized.token &&
    existing.provider === normalized.provider
  ) {
    return inheritPendingAuthSessionAdoptionDecision(normalized, existing);
  }

  return normalized;
}

function resolveErrorMessage(error: unknown, fallback: string): string {
  const err = error as {
    message?: string;
    response?: {
      data?: {
        message?: string;
        detail?: string;
        error?: string;
      };
    };
  };

  return (
    err.response?.data?.message ||
    err.response?.data?.detail ||
    err.response?.data?.error ||
    err.message ||
    fallback
  );
}

async function loadProviderName() {
  try {
    const settings = await getPublicSettings();
    const name = settings.oidc_oauth_provider_name?.trim();
    if (name) {
      providerName.value = name;
    }
  } catch {
    // Ignore and keep the default label.
  }
}

async function routeToAuthEntry(name: "Login" | "Register", redirect: string) {
  const sanitized = sanitizeAuthRedirectPath(redirect);
  if (sanitized === "/dashboard") {
    await router.replace({ name });
    return;
  }

  await router.replace({
    name,
    query: { redirect: sanitized },
  });
}

async function finalizeSuccess(payload: CallbackSuccessPayload) {
  if (isHandlingAction.value) return;

  isHandlingAction.value = true;
  deferredSuccessPayload.value = null;

  try {
    persistOAuthTokenPair({
      access_token: payload.accessToken,
      refresh_token: payload.refreshToken ?? undefined,
      expires_in: payload.expiresIn ?? undefined,
      token_type: payload.tokenType ?? "Bearer",
    });

    authStore.clearPendingAuthSession();
    await authStore.setToken(payload.accessToken);
    appStore.showSuccess(t("auth.loginSuccess"));
    await router.replace(sanitizeAuthRedirectPath(payload.redirect));
  } catch (error: unknown) {
    appStore.showError(resolveErrorMessage(error, t("auth.loginFailed")));
  } finally {
    isHandlingAction.value = false;
  }
}

function handleError(message: string) {
  appStore.showError(message);
}

function handlePendingSession(summary: CallbackPendingSession) {
  authStore.setPendingAuthSession(persistPendingSession(summary));
}

function handleTotpRequired(payload: CallbackTotpPayload) {
  totpTempToken.value = payload.tempToken;
  totpUserEmailMasked.value = payload.userEmailMasked || "";
  pendingAuthTokenFor2FA.value = payload.pendingAuthToken || "";
  totpRedirectPath.value = sanitizeAuthRedirectPath(payload.redirect);
  show2FAModal.value = true;
}

async function ensurePublicSettingsLoaded() {
  try {
    return await appStore.fetchPublicSettings();
  } catch {
    return appStore.cachedPublicSettings;
  }
}

async function handleCreateAccount(summary: CallbackPendingSession) {
  authStore.setPendingAuthSession(persistPendingSession(summary));
  const publicSettings = await ensurePublicSettingsLoaded();
  const requiresEmailBinding =
    publicSettings?.third_party_first_login_require_email === true;
  const invitationCodeEnabled =
    publicSettings?.invitation_code_enabled === true;

  if (!requiresEmailBinding && !invitationCodeEnabled) {
    if (isHandlingAction.value) return;

    isHandlingAction.value = true;
    try {
      const pendingSession = authStore.pendingAuthSession;
      const adoptionDecision =
        getPendingAuthSessionAdoptionDecision(pendingSession);
      const response = await createOAuthAccount(summary.provider, {
        pendingAuthToken: summary.pendingAuthToken,
        adoptDisplayName: adoptionDecision?.adoptDisplayName,
        adoptAvatar: adoptionDecision?.adoptAvatar,
      });
      persistOAuthTokenPair(response);
      authStore.clearPendingAuthSession();
      await authStore.setToken(response.access_token);
      appStore.showSuccess(
        t("auth.accountCreatedSuccess", {
          siteName: appStore.siteName || "Sub2API",
        }),
      );
      await router.replace(sanitizeAuthRedirectPath(summary.redirect));
      return;
    } catch (error: unknown) {
      appStore.showError(
        resolveErrorMessage(error, t("auth.oidc.completeRegistrationFailed")),
      );
      return;
    } finally {
      isHandlingAction.value = false;
    }
  }

  await routeToAuthEntry("Register", summary.redirect);
}

async function handleAdoptExistingUser(summary: CallbackPendingSession) {
  authStore.setPendingAuthSession(persistPendingSession(summary));
  await routeToAuthEntry("Login", summary.redirect);
}

async function handleBindCurrentUser(summary: CallbackPendingSession) {
  if (isHandlingAction.value) return;

  authStore.setPendingAuthSession(persistPendingSession(summary));

  if (!authStore.token) {
    appStore.showError(t("auth.reloginRequired"));
    await router.replace({
      name: "Login",
      query: { redirect: "/profile" },
    });
    return;
  }

  isHandlingAction.value = true;

  try {
    await userAPI.bindAccount(summary.provider, summary.pendingAuthToken);
    authStore.clearPendingAuthSession();
    await authStore.refreshUser();
    appStore.showSuccess(
      `${providerLabel.value} ${t("profile.bindings.actions.connected")}`,
    );

    const redirect = sanitizeAuthRedirectPath(summary.redirect);
    await router.replace(
      redirect.startsWith("/profile") ? "/profile" : redirect,
    );
  } catch (error: unknown) {
    appStore.showError(
      resolveErrorMessage(error, t("auth.oidc.completeRegistrationFailed")),
    );
  } finally {
    isHandlingAction.value = false;
  }
}

async function handleSuccess(payload: CallbackSuccessPayload) {
  if (payload.adoptionRequired) {
    deferredSuccessPayload.value = payload;
    return;
  }

  await finalizeSuccess(payload);
}

async function handleAdoptionDecision({
  adoptDisplayName,
  adoptAvatar,
  context,
}: AdoptionDecisionPayload) {
  if ("accessToken" in context) {
    await finalizeSuccess(deferredSuccessPayload.value ?? context);
    return;
  }

  if (context.intent === "bind_current_user" && authStore.token) {
    try {
      await userAPI.setAccountBindingAdoptionDecision(
        context.provider,
        context.pendingAuthToken,
        adoptDisplayName,
        adoptAvatar,
      );
    } catch (error: unknown) {
      appStore.showError(
        resolveErrorMessage(error, t("auth.oidc.completeRegistrationFailed")),
      );
      return;
    }

    await handleBindCurrentUser(context);
    return;
  }

  authStore.setPendingAuthSession(
    withPendingAuthSessionAdoptionDecision(persistPendingSession(context), {
      adoptDisplayName,
      adoptAvatar,
    }),
  );
}

async function handle2FAVerify(code: string) {
  try {
    await authStore.login2FA(
      totpTempToken.value,
      code,
      pendingAuthTokenFor2FA.value || undefined,
    );
    show2FAModal.value = false;
    appStore.showSuccess(t("auth.loginSuccess"));
    await router.replace(totpRedirectPath.value);
  } catch (error: unknown) {
    appStore.showError(
      resolveErrorMessage(error, t("profile.totp.loginFailed")),
    );
  }
}

function handle2FACancel() {
  show2FAModal.value = false;
  totpTempToken.value = "";
  totpUserEmailMasked.value = "";
  pendingAuthTokenFor2FA.value = "";
  totpRedirectPath.value = "/dashboard";
}

onMounted(() => {
  void loadProviderName();
});
</script>
