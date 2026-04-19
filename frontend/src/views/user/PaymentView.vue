<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-20">
        <div
          class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"
        ></div>
      </div>
      <template v-else>
        <!-- Tab Switcher (hide during payment and subscription confirm) -->
        <div
          v-if="tabs.length > 1 && paymentPhase === 'select' && !selectedPlan"
          class="flex space-x-1 rounded-xl bg-gray-100 p-1 dark:bg-dark-800"
        >
          <button
            v-for="tab in tabs"
            :key="tab.key"
            class="flex-1 rounded-lg px-4 py-2.5 text-sm font-medium transition-all"
            :class="
              activeTab === tab.key
                ? 'bg-white text-gray-900 shadow dark:bg-dark-700 dark:text-white'
                : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
            "
            @click="activeTab = tab.key"
          >
            {{ tab.label }}
          </button>
        </div>
        <!-- Payment in progress (shared by recharge and subscription) -->
        <template v-if="paymentPhase === 'paying'">
          <PaymentStatusPanel
            :order-id="paymentState.orderId"
            :qr-code="paymentState.qrCode"
            :expires-at="paymentState.expiresAt"
            :payment-type="paymentState.paymentType"
            :pay-url="paymentState.payUrl"
            :order-type="paymentState.orderType"
            :status-mode="paymentState.statusMode"
            @done="onPaymentDone"
            @success="onPaymentSuccess"
          />
        </template>
        <template v-else-if="paymentPhase === 'stripe'">
          <StripePaymentInline
            :order-id="paymentState.orderId"
            :amount="paymentState.amount"
            :client-secret="paymentState.clientSecret"
            :order-type="paymentState.orderType || undefined"
            :publishable-key="
              paymentState.publishableKey || checkout.stripe_publishable_key
            "
            :pay-amount="paymentState.payAmount"
            @success="onPaymentSuccess"
            @done="onStripeDone"
            @back="resetPayment"
            @redirect="onStripeRedirect"
          />
        </template>
        <!-- Tab content (select phase) -->
        <template v-else>
          <!-- Top-up Tab -->
          <template v-if="activeTab === 'recharge'">
            <!-- Recharge Account Card -->
            <div class="card p-5">
              <p class="text-xs font-medium text-gray-400 dark:text-gray-500">
                {{ t("payment.rechargeAccount") }}
              </p>
              <p
                class="mt-1 text-base font-semibold text-gray-900 dark:text-white"
              >
                {{ user?.username || "" }}
              </p>
              <p
                class="mt-0.5 text-sm font-medium text-green-600 dark:text-green-400"
              >
                {{ t("payment.currentBalance") }}:
                {{ user?.balance?.toFixed(2) || "0.00" }}
              </p>
            </div>
            <div
              v-if="methodOptions.length === 0"
              class="card py-16 text-center"
            >
              <p class="text-gray-500 dark:text-gray-400">
                {{ t("payment.notAvailable") }}
              </p>
            </div>
            <template v-else>
              <div class="card p-6">
                <AmountInput
                  v-model="amount"
                  :amounts="[10, 20, 50, 100, 200, 500, 1000, 2000, 5000]"
                  :min="globalMinAmount"
                  :max="globalMaxAmount"
                />
                <p
                  v-if="amountError"
                  class="mt-2 text-xs text-amber-600 dark:text-amber-300"
                >
                  {{ amountError }}
                </p>
              </div>
              <div v-if="methodOptions.length > 0" class="card p-6">
                <PaymentMethodSelector
                  :methods="methodOptions"
                  :selected="selectedMethod"
                  @select="selectedMethod = $event"
                />
              </div>
              <div v-if="validAmount > 0" class="card p-6">
                <div class="space-y-2 text-sm">
                  <div class="flex justify-between">
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t("payment.paymentAmount")
                    }}</span>
                    <span class="text-gray-900 dark:text-white"
                      >¥{{ validAmount.toFixed(2) }}</span
                    >
                  </div>
                  <div v-if="feeRate > 0" class="flex justify-between">
                    <span class="text-gray-500 dark:text-gray-400"
                      >{{ t("payment.fee") }} ({{ feeRate }}%)</span
                    >
                    <span class="text-gray-900 dark:text-white"
                      >¥{{ feeAmount.toFixed(2) }}</span
                    >
                  </div>
                  <div
                    v-if="feeRate > 0"
                    class="flex justify-between border-t border-gray-200 pt-2 dark:border-dark-600"
                  >
                    <span
                      class="font-medium text-gray-700 dark:text-gray-300"
                      >{{ t("payment.actualPay") }}</span
                    >
                    <span
                      class="text-lg font-bold text-primary-600 dark:text-primary-400"
                      >¥{{ totalAmount.toFixed(2) }}</span
                    >
                  </div>
                  <div
                    v-if="balanceRechargeMultiplier !== 1"
                    class="flex justify-between"
                    :class="{
                      'border-t border-gray-200 pt-2 dark:border-dark-600':
                        feeRate <= 0,
                    }"
                  >
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t("payment.creditedBalance")
                    }}</span>
                    <span class="text-gray-900 dark:text-white"
                      >${{ creditedAmount.toFixed(2) }}</span
                    >
                  </div>
                  <p
                    v-if="balanceRechargeMultiplier !== 1"
                    class="border-t border-gray-200 pt-2 text-xs text-gray-500 dark:border-dark-600 dark:text-gray-400"
                  >
                    {{
                      t("payment.rechargeRatePreview", {
                        usd: balanceRechargeMultiplier.toFixed(2),
                      })
                    }}
                  </p>
                </div>
              </div>
              <button
                :class="[
                  'btn w-full py-3 text-base font-medium',
                  paymentButtonClass,
                ]"
                :disabled="!canSubmit || submitting"
                @click="handleSubmitRecharge"
              >
                <span
                  v-if="submitting"
                  class="flex items-center justify-center gap-2"
                >
                  <span
                    class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"
                  ></span>
                  {{ t("common.processing") }}
                </span>
                <span v-else
                  >{{ t("payment.createOrder") }} ¥{{
                    totalAmount.toFixed(2)
                  }}</span
                >
              </button>
              <div
                v-if="errorMessage"
                class="rounded-xl border border-red-200 bg-red-50 p-4 dark:border-red-800/50 dark:bg-red-900/20"
              >
                <p class="text-sm text-red-700 dark:text-red-400">
                  {{ errorMessage }}
                </p>
              </div>
              <div
                v-if="showWechatRecoveryHint"
                class="rounded-2xl border border-amber-200 bg-amber-50/80 p-5 dark:border-amber-800/40 dark:bg-amber-950/20"
              >
                <div
                  class="flex flex-col gap-5 md:flex-row md:items-center md:justify-between"
                >
                  <div class="space-y-3">
                    <div>
                      <div
                        class="text-sm font-semibold text-amber-900 dark:text-amber-200"
                      >
                        {{ wechatRecoveryTitle }}
                      </div>
                      <p
                        class="mt-1 text-sm leading-6 text-amber-800/90 dark:text-amber-200/80"
                      >
                        {{ wechatRecoveryHintText }}
                      </p>
                    </div>
                    <div
                      class="text-xs text-amber-700/90 dark:text-amber-300/80"
                    >
                      {{ wechatRecoverySummary }}
                    </div>
                    <div class="flex flex-wrap gap-3">
                      <button
                        class="btn btn-secondary"
                        @click="copyWechatRecoveryUrl"
                      >
                        {{ wechatRecoveryCopyLabel }}
                      </button>
                    </div>
                  </div>
                  <div
                    class="w-full max-w-[220px] rounded-2xl bg-white p-4 shadow-sm dark:bg-dark-900"
                  >
                    <div
                      class="text-center text-xs font-medium text-gray-500 dark:text-gray-400"
                    >
                      {{ wechatRecoveryScanLabel }}
                    </div>
                    <div class="mt-3 flex justify-center">
                      <canvas
                        ref="wechatRecoveryCanvasRef"
                        class="h-44 w-44"
                      ></canvas>
                    </div>
                  </div>
                </div>
              </div>
            </template>
          </template>
          <!-- Subscribe Tab -->
          <template v-else-if="activeTab === 'subscription'">
            <!-- Subscription confirm (inline, replaces plan list) -->
            <template v-if="selectedPlan">
              <div class="card p-5">
                <!-- Header: platform badge + plan name -->
                <div class="mb-3 flex flex-wrap items-center gap-2">
                  <span
                    :class="[
                      'rounded-md border px-2 py-0.5 text-xs font-medium',
                      planBadgeClass,
                    ]"
                  >
                    {{ platformLabel(selectedPlan.group_platform || "") }}
                  </span>
                  <h3 class="text-lg font-bold text-gray-900 dark:text-white">
                    {{ selectedPlan.name }}
                  </h3>
                </div>
                <!-- Price -->
                <div class="flex items-baseline gap-2">
                  <span
                    v-if="selectedPlan.original_price"
                    class="text-sm text-gray-400 line-through dark:text-gray-500"
                  >
                    ¥{{ selectedPlan.original_price }}
                  </span>
                  <span :class="['text-3xl font-bold', planTextClass]"
                    >¥{{ selectedPlan.price }}</span
                  >
                  <span class="text-sm text-gray-500 dark:text-gray-400"
                    >/ {{ planValiditySuffix }}</span
                  >
                </div>
                <!-- Description -->
                <p
                  v-if="selectedPlan.description"
                  class="mt-2 text-sm leading-relaxed text-gray-500 dark:text-gray-400"
                >
                  {{ selectedPlan.description }}
                </p>
                <!-- Rate + Limits grid -->
                <div class="mt-3 grid grid-cols-2 gap-3">
                  <div>
                    <span class="text-xs text-gray-400 dark:text-gray-500">{{
                      t("payment.planCard.rate")
                    }}</span>
                    <div class="flex items-baseline">
                      <span :class="['text-lg font-bold', planTextClass]"
                        >×{{ selectedPlan.rate_multiplier ?? 1 }}</span
                      >
                    </div>
                  </div>
                  <div v-if="selectedPlan.daily_limit_usd != null">
                    <span class="text-xs text-gray-400 dark:text-gray-500">{{
                      t("payment.planCard.dailyLimit")
                    }}</span>
                    <div
                      class="text-lg font-semibold text-gray-800 dark:text-gray-200"
                    >
                      ${{ selectedPlan.daily_limit_usd }}
                    </div>
                  </div>
                  <div v-if="selectedPlan.weekly_limit_usd != null">
                    <span class="text-xs text-gray-400 dark:text-gray-500">{{
                      t("payment.planCard.weeklyLimit")
                    }}</span>
                    <div
                      class="text-lg font-semibold text-gray-800 dark:text-gray-200"
                    >
                      ${{ selectedPlan.weekly_limit_usd }}
                    </div>
                  </div>
                  <div v-if="selectedPlan.monthly_limit_usd != null">
                    <span class="text-xs text-gray-400 dark:text-gray-500">{{
                      t("payment.planCard.monthlyLimit")
                    }}</span>
                    <div
                      class="text-lg font-semibold text-gray-800 dark:text-gray-200"
                    >
                      ${{ selectedPlan.monthly_limit_usd }}
                    </div>
                  </div>
                  <div
                    v-if="
                      selectedPlan.daily_limit_usd == null &&
                      selectedPlan.weekly_limit_usd == null &&
                      selectedPlan.monthly_limit_usd == null
                    "
                  >
                    <span class="text-xs text-gray-400 dark:text-gray-500">{{
                      t("payment.planCard.quota")
                    }}</span>
                    <div
                      class="text-lg font-semibold text-gray-800 dark:text-gray-200"
                    >
                      {{ t("payment.planCard.unlimited") }}
                    </div>
                  </div>
                </div>
              </div>
              <div v-if="subMethodOptions.length > 0" class="card p-6">
                <PaymentMethodSelector
                  :methods="subMethodOptions"
                  :selected="selectedMethod"
                  @select="selectedMethod = $event"
                />
              </div>
              <div
                v-if="feeRate > 0 && selectedPlan.price > 0"
                class="card p-6"
              >
                <div class="space-y-2 text-sm">
                  <div class="flex justify-between">
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t("payment.amountLabel")
                    }}</span>
                    <span class="text-gray-900 dark:text-white"
                      >¥{{ selectedPlan.price.toFixed(2) }}</span
                    >
                  </div>
                  <div class="flex justify-between">
                    <span class="text-gray-500 dark:text-gray-400"
                      >{{ t("payment.fee") }} ({{ feeRate }}%)</span
                    >
                    <span class="text-gray-900 dark:text-white"
                      >¥{{ subFeeAmount.toFixed(2) }}</span
                    >
                  </div>
                  <div
                    class="flex justify-between border-t border-gray-200 pt-2 dark:border-dark-600"
                  >
                    <span
                      class="font-medium text-gray-700 dark:text-gray-300"
                      >{{ t("payment.actualPay") }}</span
                    >
                    <span
                      class="text-lg font-bold text-primary-600 dark:text-primary-400"
                      >¥{{ subTotalAmount.toFixed(2) }}</span
                    >
                  </div>
                </div>
              </div>
              <button
                :class="[
                  'btn w-full py-3 text-base font-medium',
                  paymentButtonClass,
                ]"
                :disabled="!canSubmitSubscription || submitting"
                @click="confirmSubscribe"
              >
                <span
                  v-if="submitting"
                  class="flex items-center justify-center gap-2"
                >
                  <span
                    class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"
                  ></span>
                  {{ t("common.processing") }}
                </span>
                <span v-else
                  >{{ t("payment.createOrder") }} ¥{{
                    (feeRate > 0 ? subTotalAmount : selectedPlan.price).toFixed(
                      2,
                    )
                  }}</span
                >
              </button>
              <button
                class="btn btn-secondary w-full"
                @click="selectedPlan = null"
              >
                {{ t("common.cancel") }}
              </button>
              <div
                v-if="errorMessage"
                class="rounded-xl border border-red-200 bg-red-50 p-4 dark:border-red-800/50 dark:bg-red-900/20"
              >
                <p class="text-sm text-red-700 dark:text-red-400">
                  {{ errorMessage }}
                </p>
              </div>
              <div
                v-if="showWechatRecoveryHint"
                class="rounded-2xl border border-amber-200 bg-amber-50/80 p-5 dark:border-amber-800/40 dark:bg-amber-950/20"
              >
                <div
                  class="flex flex-col gap-5 md:flex-row md:items-center md:justify-between"
                >
                  <div class="space-y-3">
                    <div>
                      <div
                        class="text-sm font-semibold text-amber-900 dark:text-amber-200"
                      >
                        {{ wechatRecoveryTitle }}
                      </div>
                      <p
                        class="mt-1 text-sm leading-6 text-amber-800/90 dark:text-amber-200/80"
                      >
                        {{ wechatRecoveryHintText }}
                      </p>
                    </div>
                    <div
                      class="text-xs text-amber-700/90 dark:text-amber-300/80"
                    >
                      {{ wechatRecoverySummary }}
                    </div>
                    <div class="flex flex-wrap gap-3">
                      <button
                        class="btn btn-secondary"
                        @click="copyWechatRecoveryUrl"
                      >
                        {{ wechatRecoveryCopyLabel }}
                      </button>
                    </div>
                  </div>
                  <div
                    class="w-full max-w-[220px] rounded-2xl bg-white p-4 shadow-sm dark:bg-dark-900"
                  >
                    <div
                      class="text-center text-xs font-medium text-gray-500 dark:text-gray-400"
                    >
                      {{ wechatRecoveryScanLabel }}
                    </div>
                    <div class="mt-3 flex justify-center">
                      <canvas
                        ref="wechatRecoveryCanvasRef"
                        class="h-44 w-44"
                      ></canvas>
                    </div>
                  </div>
                </div>
              </div>
            </template>
            <!-- Plan list -->
            <template v-else>
              <div
                v-if="checkout.plans.length === 0"
                class="card py-16 text-center"
              >
                <Icon
                  name="gift"
                  size="xl"
                  class="mx-auto mb-3 text-gray-300 dark:text-dark-600"
                />
                <p class="text-gray-500 dark:text-gray-400">
                  {{ t("payment.noPlans") }}
                </p>
              </div>
              <div v-else :class="planGridClass">
                <SubscriptionPlanCard
                  v-for="plan in checkout.plans"
                  :key="plan.id"
                  :plan="plan"
                  :active-subscriptions="activeSubscriptions"
                  @select="selectPlan"
                />
              </div>
              <!-- Active subscriptions (compact, below plan list) -->
              <div v-if="activeSubscriptions.length > 0">
                <p
                  class="mb-2 text-xs font-medium text-gray-400 dark:text-gray-500"
                >
                  {{ t("payment.activeSubscription") }}
                </p>
                <div class="space-y-2">
                  <div
                    v-for="sub in activeSubscriptions"
                    :key="sub.id"
                    class="flex items-center gap-3 rounded-xl border border-gray-100 bg-white px-3 py-2 dark:border-dark-700 dark:bg-dark-800"
                  >
                    <div
                      :class="[
                        'h-6 w-1 shrink-0 rounded-full',
                        platformAccentBarClass(sub.group?.platform || ''),
                      ]"
                    />
                    <div class="min-w-0 flex-1">
                      <div class="flex items-center gap-1.5">
                        <span
                          class="truncate text-xs font-semibold text-gray-900 dark:text-white"
                          >{{
                            sub.group?.name || `Group #${sub.group_id}`
                          }}</span
                        >
                        <span
                          :class="[
                            'shrink-0 rounded-full px-1.5 py-0.5 text-[9px] font-medium',
                            platformBadgeLightClass(sub.group?.platform || ''),
                          ]"
                          >{{ platformLabel(sub.group?.platform || "") }}</span
                        >
                      </div>
                      <div
                        class="flex flex-wrap gap-x-3 text-[11px] text-gray-400 dark:text-gray-500"
                      >
                        <span
                          >{{ t("payment.planCard.rate") }}: ×{{
                            sub.group?.rate_multiplier ?? 1
                          }}</span
                        >
                        <span
                          v-if="
                            sub.group?.daily_limit_usd == null &&
                            sub.group?.weekly_limit_usd == null &&
                            sub.group?.monthly_limit_usd == null
                          "
                          >{{ t("payment.planCard.quota") }}:
                          {{ t("payment.planCard.unlimited") }}</span
                        >
                        <span v-if="sub.expires_at">{{
                          t("userSubscriptions.daysRemaining", {
                            days: getDaysRemaining(sub.expires_at),
                          })
                        }}</span>
                        <span v-else>{{
                          t("userSubscriptions.noExpiration")
                        }}</span>
                      </div>
                    </div>
                    <span class="badge badge-success shrink-0 text-[10px]">{{
                      t("userSubscriptions.status.active")
                    }}</span>
                  </div>
                </div>
              </div>
            </template>
          </template>
        </template>
        <div
          v-if="
            (checkout.help_text || checkout.help_image_url) &&
            paymentPhase === 'select' &&
            !selectedPlan
          "
          class="card p-4"
        >
          <div class="flex flex-col items-center gap-3">
            <img
              v-if="checkout.help_image_url"
              :src="checkout.help_image_url"
              alt=""
              class="h-40 max-w-full cursor-pointer rounded-lg object-contain transition-opacity hover:opacity-80"
              @click="previewImage = checkout.help_image_url"
            />
            <p
              v-if="checkout.help_text"
              class="text-center text-sm text-gray-500 dark:text-gray-400"
            >
              {{ checkout.help_text }}
            </p>
          </div>
        </div>
      </template>
    </div>
    <!-- Renewal Plan Selection Modal -->
    <Teleport to="body">
      <Transition name="modal">
        <div
          v-if="showRenewalModal"
          class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4"
          @click.self="closeRenewalModal"
        >
          <div
            class="relative w-full max-w-lg rounded-2xl border border-gray-200 bg-white p-6 shadow-2xl dark:border-dark-700 dark:bg-dark-900"
          >
            <!-- Close button -->
            <button
              class="absolute right-4 top-4 rounded-lg p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-200"
              @click="closeRenewalModal"
            >
              <svg
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
            <h3
              class="mb-4 text-lg font-semibold text-gray-900 dark:text-white"
            >
              {{ t("payment.selectPlan") }}
            </h3>
            <div class="space-y-4">
              <SubscriptionPlanCard
                v-for="plan in renewalPlans"
                :key="plan.id"
                :plan="plan"
                :active-subscriptions="activeSubscriptions"
                @select="selectPlanFromModal"
              />
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
    <!-- Image Preview Overlay -->
    <Teleport to="body">
      <Transition name="modal">
        <div
          v-if="previewImage"
          class="fixed inset-0 z-[60] flex items-center justify-center bg-black/70 backdrop-blur-sm"
          @click="previewImage = ''"
        >
          <img
            :src="previewImage"
            alt=""
            class="max-h-[85vh] max-w-[90vw] rounded-xl object-contain shadow-2xl"
          />
        </div>
      </Transition>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute } from "vue-router";
import QRCode from "qrcode";
import { useAuthStore } from "@/stores/auth";
import { usePaymentStore } from "@/stores/payment";
import { useSubscriptionStore } from "@/stores/subscriptions";
import { useAppStore } from "@/stores";
import { paymentAPI } from "@/api/payment";
import { useClipboard } from "@/composables/useClipboard";
import { extractApiErrorMessage } from "@/utils/apiError";
import { isMobileDevice } from "@/utils/device";
import type {
  CreateOrderResult,
  CheckoutInfoResponse,
  OrderType,
  SubscriptionPlan,
  WechatJsapiPayload,
} from "@/types/payment";
import AppLayout from "@/components/layout/AppLayout.vue";
import AmountInput from "@/components/payment/AmountInput.vue";
import PaymentMethodSelector from "@/components/payment/PaymentMethodSelector.vue";
import {
  METHOD_ORDER,
  PAYMENT_MODE_QRCODE,
  POPUP_WINDOW_FEATURES,
  normalizeVisiblePaymentType,
} from "@/components/payment/providerConfig";
import {
  platformAccentBarClass,
  platformBadgeLightClass,
  platformBadgeClass,
  platformTextClass,
  platformLabel,
} from "@/utils/platformColors";
import SubscriptionPlanCard from "@/components/payment/SubscriptionPlanCard.vue";
import PaymentStatusPanel from "@/components/payment/PaymentStatusPanel.vue";
import StripePaymentInline from "@/components/payment/StripePaymentInline.vue";
import Icon from "@/components/icons/Icon.vue";

const { t, locale } = useI18n();
const route = useRoute();
const authStore = useAuthStore();
const paymentStore = usePaymentStore();
const subscriptionStore = useSubscriptionStore();
const appStore = useAppStore();
const { copyToClipboard } = useClipboard();

const user = computed(() => authStore.user);
const activeSubscriptions = computed(
  () => subscriptionStore.activeSubscriptions,
);
const isMobile = computed(() => isMobileDevice());

const PENDING_WECHAT_PAYMENT_KEY = "sub2api.pending_wechat_payment";
const MAX_WECHAT_OAUTH_REDIRECTS = 2;
const WECHAT_CALLBACK_QUERY_KEYS = [
  "wechat_resume",
  "openid",
  "state",
  "scope",
  "payment_type",
  "amount",
  "order_type",
  "plan_id",
  "redirect",
  "error",
  "error_description",
  "errmsg",
  "message",
];
const VISIBLE_METHOD_ORDER = ["alipay", "wxpay", "stripe"] as const;

type VisiblePaymentMethod = (typeof VISIBLE_METHOD_ORDER)[number];
type PaymentStatusMode = "popup" | "jsapi";
type PaymentRedirectParams = Record<
  string,
  string | number | boolean | null | undefined
>;

interface ResolvedPaymentMethodOption {
  type: VisiblePaymentMethod;
  fee_rate: number;
  available: boolean;
  requestType: string;
  rawTypes: string[];
}

interface PendingWechatPaymentContext {
  amount: number;
  paymentType: string;
  orderType: OrderType;
  planId?: number;
  state?: string;
  returnTab: "recharge" | "subscription";
  oauthRedirectCount: number;
  savedAt: number;
}

interface NormalizedWechatJsapiPayload {
  appId: string;
  timeStamp: string;
  nonceStr: string;
  package: string;
  signType: string;
  paySign: string;
}

interface CreateOrderOptions {
  isResume?: boolean;
  openid?: string;
  requestType?: string;
}

interface WeixinJSBridgeLike {
  invoke(
    action: string,
    payload: Record<string, unknown>,
    callback: (response: Record<string, unknown>) => void,
  ): void;
}

function textWithFallback(
  key: string,
  zh: string,
  en: string,
  params?: Record<string, unknown>,
): string {
  const translated = params ? t(key, params) : t(key);
  if (translated !== key) return translated;
  return String(locale.value).toLowerCase().startsWith("zh") ? zh : en;
}

function getDaysRemaining(expiresAt: string): number {
  const diff = new Date(expiresAt).getTime() - Date.now();
  return Math.max(0, Math.ceil(diff / (1000 * 60 * 60 * 24)));
}

function firstString(...values: unknown[]): string {
  for (const value of values) {
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }
  return "";
}

function readResultString(
  result: Record<string, unknown>,
  ...keys: string[]
): string {
  return firstString(...keys.map((key) => result[key]));
}

function readResultObject(
  result: Record<string, unknown>,
  ...keys: string[]
): Record<string, unknown> {
  for (const key of keys) {
    const value = result[key];
    if (value && typeof value === "object" && !Array.isArray(value)) {
      return value as Record<string, unknown>;
    }
  }
  return {};
}

function readResultParams(
  result: Record<string, unknown>,
  ...keys: string[]
): PaymentRedirectParams {
  for (const key of keys) {
    const value = result[key];
    if (!value) continue;
    if (typeof value === "string") {
      try {
        const params = new URLSearchParams(
          value.startsWith("?") ? value.slice(1) : value,
        );
        const entries = Array.from(params.entries());
        if (entries.length > 0) return Object.fromEntries(entries);
      } catch {
        continue;
      }
    }
    if (typeof value === "object" && !Array.isArray(value)) {
      return value as PaymentRedirectParams;
    }
  }
  return {};
}

function readUrlParams(value: string): PaymentRedirectParams {
  if (!value) return {};
  try {
    const url = new URL(value, window.location.origin);
    const entries = Array.from(url.searchParams.entries());
    return entries.length > 0 ? Object.fromEntries(entries) : {};
  } catch {
    try {
      const params = new URLSearchParams(
        value.startsWith("?") ? value.slice(1) : value,
      );
      const entries = Array.from(params.entries());
      return entries.length > 0 ? Object.fromEntries(entries) : {};
    } catch {
      return {};
    }
  }
}

function setSearchParam(params: URLSearchParams, key: string, value: unknown) {
  if (value == null) return;
  const normalized = String(value).trim();
  if (!normalized) return;
  params.set(key, normalized);
}

function normalizeVisiblePaymentMethod(
  type: string,
): VisiblePaymentMethod | "" {
  const normalized = normalizeVisiblePaymentType(type);
  return VISIBLE_METHOD_ORDER.includes(normalized as VisiblePaymentMethod)
    ? (normalized as VisiblePaymentMethod)
    : "";
}

function sortRawMethods(methodTypes: string[]): string[] {
  return [...methodTypes].sort((a, b) => {
    const ai = METHOD_ORDER.indexOf(a as (typeof METHOD_ORDER)[number]);
    const bi = METHOD_ORDER.indexOf(b as (typeof METHOD_ORDER)[number]);
    return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi);
  });
}

function readQueryString(key: string): string {
  const value = route.query[key];
  if (Array.isArray(value)) {
    return typeof value[0] === "string" ? value[0] : "";
  }
  return typeof value === "string" ? value : "";
}

function loadPendingWechatPayment(): PendingWechatPaymentContext | null {
  try {
    const raw = window.localStorage.getItem(PENDING_WECHAT_PAYMENT_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Partial<PendingWechatPaymentContext>;
    if (
      typeof parsed.amount !== "number" ||
      !parsed.paymentType ||
      (parsed.orderType !== "balance" && parsed.orderType !== "subscription") ||
      (parsed.returnTab !== "recharge" &&
        parsed.returnTab !== "subscription") ||
      typeof parsed.savedAt !== "number"
    ) {
      window.localStorage.removeItem(PENDING_WECHAT_PAYMENT_KEY);
      return null;
    }
    if (Date.now() - parsed.savedAt > 15 * 60 * 1000) {
      window.localStorage.removeItem(PENDING_WECHAT_PAYMENT_KEY);
      return null;
    }
    return {
      amount: parsed.amount,
      paymentType: parsed.paymentType,
      orderType: parsed.orderType,
      planId: typeof parsed.planId === "number" ? parsed.planId : undefined,
      state: typeof parsed.state === "string" ? parsed.state : undefined,
      returnTab: parsed.returnTab,
      oauthRedirectCount:
        typeof parsed.oauthRedirectCount === "number"
          ? parsed.oauthRedirectCount
          : 0,
      savedAt: parsed.savedAt,
    };
  } catch {
    window.localStorage.removeItem(PENDING_WECHAT_PAYMENT_KEY);
    return null;
  }
}

function savePendingWechatPayment(context: PendingWechatPaymentContext) {
  window.localStorage.setItem(
    PENDING_WECHAT_PAYMENT_KEY,
    JSON.stringify(context),
  );
}

function clearPendingWechatPayment() {
  window.localStorage.removeItem(PENDING_WECHAT_PAYMENT_KEY);
}

function clearWechatCallbackQuery() {
  const url = new URL(window.location.href);
  WECHAT_CALLBACK_QUERY_KEYS.forEach((key) => url.searchParams.delete(key));
  window.history.replaceState({}, document.title, url.toString());
}

function isWechatOAuthCallback(): boolean {
  return (
    readQueryString("wechat_resume") === "1" ||
    !!readQueryString("openid") ||
    !!readQueryString("state")
  );
}

function isWechatBrowser(): boolean {
  return /micromessenger/i.test(window.navigator.userAgent);
}

function formatRecoveryAmount(value: number | null): string {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0)
    return "";
  return value.toFixed(2);
}

const loading = ref(true);
const submitting = ref(false);
const errorMessage = ref("");
const errorReason = ref("");
const activeTab = ref<"recharge" | "subscription">("recharge");
const amount = ref<number | null>(null);
const selectedMethod = ref("");
const selectedPlan = ref<SubscriptionPlan | null>(null);
const previewImage = ref("");
const wechatRecoveryCanvasRef = ref<HTMLCanvasElement | null>(null);

const paymentPhase = ref<"select" | "paying" | "stripe">("select");
const paymentState = ref<{
  orderId: number;
  amount: number;
  qrCode: string;
  expiresAt: string;
  paymentType: string;
  payUrl: string;
  clientSecret: string;
  publishableKey: string;
  payAmount: number;
  orderType: OrderType | "";
  statusMode: PaymentStatusMode;
}>({
  orderId: 0,
  amount: 0,
  qrCode: "",
  expiresAt: "",
  paymentType: "",
  payUrl: "",
  clientSecret: "",
  publishableKey: "",
  payAmount: 0,
  orderType: "",
  statusMode: "popup",
});

function resetPayment() {
  paymentPhase.value = "select";
  paymentState.value = {
    orderId: 0,
    amount: 0,
    qrCode: "",
    expiresAt: "",
    paymentType: "",
    payUrl: "",
    clientSecret: "",
    publishableKey: "",
    payAmount: 0,
    orderType: "",
    statusMode: "popup",
  };
  errorReason.value = "";
}

function onPaymentDone() {
  const wasSubscription = paymentState.value.orderType === "subscription";
  resetPayment();
  selectedPlan.value = null;
  if (wasSubscription) {
    subscriptionStore.fetchActiveSubscriptions(true).catch(() => {});
  }
}

function onPaymentSuccess() {
  authStore.refreshUser();
  if (paymentState.value.orderType === "subscription") {
    subscriptionStore.fetchActiveSubscriptions(true).catch(() => {});
  }
}

function onStripeDone() {
  const wasSubscription = paymentState.value.orderType === "subscription";
  resetPayment();
  selectedPlan.value = null;
  if (wasSubscription) {
    subscriptionStore.fetchActiveSubscriptions(true).catch(() => {});
  }
}

function onStripeRedirect(orderId: number, payUrl: string) {
  paymentState.value = {
    ...paymentState.value,
    orderId,
    payUrl,
    qrCode: "",
    statusMode: "popup",
  };
  paymentPhase.value = "paying";
}

const checkout = ref<CheckoutInfoResponse>({
  methods: {},
  global_min: 0,
  global_max: 0,
  plans: [],
  balance_disabled: false,
  balance_recharge_multiplier: 1,
  recharge_fee_rate: 0,
  help_text: "",
  help_image_url: "",
  stripe_publishable_key: "",
});

const tabs = computed(() => {
  const result: { key: "recharge" | "subscription"; label: string }[] = [];
  if (!checkout.value.balance_disabled)
    result.push({ key: "recharge", label: t("payment.tabTopUp") });
  result.push({ key: "subscription", label: t("payment.tabSubscribe") });
  return result;
});

const enabledMethods = computed(() => Object.keys(checkout.value.methods));
const validAmount = computed(() => amount.value ?? 0);
const balanceRechargeMultiplier = computed(() => {
  const multiplier = checkout.value.balance_recharge_multiplier;
  return multiplier > 0 ? multiplier : 1;
});
const creditedAmount = computed(
  () =>
    Math.round(validAmount.value * balanceRechargeMultiplier.value * 100) / 100,
);

const planGridClass = computed(() => {
  const count = checkout.value.plans.length;
  if (count <= 2) return "grid grid-cols-1 gap-5 sm:grid-cols-2";
  return "grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3";
});

function amountFitsRawMethod(amt: number, methodType: string): boolean {
  if (amt <= 0) return true;
  const methodLimit = checkout.value.methods[methodType];
  if (!methodLimit) return false;
  if (methodLimit.single_min > 0 && amt < methodLimit.single_min) return false;
  if (methodLimit.single_max > 0 && amt > methodLimit.single_max) return false;
  return true;
}

function isRawMethodEnabled(methodType: string): boolean {
  const method = checkout.value.methods[methodType];
  return !!method && method.available !== false;
}

function resolveVisibleMethodOptions(
  amt: number,
): ResolvedPaymentMethodOption[] {
  return VISIBLE_METHOD_ORDER.map((visibleType) => {
    const rawTypes = sortRawMethods(
      enabledMethods.value.filter(
        (methodType) =>
          normalizeVisiblePaymentMethod(methodType) === visibleType,
      ),
    );
    if (!rawTypes.length) return null;

    const enabledRawTypes = rawTypes.filter(isRawMethodEnabled);
    const eligibleRawTypes = enabledRawTypes.filter((methodType) =>
      amountFitsRawMethod(amt, methodType),
    );
    const preferredRawType =
      (amt > 0 ? eligibleRawTypes[0] : enabledRawTypes[0]) || rawTypes[0];

    return {
      type: visibleType,
      fee_rate: checkout.value.methods[preferredRawType]?.fee_rate ?? 0,
      available:
        amt > 0 ? eligibleRawTypes.length > 0 : enabledRawTypes.length > 0,
      requestType: preferredRawType,
      rawTypes,
    };
  }).filter((option): option is ResolvedPaymentMethodOption => option !== null);
}

function visibleMethodValidationError(
  amt: number,
  visibleType: string,
): string {
  if (amt <= 0) return "";

  const rawTypes = sortRawMethods(
    enabledMethods.value.filter(
      (methodType) => normalizeVisiblePaymentMethod(methodType) === visibleType,
    ),
  ).filter(isRawMethodEnabled);

  if (
    !rawTypes.length ||
    rawTypes.some((methodType) => amountFitsRawMethod(amt, methodType))
  ) {
    return "";
  }

  const minCandidates = rawTypes
    .map((methodType) => checkout.value.methods[methodType]?.single_min ?? 0)
    .filter((min) => min > 0);
  if (minCandidates.length > 0) {
    const minRequired = Math.min(...minCandidates);
    if (amt < minRequired)
      return t("payment.amountTooLow", { min: minRequired });
  }

  const maxCandidates = rawTypes
    .map((methodType) => checkout.value.methods[methodType]?.single_max ?? 0)
    .filter((max) => max > 0);
  if (maxCandidates.length > 0) {
    const maxAllowed = Math.max(...maxCandidates);
    if (amt > maxAllowed)
      return t("payment.amountTooHigh", { max: maxAllowed });
  }

  return "";
}

function getMethodOptionsForOrderType(
  orderType: OrderType,
): ResolvedPaymentMethodOption[] {
  return orderType === "subscription"
    ? subMethodOptions.value
    : methodOptions.value;
}

function resolveSelectedOption(
  orderType: OrderType,
  preferredRequestType?: string,
): ResolvedPaymentMethodOption | undefined {
  const options = getMethodOptionsForOrderType(orderType);
  if (preferredRequestType) {
    return (
      options.find(
        (option) =>
          option.rawTypes.includes(preferredRequestType) ||
          option.requestType === preferredRequestType,
      ) ??
      options.find(
        (option) =>
          option.type === normalizeVisiblePaymentMethod(preferredRequestType),
      )
    );
  }
  return options.find((option) => option.type === selectedMethod.value);
}

function resolveRequestedPaymentType(
  orderType: OrderType,
  preferredRequestType?: string,
): string {
  const selectedOption = resolveSelectedOption(orderType, preferredRequestType);
  if (!selectedOption) return preferredRequestType || "";
  if (
    preferredRequestType &&
    selectedOption.rawTypes.includes(preferredRequestType)
  ) {
    return preferredRequestType;
  }
  return selectedOption.requestType;
}

function buildPaymentResultUrl(
  result: Record<string, unknown>,
  orderType: OrderType,
  requestType: string,
): string {
  const wechatH5 = readResultObject(result, "wechat_h5", "wechatH5");
  const url = new URL("/payment/result", window.location.origin);
  const extraParams = {
    ...readUrlParams(readResultString(result, "return_url", "redirect_url")),
    ...readUrlParams(
      readResultString(result, "wechat_h5_return_url", "wechatH5ReturnUrl"),
    ),
    ...readUrlParams(
      readResultString(wechatH5, "url", "return_url", "redirect_url"),
    ),
    ...readResultParams(
      result,
      "return_params",
      "redirect_params",
      "wechat_h5_return_params",
      "wechatH5ReturnParams",
    ),
    ...readResultParams(wechatH5, "params", "return_params", "redirect_params"),
  };

  Object.entries(extraParams).forEach(([key, value]) =>
    setSearchParam(url.searchParams, key, value),
  );
  setSearchParam(url.searchParams, "order_id", result.order_id);
  setSearchParam(
    url.searchParams,
    "out_trade_no",
    readResultString(result, "out_trade_no", "outTradeNo"),
  );
  setSearchParam(
    url.searchParams,
    "payment_type",
    firstString(
      readResultString(result, "payment_type", "paymentType"),
      requestType,
      selectedMethod.value,
    ),
  );
  setSearchParam(url.searchParams, "order_type", orderType);
  return url.toString();
}

function buildWechatH5PayUrl(
  result: Record<string, unknown>,
  orderType: OrderType,
  requestType: string,
): string {
  const wechatH5 = readResultObject(result, "wechat_h5", "wechatH5");
  const payUrl = firstString(
    readResultString(result, "pay_url", "payUrl"),
    readResultString(wechatH5, "pay_url", "payUrl", "url"),
  );
  if (!payUrl) return "";
  try {
    const url = new URL(payUrl);
    url.searchParams.set(
      "redirect_url",
      buildPaymentResultUrl(result, orderType, requestType),
    );
    return url.toString();
  } catch {
    return payUrl;
  }
}

function isMobileWechatQrDeclined(
  result: Record<string, unknown>,
  requestType: string,
): boolean {
  const paymentMode = firstString(
    readResultString(result, "payment_mode", "paymentMode"),
  );
  return (
    isMobile.value &&
    normalizeVisiblePaymentMethod(requestType) === "wxpay" &&
    paymentMode === PAYMENT_MODE_QRCODE &&
    !readResultString(result, "pay_url", "payUrl") &&
    !!readResultString(result, "qr_code", "qrCode")
  );
}

function getWechatAuthorizeUrl(result: Record<string, unknown>): string {
  const oauth = readResultObject(result, "oauth");
  return firstString(
    readResultString(oauth, "authorize_url", "authorizeUrl"),
    readResultString(result, "authorize_url", "authorizeUrl"),
  );
}

function getWechatAuthorizeState(result: Record<string, unknown>): string {
  const oauth = readResultObject(result, "oauth");
  const state = firstString(
    readResultString(oauth, "state"),
    readResultString(result, "state"),
  );
  if (state) return state;
  const authorizeUrl = getWechatAuthorizeUrl(result);
  if (!authorizeUrl) return "";
  try {
    return (
      new URL(authorizeUrl, window.location.origin).searchParams.get("state") ||
      ""
    );
  } catch {
    return "";
  }
}

function normalizeWechatJsapiPayload(
  payload: WechatJsapiPayload | Record<string, unknown>,
): NormalizedWechatJsapiPayload | null {
  const appId = firstString(payload.appId, payload.appid);
  const timeStamp = firstString(payload.timeStamp, payload.timestamp);
  const nonceStr = firstString(payload.nonceStr, payload.nonce_str);
  const packageValue = firstString(payload.package);
  const signType = firstString(payload.signType, payload.sign_type) || "MD5";
  const paySign = firstString(payload.paySign, payload.pay_sign);
  if (
    !appId ||
    !timeStamp ||
    !nonceStr ||
    !packageValue ||
    !signType ||
    !paySign
  ) {
    return null;
  }
  return {
    appId,
    timeStamp,
    nonceStr,
    package: packageValue,
    signType,
    paySign,
  };
}

function getWeixinJSBridge(): WeixinJSBridgeLike | undefined {
  return (window as Window & { WeixinJSBridge?: WeixinJSBridgeLike })
    .WeixinJSBridge;
}

function waitForWeixinJSBridge(
  timeoutMs = 4000,
): Promise<WeixinJSBridgeLike | null> {
  const existingBridge = getWeixinJSBridge();
  if (existingBridge) return Promise.resolve(existingBridge);
  return new Promise((resolve) => {
    let settled = false;
    const handleReady = () => finish(getWeixinJSBridge() ?? null);
    const cleanup = () => {
      document.removeEventListener("WeixinJSBridgeReady", handleReady);
      document.removeEventListener("onWeixinJSBridgeReady", handleReady);
      window.clearTimeout(timer);
    };
    const finish = (bridge: WeixinJSBridgeLike | null) => {
      if (settled) return;
      settled = true;
      cleanup();
      resolve(bridge);
    };
    const timer = window.setTimeout(
      () => finish(getWeixinJSBridge() ?? null),
      timeoutMs,
    );
    document.addEventListener("WeixinJSBridgeReady", handleReady, false);
    document.addEventListener("onWeixinJSBridgeReady", handleReady, false);
  });
}

function invokeWechatPay(
  payload: NormalizedWechatJsapiPayload,
): Promise<Record<string, unknown>> {
  return new Promise((resolve) => {
    waitForWeixinJSBridge().then((bridge) => {
      if (!bridge) {
        resolve({ err_msg: "weixin_js_bridge_unavailable" });
        return;
      }
      bridge.invoke("getBrandWCPayRequest", { ...payload }, (response) => {
        resolve((response ?? {}) as Record<string, unknown>);
      });
    });
  });
}

function buildPendingWechatPaymentContext(
  orderAmount: number,
  orderType: OrderType,
  requestType: string,
  planId?: number,
): PendingWechatPaymentContext {
  return {
    amount: orderAmount,
    paymentType: requestType,
    orderType,
    planId,
    state: "",
    returnTab: orderType === "subscription" ? "subscription" : activeTab.value,
    oauthRedirectCount: 0,
    savedAt: Date.now(),
  };
}

function resolveDisplayPaymentType(paymentType: string): string {
  return normalizeVisiblePaymentMethod(paymentType) || paymentType;
}

function createWaitingPaymentState(
  result: CreateOrderResult,
  orderType: OrderType,
  overrides: Partial<typeof paymentState.value> = {},
) {
  paymentState.value = {
    orderId: result.order_id,
    amount: result.amount,
    qrCode: "",
    expiresAt: result.expires_at || "",
    paymentType: resolveDisplayPaymentType(
      firstString(result.payment_type, selectedMethod.value),
    ),
    payUrl: "",
    clientSecret: "",
    publishableKey: "",
    payAmount: result.pay_amount,
    orderType,
    statusMode: "popup",
    ...overrides,
  };
}

const globalMinAmount = computed(() => checkout.value.global_min);
const globalMaxAmount = computed(() => checkout.value.global_max);
const methodOptions = computed<ResolvedPaymentMethodOption[]>(() =>
  resolveVisibleMethodOptions(validAmount.value),
);

const feeRate = computed(() => checkout.value.recharge_fee_rate ?? 0);
const feeAmount = computed(() =>
  feeRate.value > 0 && validAmount.value > 0
    ? Math.ceil(((validAmount.value * feeRate.value) / 100) * 100) / 100
    : 0,
);
const totalAmount = computed(() =>
  feeRate.value > 0 && validAmount.value > 0
    ? Math.round((validAmount.value + feeAmount.value) * 100) / 100
    : validAmount.value,
);

const amountError = computed(() => {
  if (validAmount.value <= 0) return "";
  if (!methodOptions.value.some((method) => method.available)) {
    return t("payment.amountNoMethod");
  }
  return visibleMethodValidationError(validAmount.value, selectedMethod.value);
});

const canSubmit = computed(
  () =>
    validAmount.value > 0 &&
    methodOptions.value.some(
      (method) => method.type === selectedMethod.value && method.available,
    ),
);

const subMethodOptions = computed<ResolvedPaymentMethodOption[]>(() => {
  const planPrice = selectedPlan.value?.price ?? 0;
  return resolveVisibleMethodOptions(planPrice);
});

const subFeeAmount = computed(() => {
  const price = selectedPlan.value?.price ?? 0;
  if (feeRate.value <= 0 || price <= 0) return 0;
  return Math.ceil(((price * feeRate.value) / 100) * 100) / 100;
});

const subTotalAmount = computed(() => {
  const price = selectedPlan.value?.price ?? 0;
  if (feeRate.value <= 0 || price <= 0) return price;
  return Math.round((price + subFeeAmount.value) * 100) / 100;
});

const canSubmitSubscription = computed(
  () =>
    selectedPlan.value !== null &&
    subMethodOptions.value.some(
      (method) => method.type === selectedMethod.value && method.available,
    ),
);

watch(
  () => [validAmount.value, selectedMethod.value] as const,
  ([amt, method]) => {
    if (
      amt <= 0 ||
      methodOptions.value.some(
        (option) => option.type === method && option.available,
      )
    )
      return;
    const available = methodOptions.value.find((option) => option.available);
    if (available) selectedMethod.value = available.type;
  },
);

watch(
  () => [selectedPlan.value?.price ?? 0, selectedMethod.value] as const,
  ([price, method]) => {
    if (
      price <= 0 ||
      subMethodOptions.value.some(
        (option) => option.type === method && option.available,
      )
    )
      return;
    const available = subMethodOptions.value.find((option) => option.available);
    if (available) selectedMethod.value = available.type;
  },
);

const paymentButtonClass = computed(() => {
  const method = selectedMethod.value;
  if (!method) return "btn-primary";
  if (method === "alipay") return "btn-alipay";
  if (method === "wxpay") return "btn-wxpay";
  if (method === "stripe") return "btn-stripe";
  return "btn-primary";
});

const planBadgeClass = computed(() =>
  platformBadgeClass(selectedPlan.value?.group_platform || ""),
);
const planTextClass = computed(() =>
  platformTextClass(selectedPlan.value?.group_platform || ""),
);

const showRenewalModal = ref(false);
const renewGroupId = ref<number | null>(null);
const renewalPlans = computed(() => {
  if (renewGroupId.value == null) return [];
  return checkout.value.plans.filter(
    (plan) => plan.group_id === renewGroupId.value,
  );
});

const planValiditySuffix = computed(() => {
  if (!selectedPlan.value) return "";
  const unit = selectedPlan.value.validity_unit || "day";
  if (unit === "month") return t("payment.perMonth");
  if (unit === "year") return t("payment.perYear");
  return `${selectedPlan.value.validity_days}${t("payment.days")}`;
});

function currentRecoveryRequestType(): string {
  const orderType =
    activeTab.value === "subscription" ? "subscription" : "balance";
  return resolveRequestedPaymentType(orderType);
}

const showWechatRecoveryHint = computed(
  () =>
    errorReason.value === "WECHAT_H5_NOT_AUTHORIZED" &&
    selectedMethod.value === "wxpay" &&
    paymentPhase.value === "select",
);

const wechatRecoveryTitle = computed(() =>
  textWithFallback(
    "payment.wechat.openCurrentPageTitle",
    "在微信里继续当前支付",
    "Continue this payment inside WeChat",
  ),
);

const wechatRecoveryHintText = computed(() =>
  textWithFallback(
    "payment.wechat.openCurrentPageHint",
    "请在微信内打开当前支付页，再重新发起支付。系统会自动切换到微信内可用的支付流程。",
    "Open this payment page inside WeChat, then retry the payment. The flow will switch to the in-WeChat payment path automatically.",
  ),
);

const wechatRecoveryCopyLabel = computed(() =>
  textWithFallback(
    "payment.wechat.copyCurrentPageLink",
    "复制当前页面链接",
    "Copy current page link",
  ),
);

const wechatRecoveryScanLabel = computed(() =>
  textWithFallback(
    "payment.wechat.scanToOpenCurrentPage",
    "扫码在微信中打开当前页",
    "Scan to open this page in WeChat",
  ),
);

const wechatRecoveryUrl = computed(() => {
  if (typeof window === "undefined") return "";

  const url = new URL(window.location.href);
  WECHAT_CALLBACK_QUERY_KEYS.forEach((key) => url.searchParams.delete(key));
  url.searchParams.set("tab", activeTab.value);

  const requestType = currentRecoveryRequestType();
  if (requestType) {
    url.searchParams.set("payment_type", requestType);
  } else if (selectedMethod.value) {
    url.searchParams.set("payment_type", selectedMethod.value);
  } else {
    url.searchParams.delete("payment_type");
  }

  if (activeTab.value === "recharge") {
    const formattedAmount = formatRecoveryAmount(amount.value);
    if (formattedAmount) {
      url.searchParams.set("amount", formattedAmount);
    } else {
      url.searchParams.delete("amount");
    }
    url.searchParams.delete("plan_id");
    url.searchParams.delete("group");
  } else if (selectedPlan.value) {
    url.searchParams.set("plan_id", String(selectedPlan.value.id));
    url.searchParams.set("group", String(selectedPlan.value.group_id));
    url.searchParams.delete("amount");
  }

  return url.toString();
});

const wechatRecoverySummary = computed(() => {
  const methodLabel = t(
    `payment.methods.${selectedMethod.value}`,
    selectedMethod.value || t("payment.methods.wxpay"),
  );
  if (activeTab.value === "subscription" && selectedPlan.value) {
    return textWithFallback(
      "payment.wechat.openCurrentPageSummaryPlan",
      `当前将使用${methodLabel}购买套餐“${selectedPlan.value.name}”。`,
      `This page will resume the ${methodLabel} payment for plan "${selectedPlan.value.name}".`,
      { method: methodLabel, plan: selectedPlan.value.name },
    );
  }
  return textWithFallback(
    "payment.wechat.openCurrentPageSummaryAmount",
    `当前将使用${methodLabel}支付 ¥${formatRecoveryAmount(amount.value) || "0.00"}。`,
    `This page will resume the ${methodLabel} payment for ¥${formatRecoveryAmount(amount.value) || "0.00"}.`,
    {
      method: methodLabel,
      amount: formatRecoveryAmount(amount.value) || "0.00",
    },
  );
});

async function renderWechatRecoveryQr() {
  if (
    !showWechatRecoveryHint.value ||
    !wechatRecoveryCanvasRef.value ||
    !wechatRecoveryUrl.value
  )
    return;
  await nextTick();
  if (!wechatRecoveryCanvasRef.value) return;
  await QRCode.toCanvas(
    wechatRecoveryCanvasRef.value,
    wechatRecoveryUrl.value,
    {
      width: 176,
      margin: 1,
      color: {
        dark: "#111827",
        light: "#FFFFFF",
      },
    },
  );
}

async function copyWechatRecoveryUrl() {
  await copyToClipboard(
    wechatRecoveryUrl.value,
    textWithFallback(
      "payment.wechat.currentPageLinkCopied",
      "当前页面链接已复制",
      "The current page link has been copied",
    ),
  );
}

async function handleWechatOAuthRequired(
  result: CreateOrderResult,
  orderAmount: number,
  orderType: OrderType,
  requestType: string,
  planId?: number,
  options: CreateOrderOptions = {},
) {
  if (options.isResume || options.openid) {
    clearPendingWechatPayment();
    errorMessage.value = textWithFallback(
      "payment.wechat.authResumeFailed",
      "微信授权已完成，但支付仍然要求重新授权，请重新发起支付。",
      "WeChat authorization finished, but the payment still requires authorization. Please restart the payment flow.",
    );
    appStore.showError(errorMessage.value);
    return;
  }

  const authorizeUrl = getWechatAuthorizeUrl(result as Record<string, unknown>);
  if (!authorizeUrl) {
    clearPendingWechatPayment();
    errorMessage.value = textWithFallback(
      "payment.wechat.authMissingUrl",
      "支付返回了微信授权要求，但缺少授权地址。",
      "The payment requires WeChat authorization, but no authorization URL was returned.",
    );
    appStore.showError(errorMessage.value);
    return;
  }

  const pending =
    loadPendingWechatPayment() ??
    buildPendingWechatPaymentContext(
      orderAmount,
      orderType,
      requestType,
      planId,
    );
  const nextRedirectCount = (pending.oauthRedirectCount || 0) + 1;
  if (nextRedirectCount > MAX_WECHAT_OAUTH_REDIRECTS) {
    clearPendingWechatPayment();
    errorMessage.value = textWithFallback(
      "payment.wechat.authRetryExceeded",
      "微信授权重试次数过多，请重新发起支付。",
      "Too many WeChat authorization retries. Please restart the payment.",
    );
    appStore.showError(errorMessage.value);
    return;
  }

  savePendingWechatPayment({
    ...pending,
    amount: orderAmount,
    paymentType: requestType,
    orderType,
    planId,
    state: getWechatAuthorizeState(result as Record<string, unknown>),
    returnTab: orderType === "subscription" ? "subscription" : activeTab.value,
    oauthRedirectCount: nextRedirectCount,
    savedAt: Date.now(),
  });

  appStore.showInfo(
    textWithFallback(
      options.isResume
        ? "payment.wechat.authRetrying"
        : "payment.wechat.authRedirecting",
      options.isResume ? "正在重新拉起微信授权..." : "正在跳转到微信授权...",
      options.isResume
        ? "Retrying WeChat authorization..."
        : "Redirecting to WeChat authorization...",
    ),
    2500,
  );
  window.location.href = authorizeUrl;
}

async function handleWechatJsapiReady(
  result: CreateOrderResult,
  orderType: OrderType,
  requestType: string,
) {
  if (!isWechatBrowser()) {
    errorMessage.value = textWithFallback(
      "payment.wechat.jsapiUnavailable",
      "当前环境无法拉起微信内支付，请在微信中打开后重试。",
      "WeChat in-app payment is unavailable in this environment. Open the page in WeChat and try again.",
    );
    appStore.showError(errorMessage.value);
    return;
  }

  const rawPayload = (result.jsapi_payload ||
    result.jsapi ||
    {}) as WechatJsapiPayload;
  const payload = normalizeWechatJsapiPayload(rawPayload);
  if (!payload) {
    errorMessage.value = textWithFallback(
      "payment.wechat.jsapiInvalidPayload",
      "微信支付参数不完整，请稍后重试。",
      "WeChat Pay returned incomplete JSAPI parameters. Please try again.",
    );
    appStore.showError(errorMessage.value);
    return;
  }

  clearPendingWechatPayment();
  createWaitingPaymentState(result, orderType, {
    paymentType: resolveDisplayPaymentType(
      firstString(result.payment_type, requestType, selectedMethod.value),
    ),
    statusMode: "jsapi",
  });
  paymentPhase.value = "paying";

  appStore.showInfo(
    textWithFallback(
      "payment.wechat.jsapiInvoking",
      "正在拉起微信支付...",
      "Launching WeChat Pay...",
    ),
    2500,
  );

  const response = await invokeWechatPay(payload);
  const errMsg = firstString(response.err_msg, response.errMsg).toLowerCase();

  if (errMsg.includes(":ok")) {
    appStore.showInfo(
      textWithFallback(
        "payment.wechat.jsapiProcessing",
        "支付已发起，正在等待微信确认...",
        "Payment launched. Waiting for WeChat confirmation...",
      ),
      3000,
    );
    return;
  }

  if (errMsg.includes(":cancel")) {
    appStore.showWarning(
      textWithFallback(
        "payment.wechat.jsapiCancelled",
        "已取消微信支付",
        "WeChat Pay was cancelled",
      ),
    );
    return;
  }

  const failureReason =
    firstString(response.err_msg, response.errMsg) || "unknown";
  const failureMessage =
    errMsg === "weixin_js_bridge_unavailable"
      ? textWithFallback(
          "payment.wechat.jsapiUnavailable",
          "当前环境无法拉起微信内支付，请在微信中打开后重试。",
          "WeChat in-app payment is unavailable in this environment. Open the page in WeChat and try again.",
        )
      : textWithFallback(
          "payment.wechat.jsapiFailed",
          `微信支付拉起失败：${failureReason}`,
          `Failed to launch WeChat Pay: ${failureReason}`,
          { reason: failureReason },
        );
  appStore.showError(failureMessage);
}

async function consumeOrderCreatedResult(
  result: CreateOrderResult,
  orderType: OrderType,
  requestType: string,
  selectedOption: ResolvedPaymentMethodOption,
) {
  const rawResult = result as Record<string, unknown>;
  const displayType =
    selectedOption.type ||
    resolveDisplayPaymentType(
      firstString(result.payment_type, requestType, selectedMethod.value),
    );
  const openWindow = (url: string) => {
    const popup = window.open(url, "paymentPopup", POPUP_WINDOW_FEATURES);
    if (!popup || popup.closed) {
      window.location.href = url;
    }
  };

  clearPendingWechatPayment();

  if (result.client_secret) {
    paymentState.value = {
      orderId: result.order_id,
      amount: result.amount,
      qrCode: "",
      expiresAt: result.expires_at || "",
      paymentType: displayType,
      payUrl: "",
      clientSecret: result.client_secret,
      publishableKey:
        result.stripe_publishable_key || checkout.value.stripe_publishable_key,
      payAmount: result.pay_amount,
      orderType,
      statusMode: "popup",
    };
    paymentPhase.value = "stripe";
    return;
  }

  if (isMobileWechatQrDeclined(rawResult, requestType)) {
    errorMessage.value = `${t(`payment.methods.${displayType}`, displayType)} ${t("payment.notAvailable")}`;
    appStore.showError(errorMessage.value);
    return;
  }

  if (isMobile.value && result.pay_url) {
    const mobilePayUrl =
      displayType === "wxpay"
        ? buildWechatH5PayUrl(rawResult, orderType, requestType) ||
          result.pay_url
        : result.pay_url;
    createWaitingPaymentState(result, orderType, {
      paymentType: displayType,
      payUrl: mobilePayUrl,
      statusMode: "popup",
    });
    paymentPhase.value = "paying";
    window.location.href = mobilePayUrl;
    return;
  }

  if (result.qr_code) {
    createWaitingPaymentState(result, orderType, {
      paymentType: displayType,
      qrCode: result.qr_code,
      statusMode: "popup",
    });
    paymentPhase.value = "paying";
    return;
  }

  if (result.pay_url) {
    const payUrl =
      displayType === "wxpay"
        ? buildWechatH5PayUrl(rawResult, orderType, requestType) ||
          result.pay_url
        : result.pay_url;
    openWindow(payUrl);
    createWaitingPaymentState(result, orderType, {
      paymentType: displayType,
      payUrl,
      statusMode: "popup",
    });
    paymentPhase.value = "paying";
    return;
  }

  errorMessage.value = t("payment.result.failed");
  appStore.showError(errorMessage.value);
}

async function maybeResumeWechatPayment() {
  if (!isWechatOAuthCallback()) return;

  const callbackError = firstString(
    readQueryString("error"),
    readQueryString("error_description"),
    readQueryString("errmsg"),
    readQueryString("message"),
  );
  if (callbackError) {
    clearPendingWechatPayment();
    clearWechatCallbackQuery();
    errorMessage.value = callbackError;
    appStore.showError(callbackError);
    return;
  }

  const pending = loadPendingWechatPayment();
  const callbackState = readQueryString("state");
  const openid = readQueryString("openid");
  if (!pending) {
    clearWechatCallbackQuery();
    errorMessage.value = textWithFallback(
      "payment.wechat.authExpired",
      "微信支付授权已过期，请重新发起支付。",
      "The WeChat payment authorization has expired. Please restart the payment.",
    );
    appStore.showError(errorMessage.value);
    return;
  }
  if (!openid) {
    clearPendingWechatPayment();
    clearWechatCallbackQuery();
    errorMessage.value = textWithFallback(
      "payment.wechat.callbackMissingOpenId",
      "微信支付回跳缺少 openid，无法恢复支付。",
      "The WeChat payment callback is missing the openid, so the payment cannot be resumed.",
    );
    appStore.showError(errorMessage.value);
    return;
  }
  if (pending.state && callbackState && pending.state !== callbackState) {
    clearPendingWechatPayment();
    clearWechatCallbackQuery();
    errorMessage.value = textWithFallback(
      "payment.wechat.authStateMismatch",
      "微信支付授权状态不匹配，请重新发起支付。",
      "The WeChat payment authorization state did not match. Please restart the payment.",
    );
    appStore.showError(errorMessage.value);
    return;
  }

  selectedMethod.value =
    normalizeVisiblePaymentMethod(pending.paymentType) || selectedMethod.value;
  activeTab.value = pending.returnTab;

  if (pending.orderType === "subscription") {
    activeTab.value = "subscription";
    const resumePlan = checkout.value.plans.find(
      (plan) => plan.id === pending.planId,
    );
    if (!resumePlan) {
      clearPendingWechatPayment();
      clearWechatCallbackQuery();
      errorMessage.value = textWithFallback(
        "payment.wechat.resumePlanMissing",
        "待恢复的套餐已不可用，请重新选择套餐。",
        "The plan for the resumed WeChat payment is no longer available. Please choose a plan again.",
      );
      appStore.showError(errorMessage.value);
      return;
    }
    selectedPlan.value = resumePlan;
  } else {
    amount.value = pending.amount;
  }

  clearWechatCallbackQuery();
  appStore.showInfo(
    textWithFallback(
      "payment.wechat.authResuming",
      "正在恢复微信支付...",
      "Resuming WeChat payment...",
    ),
    2500,
  );
  await createOrder(pending.amount, pending.orderType, pending.planId, {
    isResume: true,
    openid,
    requestType: pending.paymentType,
  });
}

function selectPlan(plan: SubscriptionPlan) {
  selectedPlan.value = plan;
  errorMessage.value = "";
}

function selectPlanFromModal(plan: SubscriptionPlan) {
  showRenewalModal.value = false;
  renewGroupId.value = null;
  selectedPlan.value = plan;
  errorMessage.value = "";
}

function closeRenewalModal() {
  showRenewalModal.value = false;
  renewGroupId.value = null;
}

async function handleSubmitRecharge() {
  if (!canSubmit.value || submitting.value) return;
  await createOrder(validAmount.value, "balance");
}

async function confirmSubscribe() {
  if (!selectedPlan.value || submitting.value) return;
  await createOrder(
    selectedPlan.value.price,
    "subscription",
    selectedPlan.value.id,
  );
}

async function createOrder(
  orderAmount: number,
  orderType: OrderType,
  planId?: number,
  options: CreateOrderOptions = {},
) {
  submitting.value = true;
  errorMessage.value = "";
  errorReason.value = "";
  try {
    const selectedOption = resolveSelectedOption(
      orderType,
      options.requestType,
    );
    if (!selectedOption) {
      errorMessage.value = t("payment.notAvailable");
      appStore.showError(errorMessage.value);
      return;
    }

    const requestType = resolveRequestedPaymentType(
      orderType,
      options.requestType,
    );
    const result = await paymentStore.createOrder({
      amount: orderAmount,
      payment_type: requestType,
      openid: options.openid,
      order_type: orderType,
      plan_id: planId,
    });

    const resultType = result.result_type || "order_created";
    if (resultType === "oauth_required") {
      await handleWechatOAuthRequired(
        result,
        orderAmount,
        orderType,
        requestType,
        planId,
        options,
      );
    } else if (resultType === "jsapi_ready") {
      await handleWechatJsapiReady(result, orderType, requestType);
    } else {
      await consumeOrderCreatedResult(
        result,
        orderType,
        requestType,
        selectedOption,
      );
    }
  } catch (err: unknown) {
    const apiErr = err as Record<string, unknown>;
    errorReason.value = typeof apiErr.reason === "string" ? apiErr.reason : "";
    if (apiErr.reason === "TOO_MANY_PENDING") {
      const metadata = apiErr.metadata as Record<string, unknown> | undefined;
      errorMessage.value = t("payment.errors.tooManyPending", {
        max: metadata?.max || "",
      });
    } else if (apiErr.reason === "CANCEL_RATE_LIMITED") {
      errorMessage.value = t("payment.errors.cancelRateLimited");
    } else if (apiErr.reason === "WECHAT_H5_NOT_AUTHORIZED") {
      errorMessage.value = textWithFallback(
        "payment.errors.wechatH5NotAuthorized",
        "当前微信支付需要在微信内完成，请用微信打开当前页面后重试。",
        "This WeChat payment must be completed inside WeChat. Open the current page in WeChat and try again.",
      );
    } else if (apiErr.reason === "WECHAT_PAYMENT_MP_NOT_CONFIGURED") {
      errorMessage.value = textWithFallback(
        "payment.errors.wechatPaymentMpNotConfigured",
        "当前站点未配置微信内支付凭证，请联系管理员或改用其他支付方式。",
        "This site has not configured the WeChat in-app payment credential. Contact the administrator or use another payment method.",
      );
    } else if (apiErr.reason === "NO_AVAILABLE_INSTANCE") {
      errorMessage.value = textWithFallback(
        "payment.errors.noAvailableInstance",
        "当前没有可用的支付通道，请稍后重试。",
        "There is no available payment channel right now. Please try again later.",
      );
    } else {
      errorMessage.value = extractApiErrorMessage(
        err,
        t("payment.result.failed"),
      );
    }
    appStore.showError(errorMessage.value);
  } finally {
    submitting.value = false;
  }
}

onMounted(async () => {
  try {
    const response = await paymentAPI.getCheckoutInfo();
    checkout.value = response.data;
    if (methodOptions.value.length > 0) {
      selectedMethod.value = methodOptions.value[0].type;
    }
    if (checkout.value.balance_disabled) {
      activeTab.value = "subscription";
    }
    if (route.query.tab === "subscription") {
      activeTab.value = "subscription";
      if (route.query.plan_id) {
        const planId = Number(route.query.plan_id);
        if (Number.isFinite(planId) && planId > 0) {
          const matchedPlan = checkout.value.plans.find(
            (plan) => plan.id === planId,
          );
          if (matchedPlan) {
            selectedPlan.value = matchedPlan;
          }
        }
      }
      if (!selectedPlan.value && route.query.group) {
        const groupId = Number(route.query.group);
        const groupPlans = checkout.value.plans.filter(
          (plan) => plan.group_id === groupId,
        );
        if (groupPlans.length === 1) {
          selectedPlan.value = groupPlans[0];
        } else if (groupPlans.length > 1) {
          renewGroupId.value = groupId;
          showRenewalModal.value = true;
        }
      }
    }
    if (typeof route.query.amount === "string") {
      const queryAmount = Number(route.query.amount);
      if (Number.isFinite(queryAmount) && queryAmount > 0) {
        amount.value = queryAmount;
      }
    }
    if (typeof route.query.payment_type === "string") {
      const normalized = normalizeVisiblePaymentMethod(
        route.query.payment_type,
      );
      if (
        normalized &&
        methodOptions.value.some((option) => option.type === normalized)
      ) {
        selectedMethod.value = normalized;
      }
    }
    await maybeResumeWechatPayment();
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
  } finally {
    loading.value = false;
  }
  subscriptionStore.fetchActiveSubscriptions().catch(() => {});
});

watch(
  [showWechatRecoveryHint, wechatRecoveryUrl],
  async ([visible]) => {
    if (!visible) return;
    await renderWechatRecoveryQr();
  },
  { flush: "post" },
);
</script>
