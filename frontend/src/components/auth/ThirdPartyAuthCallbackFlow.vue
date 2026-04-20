<template>
  <div
    class="mx-auto w-full max-w-2xl rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-600 dark:bg-dark-800"
  >
    <div class="space-y-2">
      <p
        class="text-sm font-medium uppercase tracking-wide text-gray-500 dark:text-dark-300"
      >
        {{ providerHeading }}
      </p>
      <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
        {{ stateTitle }}
      </h1>
      <p class="text-sm text-gray-600 dark:text-dark-300">
        {{ stateDescription }}
      </p>
    </div>

    <div
      v-if="errorMessage"
      class="mt-6 rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800/50 dark:bg-red-900/20 dark:text-red-300"
    >
      {{ errorMessage }}
    </div>

    <div v-else-if="pendingSession" class="mt-6 space-y-4">
      <div
        class="rounded-xl border border-gray-200 bg-gray-50 p-4 text-sm text-gray-700 dark:border-dark-600 dark:bg-dark-700 dark:text-dark-200"
      >
        <p class="font-medium text-gray-900 dark:text-white">
          {{ pendingTitle }}
        </p>
        <p class="mt-1">{{ pendingDescription }}</p>
      </div>

      <div
        v-if="pendingSession.intent === 'login'"
        class="flex flex-col gap-3 sm:flex-row"
      >
        <button
          type="button"
          class="btn btn-primary flex-1"
          data-testid="create-account-action"
          @click="emitCreateAccount"
        >
          {{
            t("auth.thirdParty.callback.pending.login.actions.createAccount")
          }}
        </button>
        <button
          type="button"
          class="btn btn-secondary flex-1"
          data-testid="adopt-existing-user-action"
          @click="emitAdoptExistingUser"
        >
          {{ t("auth.thirdParty.callback.pending.login.actions.bindExisting") }}
        </button>
      </div>

      <button
        v-else-if="pendingSession.intent === 'bind_current_user'"
        type="button"
        class="btn btn-primary w-full"
        data-testid="bind-current-user-action"
        @click="emitBindCurrentUser"
      >
        {{ t("auth.thirdParty.callback.pending.bindCurrent.actions.continue") }}
      </button>

      <button
        v-else
        type="button"
        class="btn btn-primary w-full"
        data-testid="adopt-existing-user-action"
        @click="emitAdoptExistingUser"
      >
        {{
          t(
            "auth.thirdParty.callback.pending.adoptExisting.actions.verifyAndBind",
          )
        }}
      </button>
    </div>

    <div v-else-if="successPayload" class="mt-6 space-y-4">
      <div
        class="rounded-xl border border-green-200 bg-green-50 p-4 text-sm text-green-800 dark:border-green-800/50 dark:bg-green-900/20 dark:text-green-300"
      >
        <p class="font-medium text-green-900 dark:text-green-200">
          {{ t("auth.thirdParty.callback.success.summaryTitle") }}
        </p>
        <p class="mt-1">
          {{ successMessage }}
        </p>
      </div>

      <dl class="grid gap-3 sm:grid-cols-2">
        <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
          <dt
            class="text-xs uppercase tracking-wide text-gray-500 dark:text-dark-300"
          >
            {{ t("auth.thirdParty.callback.success.redirectLabel") }}
          </dt>
          <dd class="mt-1 text-sm text-gray-900 dark:text-white">
            {{ successPayload.redirect || "/dashboard" }}
          </dd>
        </div>
        <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
          <dt
            class="text-xs uppercase tracking-wide text-gray-500 dark:text-dark-300"
          >
            {{ t("auth.thirdParty.callback.success.tokenTypeLabel") }}
          </dt>
          <dd class="mt-1 text-sm text-gray-900 dark:text-white">
            {{ successPayload.tokenType || "Bearer" }}
          </dd>
        </div>
      </dl>
    </div>

    <div v-else-if="totpChallenge" class="mt-6 space-y-4">
      <div
        class="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-800/50 dark:bg-amber-900/20 dark:text-amber-300"
      >
        <p class="font-medium text-amber-900 dark:text-amber-200">
          {{ t("auth.thirdParty.callback.success.title") }}
        </p>
        <p class="mt-1">
          {{ t("profile.totp.verifyCodeDesc") }}
        </p>
      </div>
    </div>

    <div
      v-else
      class="mt-6 rounded-xl border border-gray-200 bg-gray-50 p-4 text-sm text-gray-700 dark:border-dark-600 dark:bg-dark-700 dark:text-dark-200"
    >
      {{ t("auth.thirdParty.callback.idle") }}
    </div>

    <IdentityAdoptionDialog
      v-if="adoptionState.open"
      :provider="resolvedProvider"
      :display-name="adoptionState.displayName"
      :avatar-url="adoptionState.avatarUrl"
      @confirm="submitAdoptionDecision"
      @skip="skipAdoptionDecision"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import IdentityAdoptionDialog from "./IdentityAdoptionDialog.vue";

type ThirdPartyAuthProvider = "linuxdo" | "wechat" | "oidc";
type PendingIntent =
  | "login"
  | "bind_current_user"
  | "adopt_existing_user_by_email";

interface PendingAuthSessionSummary {
  authResult: "pending_session";
  pendingAuthToken: string;
  provider: ThirdPartyAuthProvider;
  intent: PendingIntent;
  redirect: string;
  adoptionRequired: boolean;
  suggestedDisplayName: string | null;
  suggestedAvatarUrl: string | null;
}

interface AuthSuccessPayload {
  accessToken: string;
  refreshToken: string | null;
  expiresIn: number | null;
  tokenType: string | null;
  provider: ThirdPartyAuthProvider | null;
  intent: PendingIntent | null;
  redirect: string;
  adoptionRequired: boolean;
  suggestedDisplayName: string | null;
  suggestedAvatarUrl: string | null;
}

interface TotpRequiredPayload {
  requires2FA: true;
  tempToken: string;
  userEmailMasked: string | null;
  provider: ThirdPartyAuthProvider | null;
  intent: PendingIntent | null;
  redirect: string;
  pendingAuthToken?: string | null;
}

type ResolvedCallbackState =
  | { kind: "idle" }
  | { kind: "error"; message: string }
  | { kind: "pending"; summary: PendingAuthSessionSummary }
  | { kind: "totp"; payload: TotpRequiredPayload }
  | { kind: "success"; payload: AuthSuccessPayload };

const props = defineProps<{
  hash?: string;
  provider?: ThirdPartyAuthProvider;
  providerLabel?: string;
}>();
const { t } = useI18n();

const emit = defineEmits<{
  success: [payload: AuthSuccessPayload];
  error: [message: string];
  "pending-session": [summary: PendingAuthSessionSummary];
  "create-account": [summary: PendingAuthSessionSummary];
  "bind-current-user": [summary: PendingAuthSessionSummary];
  "adopt-existing-user": [summary: PendingAuthSessionSummary];
  "totp-required": [payload: TotpRequiredPayload];
  "adoption-decision": [
    payload: {
      adoptDisplayName: boolean;
      adoptAvatar: boolean;
      context: AuthSuccessPayload | PendingAuthSessionSummary;
    },
  ];
}>();

const adoptionState = ref<{
  open: boolean;
  displayName: string | null;
  avatarUrl: string | null;
}>({
  open: false,
  displayName: null,
  avatarUrl: null,
});

const rawHash = computed(() => {
  if (typeof props.hash === "string") return props.hash;
  if (typeof window === "undefined") return "";
  return window.location.hash || "";
});

const rawSearch = computed(() => {
  if (typeof window === "undefined") return "";
  return window.location.search || "";
});

const resolved = computed<ResolvedCallbackState>(() =>
  parseCallbackHash(rawHash.value, rawSearch.value, props.provider ?? null),
);

const errorMessage = computed(() =>
  resolved.value.kind === "error" ? resolved.value.message : "",
);
const pendingSession = computed(() =>
  resolved.value.kind === "pending" ? resolved.value.summary : null,
);
const successPayload = computed(() =>
  resolved.value.kind === "success" ? resolved.value.payload : null,
);
const totpChallenge = computed(() =>
  resolved.value.kind === "totp" ? resolved.value.payload : null,
);

const resolvedProvider = computed(() => {
  if (pendingSession.value) return pendingSession.value.provider;
  if (totpChallenge.value?.provider) return totpChallenge.value.provider;
  if (successPayload.value?.provider) return successPayload.value.provider;
  return "third_party";
});

const providerHeading = computed(
  () => resolveProviderHeading(resolvedProvider.value, props.provider ?? null, props.providerLabel),
);

const stateTitle = computed(() => {
  if (errorMessage.value) return t("auth.thirdParty.callback.error.title");
  if (pendingSession.value?.intent === "bind_current_user") {
    return t("auth.thirdParty.callback.pending.bindCurrent.title");
  }
  if (pendingSession.value?.intent === "adopt_existing_user_by_email") {
    return t("auth.thirdParty.callback.pending.adoptExisting.title");
  }
  if (pendingSession.value?.intent === "login") {
    return t("auth.thirdParty.callback.pending.login.title");
  }
  if (totpChallenge.value) return t("profile.totp.verifyCode");
  if (successPayload.value) return t("auth.thirdParty.callback.success.title");
  return t("auth.thirdParty.callback.idle");
});

const stateDescription = computed(() => {
  if (errorMessage.value) {
    return t("auth.thirdParty.callback.error.description");
  }
  if (pendingSession.value?.intent === "bind_current_user") {
    return t("auth.thirdParty.callback.pending.bindCurrent.description");
  }
  if (pendingSession.value?.intent === "adopt_existing_user_by_email") {
    return t("auth.thirdParty.callback.pending.adoptExisting.description");
  }
  if (pendingSession.value?.intent === "login") {
    return t("auth.thirdParty.callback.pending.login.description");
  }
  if (totpChallenge.value) {
    return t("profile.totp.verifyCodeDesc");
  }
  if (successPayload.value) {
    return t("auth.thirdParty.callback.success.description");
  }
  return t("auth.thirdParty.callback.idleDescription");
});

const pendingTitle = computed(() => {
  if (pendingSession.value?.intent === "bind_current_user") {
    return t("auth.thirdParty.callback.pending.bindCurrent.summaryTitle");
  }
  if (pendingSession.value?.intent === "adopt_existing_user_by_email") {
    return t("auth.thirdParty.callback.pending.adoptExisting.summaryTitle");
  }
  return t("auth.thirdParty.callback.pending.login.summaryTitle");
});

const pendingDescription = computed(() => {
  if (!pendingSession.value) return "";
  if (pendingSession.value.intent === "bind_current_user") {
    return t("auth.thirdParty.callback.pending.bindCurrent.summaryDescription");
  }
  if (pendingSession.value.intent === "adopt_existing_user_by_email") {
    return t(
      "auth.thirdParty.callback.pending.adoptExisting.summaryDescription",
    );
  }
  return t("auth.thirdParty.callback.pending.login.summaryDescription");
});

const successMessage = computed(() => {
  if (!successPayload.value) return "";
  if (successPayload.value.adoptionRequired) {
    return t("auth.thirdParty.callback.success.adoptionRequired");
  }
  return t("auth.thirdParty.callback.success.noAdoptionRequired");
});

watch(
  resolved,
  (state) => {
    adoptionState.value.open = false;
    adoptionState.value.displayName = null;
    adoptionState.value.avatarUrl = null;

    if (state.kind === "error") {
      emit("error", state.message);
      return;
    }

    if (state.kind === "pending") {
      emit("pending-session", state.summary);
      if (state.summary.adoptionRequired) {
        openAdoptionDialog(state.summary);
      }
      return;
    }

    if (state.kind === "success") {
      emit("success", state.payload);
      if (state.payload.adoptionRequired) {
        openAdoptionDialog(state.payload);
      }
      return;
    }

    if (state.kind === "totp") {
      emit("totp-required", state.payload);
    }
  },
  { immediate: true },
);

function emitCreateAccount() {
  if (!pendingSession.value) return;
  emit("create-account", pendingSession.value);
}

function emitBindCurrentUser() {
  if (!pendingSession.value) return;
  emit("bind-current-user", pendingSession.value);
}

function emitAdoptExistingUser() {
  if (!pendingSession.value) return;
  emit("adopt-existing-user", pendingSession.value);
}

function submitAdoptionDecision(decision: {
  adoptDisplayName: boolean;
  adoptAvatar: boolean;
}) {
  const context = successPayload.value ?? pendingSession.value;
  if (!context) return;

  adoptionState.value.open = false;
  emit("adoption-decision", {
    ...decision,
    context,
  });
}

function skipAdoptionDecision() {
  submitAdoptionDecision({
    adoptDisplayName: false,
    adoptAvatar: false,
  });
}

function openAdoptionDialog(
  context: AuthSuccessPayload | PendingAuthSessionSummary,
) {
  adoptionState.value.open = true;
  adoptionState.value.displayName = context.suggestedDisplayName;
  adoptionState.value.avatarUrl = context.suggestedAvatarUrl;
}

function parseCallbackHash(
  rawHash: string,
  rawSearch: string,
  fallbackProvider: ThirdPartyAuthProvider | null,
): ResolvedCallbackState {
  const params = parseCallbackParams(rawHash, rawSearch);
  if (!hasAnyCallbackParam(params)) return { kind: "idle" };

  const provider = parseProvider(params.get("provider")) ?? fallbackProvider;
  const redirect = sanitizeRedirectPath(params.get("redirect"));
  const explicitError = params.get("error");
  const explicitErrorDescription =
    params.get("error_description") || params.get("error_message") || null;

  if (explicitError) {
    const legacyPending = parseLegacyPendingState(params, provider, redirect);
    if (legacyPending) {
      return {
        kind: "pending",
        summary: legacyPending,
      };
    }

    return {
      kind: "error",
      message: explicitErrorDescription || explicitError,
    };
  }

  const intent = parseIntent(params.get("intent"));
  const adoptionRequired = parseBooleanFlag(params.get("adoption_required"));
  const suggestedDisplayName = params.get("suggested_display_name");
  const suggestedAvatarUrl = params.get("suggested_avatar_url");

  if (params.get("auth_result") === "pending_session") {
    const pendingAuthToken = params.get("pending_auth_token");
    if (!pendingAuthToken || !provider || !intent) {
      return {
        kind: "error",
        message: t("auth.thirdParty.callback.error.invalidPendingPayload"),
      };
    }

    return {
      kind: "pending",
      summary: {
        authResult: "pending_session",
        pendingAuthToken,
        provider,
        intent,
        redirect,
        adoptionRequired,
        suggestedDisplayName,
        suggestedAvatarUrl,
      },
    };
  }

  if (parseBooleanFlag(params.get("requires_2fa"))) {
    const tempToken = params.get("temp_token");
    if (!tempToken) {
      return {
        kind: "error",
        message: t("auth.thirdParty.callback.error.missingResult"),
      };
    }

    return {
      kind: "totp",
      payload: {
        requires2FA: true,
        tempToken,
        userEmailMasked: params.get("user_email_masked"),
        provider,
        intent,
        redirect,
        pendingAuthToken: params.get("pending_auth_token"),
      },
    };
  }

  const accessToken = params.get("access_token");
  if (!accessToken) {
    return {
      kind: "error",
      message: t("auth.thirdParty.callback.error.missingResult"),
    };
  }

  const expiresInRaw = params.get("expires_in");
  const expiresIn =
    expiresInRaw && Number.isFinite(Number(expiresInRaw))
      ? Number(expiresInRaw)
      : null;

  return {
    kind: "success",
    payload: {
      accessToken,
      refreshToken: params.get("refresh_token"),
      expiresIn,
      tokenType: params.get("token_type"),
      provider,
      intent,
      redirect,
      adoptionRequired,
      suggestedDisplayName,
      suggestedAvatarUrl,
    },
  };
}

function parseProvider(value: string | null): ThirdPartyAuthProvider | null {
  if (value === "linuxdo" || value === "wechat" || value === "oidc") {
    return value;
  }
  return null;
}

function parseIntent(value: string | null): PendingIntent | null {
  if (
    value === "login" ||
    value === "bind_current_user" ||
    value === "adopt_existing_user_by_email"
  ) {
    return value;
  }
  return null;
}

function parseBooleanFlag(value: string | null): boolean {
  return value === "1" || value === "true" || value === "yes";
}

function sanitizeRedirectPath(path: string | null | undefined): string {
  if (!path) return "/dashboard";
  if (!path.startsWith("/")) return "/dashboard";
  if (path.startsWith("//")) return "/dashboard";
  if (path.includes("://")) return "/dashboard";
  if (path.includes("\n") || path.includes("\r")) return "/dashboard";
  return path;
}

function parseCallbackParams(rawHash: string, rawSearch: string): URLSearchParams {
  const params = new URLSearchParams();
  const normalizedSearch = rawSearch.startsWith("?") ? rawSearch.slice(1) : rawSearch;
  const normalizedHash = rawHash.startsWith("#") ? rawHash.slice(1) : rawHash;

  for (const [key, value] of new URLSearchParams(normalizedSearch).entries()) {
    params.set(key, value);
  }
  for (const [key, value] of new URLSearchParams(normalizedHash).entries()) {
    params.set(key, value);
  }

  return params;
}

function hasAnyCallbackParam(params: URLSearchParams): boolean {
  return Array.from(params.keys()).length > 0;
}

function parseLegacyPendingState(
  params: URLSearchParams,
  provider: ThirdPartyAuthProvider | null,
  redirect: string,
): PendingAuthSessionSummary | null {
  const error = (params.get("error") || "").trim();
  const pendingAuthToken = (
    params.get("pending_auth_token") ||
    params.get("pending_oauth_token") ||
    params.get("pending_login_token") ||
    ""
  ).trim();

  if (!provider || !pendingAuthToken) {
    return null;
  }

  if (
    error !== "unbound_oauth_account" &&
    error !== "account_binding_required" &&
    error !== "binding_required" &&
    error !== "oauth_account_not_bound" &&
    error !== "oauth_binding_required" &&
    error !== "oauth_not_bound" &&
    error !== "oauth_invitation_required" &&
    error !== "invitation_required"
  ) {
    return null;
  }

  const legacyBindingIntent =
    (params.get("intent") || "").trim() === "bind" || hasLegacyBindingIntent(redirect);
  const normalizedRedirect = legacyBindingIntent
    ? stripLegacyBindingIntent(redirect)
    : redirect;

  return {
    authResult: "pending_session",
    pendingAuthToken,
    provider,
    intent: legacyBindingIntent ? "bind_current_user" : "login",
    redirect: normalizedRedirect,
    adoptionRequired: false,
    suggestedDisplayName: null,
    suggestedAvatarUrl: null,
  };
}

function hasLegacyBindingIntent(path: string): boolean {
  try {
    const parsed = new URL(path, window.location.origin);
    return parsed.searchParams.get("oauth_intent") === "bind";
  } catch {
    return path.includes("oauth_intent=bind");
  }
}

function stripLegacyBindingIntent(path: string): string {
  try {
    const parsed = new URL(path, window.location.origin);
    parsed.searchParams.delete("oauth_intent");
    const query = parsed.searchParams.toString();
    return `${parsed.pathname}${query ? `?${query}` : ""}${parsed.hash}`;
  } catch {
    return path.replace(/([?&])oauth_intent=bind&?/g, "$1").replace(/[?&]$/, "");
  }
}

function resolveProviderHeading(
  resolvedProvider: string,
  fallbackProvider: ThirdPartyAuthProvider | null,
  fallbackLabel?: string,
): string {
  if (resolvedProvider === "oidc" && fallbackProvider === "oidc" && fallbackLabel) {
    return fallbackLabel;
  }

  if (resolvedProvider === "linuxdo") {
    return t("profile.bindings.providers.linuxdo");
  }
  if (resolvedProvider === "wechat") {
    return t("profile.bindings.providers.wechat");
  }
  if (resolvedProvider === "oidc") {
    return t("profile.bindings.providers.oidc");
  }

  if (fallbackLabel) {
    return fallbackLabel;
  }

  if (fallbackProvider === "linuxdo") {
    return t("profile.bindings.providers.linuxdo");
  }
  if (fallbackProvider === "wechat") {
    return t("profile.bindings.providers.wechat");
  }
  if (fallbackProvider === "oidc") {
    return t("profile.bindings.providers.oidc");
  }

  return formatProviderLabel(resolvedProvider);
}

function formatProviderLabel(provider: string): string {
  return provider
    .split("_")
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}
</script>
