import { flushPromises, mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";
import RegisterView from "@/views/auth/RegisterView.vue";

const {
  pushMock,
  getPublicSettingsMock,
  createOAuthAccountMock,
  persistOAuthTokenPairMock,
  validateInvitationCodeMock,
  showErrorMock,
  showSuccessMock,
  clearPendingAuthSessionMock,
  setTokenMock,
} = vi.hoisted(() => ({
  pushMock: vi.fn(),
  getPublicSettingsMock: vi.fn(),
  createOAuthAccountMock: vi.fn(),
  persistOAuthTokenPairMock: vi.fn(),
  validateInvitationCodeMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
  clearPendingAuthSessionMock: vi.fn(),
  setTokenMock: vi.fn(),
}));

const authStore = {
  pendingAuthSession: {
    token: "pending-token",
    provider: "linuxdo",
    intent: "login",
    auth_result: "pending_session",
    redirect: "/welcome",
  },
  setToken: setTokenMock,
  register: vi.fn(),
  clearPendingAuthSession: clearPendingAuthSessionMock,
};

vi.mock("vue-router", () => ({
  useRouter: () => ({
    push: pushMock,
  }),
  useRoute: () => ({
    query: {},
  }),
}));

vi.mock("vue-i18n", async (importOriginal) => {
  const actual = await importOriginal<typeof import("vue-i18n")>();
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      locale: { value: "en" },
    }),
  };
});

vi.mock("@/stores", () => ({
  useAuthStore: () => authStore,
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: showSuccessMock,
  }),
}));

vi.mock("@/api/auth", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api/auth")>();
  return {
    ...actual,
    createOAuthAccount: createOAuthAccountMock,
    getPublicSettings: getPublicSettingsMock,
    persistOAuthTokenPair: persistOAuthTokenPairMock,
    validatePromoCode: vi.fn(),
    validateInvitationCode: validateInvitationCodeMock,
  };
});

vi.mock("@/utils/authError", () => ({
  buildAuthErrorMessage: () => "registration failed",
}));

vi.mock("@/utils/registrationEmailPolicy", () => ({
  isRegistrationEmailSuffixAllowed: () => true,
  normalizeRegistrationEmailSuffixWhitelist: (value: string[]) => value,
}));

function mountView() {
  return mount(RegisterView, {
    global: {
      stubs: {
        AuthLayout: {
          template: '<div><slot /><slot name="footer" /></div>',
        },
        LinuxDoOAuthSection: true,
        OidcOAuthSection: true,
        WechatOAuthSection: true,
        Icon: true,
        TurnstileWidget: true,
        RouterLink: {
          template: "<a><slot /></a>",
        },
      },
    },
  });
}

describe("RegisterView pending auth account creation", () => {
  beforeEach(() => {
    pushMock.mockReset();
    getPublicSettingsMock.mockReset();
    createOAuthAccountMock.mockReset();
    persistOAuthTokenPairMock.mockReset();
    validateInvitationCodeMock.mockReset();
    showErrorMock.mockReset();
    showSuccessMock.mockReset();
    clearPendingAuthSessionMock.mockReset();
    setTokenMock.mockReset();

    getPublicSettingsMock.mockResolvedValue({
      registration_enabled: true,
      email_verify_enabled: false,
      third_party_first_login_require_email: false,
      promo_code_enabled: false,
      invitation_code_enabled: false,
      turnstile_enabled: false,
      turnstile_site_key: "",
      site_name: "Sub2API",
      linuxdo_oauth_enabled: false,
      oidc_oauth_enabled: false,
      oidc_oauth_provider_name: "OIDC",
      registration_email_suffix_whitelist: [],
    });
    createOAuthAccountMock.mockResolvedValue({
      access_token: "access-token",
      refresh_token: "refresh-token",
      expires_in: 3600,
      token_type: "Bearer",
    });
    validateInvitationCodeMock.mockResolvedValue({ valid: true });
    setTokenMock.mockResolvedValue(undefined);
  });

  it("shows a pending-auth notice before creating the local account", async () => {
    const wrapper = mountView();
    await flushPromises();

    expect(wrapper.text()).toContain("auth.pendingAuth.register.title");
    expect(wrapper.text()).toContain("auth.pendingAuth.register.description");
  });

  it("clears the pending auth session after a create-account success", async () => {
    getPublicSettingsMock.mockResolvedValue({
      registration_enabled: true,
      email_verify_enabled: false,
      third_party_first_login_require_email: true,
      promo_code_enabled: false,
      invitation_code_enabled: false,
      turnstile_enabled: false,
      turnstile_site_key: "",
      site_name: "Sub2API",
      linuxdo_oauth_enabled: false,
      oidc_oauth_enabled: false,
      oidc_oauth_provider_name: "OIDC",
      registration_email_suffix_whitelist: [],
    });

    const wrapper = mountView();
    await flushPromises();

    await wrapper.get("#email").setValue("user@example.com");
    await wrapper.get("#password").setValue("secret123");
    await wrapper.get("form").trigger("submit.prevent");
    await flushPromises();

    expect(createOAuthAccountMock).toHaveBeenCalledWith("linuxdo", {
      pendingAuthToken: "pending-token",
      email: "user@example.com",
      password: "secret123",
      verifyCode: undefined,
      invitationCode: undefined,
    });
    expect(clearPendingAuthSessionMock).toHaveBeenCalled();
    expect(setTokenMock).toHaveBeenCalledWith("access-token");
    expect(pushMock).toHaveBeenCalledWith("/welcome");
  });

  it("allows invitation-only pending auth account creation without email and password", async () => {
    getPublicSettingsMock.mockResolvedValue({
      registration_enabled: true,
      email_verify_enabled: false,
      third_party_first_login_require_email: false,
      promo_code_enabled: false,
      invitation_code_enabled: true,
      turnstile_enabled: false,
      turnstile_site_key: "",
      site_name: "Sub2API",
      linuxdo_oauth_enabled: false,
      oidc_oauth_enabled: false,
      oidc_oauth_provider_name: "OIDC",
      registration_email_suffix_whitelist: [],
    });

    const wrapper = mountView();
    await flushPromises();

    await wrapper.get("#invitation_code").setValue("INVITE-001");
    await wrapper.get("form").trigger("submit.prevent");
    await flushPromises();

    expect(createOAuthAccountMock).toHaveBeenCalledWith("linuxdo", {
      pendingAuthToken: "pending-token",
      email: undefined,
      password: undefined,
      verifyCode: undefined,
      invitationCode: "INVITE-001",
    });
    expect(clearPendingAuthSessionMock).toHaveBeenCalled();
    expect(setTokenMock).toHaveBeenCalledWith("access-token");
    expect(pushMock).toHaveBeenCalledWith("/welcome");
  });
});
