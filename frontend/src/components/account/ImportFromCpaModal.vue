<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.cpaImportTitle')"
    width="normal"
    close-on-click-outside
    @close="handleClose"
  >
    <form
      v-if="currentStep === 'input'"
      id="import-from-cpa-preview-form"
      class="space-y-4"
      @submit.prevent="handlePreview"
    >
      <div class="grid grid-cols-2 gap-2">
        <button
          type="button"
          class="rounded-lg border px-3 py-2 text-sm transition-colors"
          :class="
            importMode === 'remote'
              ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300'
              : 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-dark-600 dark:text-dark-300'
          "
          @click="setImportMode('remote')"
        >
          {{ t("admin.accounts.cpaImportModeRemote") }}
        </button>
        <button
          type="button"
          class="rounded-lg border px-3 py-2 text-sm transition-colors"
          :class="
            importMode === 'file'
              ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300'
              : 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-dark-600 dark:text-dark-300'
          "
          @click="setImportMode('file')"
        >
          {{ t("admin.accounts.cpaImportModeFile") }}
        </button>
      </div>

      <div class="text-sm text-gray-600 dark:text-dark-300">
        {{
          importMode === "remote"
            ? t("admin.accounts.cpaImportRemoteHint")
            : t("admin.accounts.cpaImportHint")
        }}
      </div>
      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-600 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {{
          importMode === "remote"
            ? t("admin.accounts.cpaImportRemoteWarning")
            : t("admin.accounts.cpaImportWarning")
        }}
      </div>

      <template v-if="importMode === 'remote'">
        <div>
          <label for="cpa-base-url" class="input-label">{{
            t("admin.accounts.cpaRemoteBaseUrl")
          }}</label>
          <input
            id="cpa-base-url"
            v-model="form.base_url"
            type="text"
            class="input"
            required
            :placeholder="t('admin.accounts.cpaRemoteBaseUrlPlaceholder')"
          />
        </div>

        <div>
          <label for="cpa-management-key" class="input-label">{{
            t("admin.accounts.cpaRemoteManagementKey")
          }}</label>
          <input
            id="cpa-management-key"
            v-model="form.management_key"
            type="password"
            class="input"
            required
            autocomplete="off"
          />
        </div>
      </template>

      <div v-else>
        <label class="input-label">{{
          t("admin.accounts.cpaImportFile")
        }}</label>
        <div
          class="flex items-center justify-between gap-3 rounded-lg border border-dashed border-gray-300 bg-gray-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="min-w-0">
            <div class="truncate text-sm text-gray-700 dark:text-dark-200">
              {{ fileName || t("admin.accounts.cpaImportSelectFile") }}
            </div>
            <div class="text-xs text-gray-500 dark:text-dark-400">
              JSON (.json)
            </div>
          </div>
          <button
            type="button"
            class="btn btn-secondary shrink-0"
            @click="openFilePicker"
          >
            {{ t("common.chooseFile") }}
          </button>
        </div>
        <input
          ref="fileInput"
          type="file"
          class="hidden"
          accept="application/json,.json"
          @change="handleFileChange"
        />
      </div>
    </form>

    <div v-else-if="currentStep === 'preview'" class="space-y-4">
      <template v-if="importMode === 'remote' && remotePreviewResult">
        <div
          class="space-y-2 rounded-xl border border-gray-200 p-4 dark:border-dark-700"
        >
          <div class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t("admin.accounts.cpaRemotePreviewSummaryTitle") }}
          </div>
          <div class="text-sm text-gray-700 dark:text-dark-300">
            {{
              t("admin.accounts.cpaRemotePreviewSummary", remoteSummaryParams)
            }}
          </div>
          <div class="text-xs text-gray-500 dark:text-dark-400">
            {{ t("admin.accounts.cpaRemoteStatusFilterNote") }}
          </div>
        </div>

        <div
          v-if="remoteExistingAccounts.length"
          class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700/60"
        >
          <div
            class="mb-2 text-sm font-medium text-gray-700 dark:text-dark-300"
          >
            {{ t("admin.accounts.cpaRemoteExistingAccounts") }}
            <span class="ml-1 text-xs text-gray-400"
              >({{ remoteExistingAccounts.length }})</span
            >
          </div>
          <div
            class="max-h-32 overflow-auto text-xs text-gray-500 dark:text-dark-400"
          >
            <div
              v-for="item in remoteExistingAccounts"
              :key="item.account.cpa_source_key"
              class="flex items-center gap-2 py-0.5"
            >
              <span
                class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
                >{{ item.account.provider }}</span
              >
              <span
                class="inline-block rounded bg-green-100 px-1.5 py-0.5 text-[10px] font-medium text-green-700 dark:bg-green-900/30 dark:text-green-400"
                >{{ item.account.platform }} / {{ item.account.type }}</span
              >
              <span class="truncate">{{ item.account.name }}</span>
            </div>
          </div>
        </div>

        <div v-if="remoteNewAccounts.length">
          <div class="mb-2 flex items-center justify-between">
            <div class="text-sm font-medium text-gray-900 dark:text-white">
              {{ t("admin.accounts.cpaRemoteNewAccounts") }}
              <span class="ml-1 text-xs text-gray-400"
                >({{ remoteNewAccounts.length }})</span
              >
            </div>
            <div class="flex gap-2">
              <button
                type="button"
                class="text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400"
                @click="selectAllRemote"
              >
                {{ t("admin.accounts.crsSelectAll") }}
              </button>
              <button
                type="button"
                class="text-xs text-gray-500 hover:text-gray-600 dark:text-gray-400"
                @click="selectNoneRemote"
              >
                {{ t("admin.accounts.crsSelectNone") }}
              </button>
            </div>
          </div>

          <div
            class="max-h-56 overflow-auto rounded-lg border border-gray-200 p-2 dark:border-dark-600"
          >
            <label
              v-for="item in remoteNewAccounts"
              :key="item.account.cpa_source_key"
              class="flex cursor-pointer items-start gap-2 rounded px-2 py-1.5 hover:bg-gray-50 dark:hover:bg-dark-700/40"
            >
              <input
                type="checkbox"
                :checked="
                  selectedRemoteSourceKeys.has(item.account.cpa_source_key)
                "
                class="mt-0.5 rounded border-gray-300 dark:border-dark-600"
                @change="toggleRemoteSelect(item.account.cpa_source_key)"
              />
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span
                    class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
                    >{{ item.account.provider }}</span
                  >
                  <span
                    class="inline-block rounded bg-green-100 px-1.5 py-0.5 text-[10px] font-medium text-green-700 dark:bg-green-900/30 dark:text-green-400"
                    >{{ item.account.platform }} / {{ item.account.type }}</span
                  >
                </div>
                <div class="truncate text-sm text-gray-700 dark:text-dark-300">
                  {{ item.account.name }}
                </div>
                <div
                  v-if="item.account.email"
                  class="truncate text-xs text-gray-500 dark:text-dark-400"
                >
                  {{ item.account.email }}
                </div>
                <div
                  v-if="item.account.warnings?.length"
                  class="mt-1 text-xs text-amber-600 dark:text-amber-400"
                >
                  {{ item.account.warnings[0] }}
                </div>
              </div>
            </label>
          </div>

          <div class="mt-1 text-xs text-gray-400">
            {{
              t("admin.accounts.crsSelectedCount", {
                count: selectedRemoteSourceKeys.size,
              })
            }}
          </div>
        </div>

        <div
          v-if="!remotePreviewResult.items.length"
          class="rounded-lg bg-gray-50 p-4 text-center text-sm text-gray-500 dark:bg-dark-700/60 dark:text-dark-400"
        >
          {{ t("admin.accounts.cpaRemoteNoImportableAccounts") }}
        </div>

        <div v-if="remoteNewAccounts.length" class="space-y-4">
          <div>
            <label class="input-label">{{ t("admin.accounts.proxy") }}</label>
            <ProxySelector v-model="form.proxy_id" :proxies="proxies" />
          </div>

          <div>
            <label class="input-label">{{
              t("admin.accounts.cpaImportConcurrency")
            }}</label>
            <input
              v-model.number="form.concurrency"
              type="number"
              min="1"
              class="input"
              @input="
                form.concurrency = Math.max(
                  1,
                  form.concurrency || defaultConcurrency,
                )
              "
            />
          </div>

          <label
            class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-300"
          >
            <input
              v-model="form.use_default_group_bind"
              type="checkbox"
              class="rounded border-gray-300 dark:border-dark-600"
            />
            {{ t("admin.accounts.cpaUseDefaultGroupBind") }}
          </label>

          <div
            v-if="!form.use_default_group_bind"
            class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"
          >
            <div class="mb-3 text-sm text-gray-600 dark:text-dark-300">
              {{ t("admin.accounts.cpaManualGroupsHint") }}
            </div>
            <GroupSelector
              v-if="remoteSelectedPlatform"
              v-model="form.group_ids"
              :groups="groups"
              :platform="remoteSelectedPlatform"
            />
            <div
              v-else
              class="rounded-lg bg-amber-50 p-3 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
            >
              {{ t("admin.accounts.cpaManualGroupsMultiPlatformHint") }}
            </div>
          </div>
        </div>
      </template>

      <template v-else-if="filePreviewResult">
        <div
          class="space-y-2 rounded-xl border border-gray-200 p-4 dark:border-dark-700"
        >
          <div class="flex flex-wrap items-center gap-2">
            <span
              class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
            >
              {{ filePreviewResult.account.provider }}
            </span>
            <span
              class="inline-block rounded bg-green-100 px-1.5 py-0.5 text-[10px] font-medium text-green-700 dark:bg-green-900/30 dark:text-green-400"
            >
              {{ filePreviewResult.account.platform }} /
              {{ filePreviewResult.account.type }}
            </span>
            <span class="text-xs text-gray-400">{{
              filePreviewResult.account.file_name
            }}</span>
          </div>
          <div class="text-sm font-medium text-gray-900 dark:text-white">
            {{ filePreviewResult.account.name }}
          </div>
          <div
            v-if="filePreviewResult.account.email"
            class="text-sm text-gray-600 dark:text-dark-300"
          >
            {{ filePreviewResult.account.email }}
          </div>
          <div
            v-if="filePreviewResult.account.warnings?.length"
            class="rounded-lg bg-amber-50 p-3 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
          >
            <div
              v-for="warning in filePreviewResult.account.warnings"
              :key="warning"
            >
              {{ warning }}
            </div>
          </div>
        </div>

        <div
          v-if="filePreviewResult.existing_account"
          class="rounded-lg border border-blue-200 bg-blue-50 p-4 text-sm text-blue-800 dark:border-blue-800/50 dark:bg-blue-900/20 dark:text-blue-200"
        >
          <div class="font-medium">
            {{ t("admin.accounts.cpaImportExistingTitle") }}
          </div>
          <div class="mt-1">
            {{
              t("admin.accounts.cpaImportExistingDesc", {
                name: filePreviewResult.existing_account.name,
              })
            }}
          </div>
        </div>

        <div v-else class="space-y-4">
          <div>
            <label class="input-label">{{ t("admin.accounts.proxy") }}</label>
            <ProxySelector v-model="form.proxy_id" :proxies="proxies" />
          </div>

          <div>
            <label class="input-label">{{
              t("admin.accounts.cpaImportConcurrency")
            }}</label>
            <input
              v-model.number="form.concurrency"
              type="number"
              min="1"
              class="input"
              @input="
                form.concurrency = Math.max(
                  1,
                  form.concurrency || defaultConcurrency,
                )
              "
            />
          </div>

          <label
            class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-300"
          >
            <input
              v-model="form.use_default_group_bind"
              type="checkbox"
              class="rounded border-gray-300 dark:border-dark-600"
            />
            {{ t("admin.accounts.cpaUseDefaultGroupBind") }}
          </label>

          <div
            v-if="!form.use_default_group_bind"
            class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"
          >
            <div class="mb-3 text-sm text-gray-600 dark:text-dark-300">
              {{ t("admin.accounts.cpaManualGroupsHint") }}
            </div>
            <GroupSelector
              v-model="form.group_ids"
              :groups="groups"
              :platform="filePreviewPlatform"
            />
          </div>
        </div>
      </template>
    </div>

    <div v-else-if="currentStep === 'result' && result" class="space-y-4">
      <div
        class="space-y-2 rounded-xl border border-gray-200 p-4 dark:border-dark-700"
      >
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t("admin.accounts.cpaImportResult") }}
        </div>
        <div class="text-sm text-gray-700 dark:text-dark-300">
          {{ t("admin.accounts.cpaImportResultSummary", resultSummaryParams) }}
        </div>
        <div v-if="errorItems.length" class="mt-2">
          <div class="text-sm font-medium text-red-600 dark:text-red-400">
            {{ t("admin.accounts.cpaImportErrors") }}
          </div>
          <div
            class="mt-2 max-h-48 overflow-auto rounded-lg bg-gray-50 p-3 font-mono text-xs dark:bg-dark-800"
          >
            <div
              v-for="(item, idx) in errorItems"
              :key="idx"
              class="whitespace-pre-wrap"
            >
              {{ item.provider }} {{ item.name }} - {{ item.error }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <template v-if="currentStep === 'input'">
          <button
            class="btn btn-secondary"
            type="button"
            :disabled="previewing"
            @click="handleClose"
          >
            {{ t("common.cancel") }}
          </button>
          <button
            class="btn btn-primary"
            type="submit"
            form="import-from-cpa-preview-form"
            :disabled="previewing"
          >
            {{
              previewing
                ? t("admin.accounts.cpaImportPreviewing")
                : t("admin.accounts.cpaImportPreview")
            }}
          </button>
        </template>

        <template v-else-if="currentStep === 'preview'">
          <button
            class="btn btn-secondary"
            type="button"
            :disabled="importing"
            @click="handleBack"
          >
            {{ t("admin.accounts.crsBack") }}
          </button>
          <button
            class="btn btn-primary"
            type="button"
            :disabled="
              importing ||
              (importMode === 'remote' && remoteHasNewButNoneSelected)
            "
            @click="handleImport"
          >
            {{
              importing
                ? t("admin.accounts.cpaImporting")
                : t("admin.accounts.cpaImportButton")
            }}
          </button>
        </template>

        <template v-else-if="currentStep === 'result'">
          <button class="btn btn-secondary" type="button" @click="handleClose">
            {{ t("common.close") }}
          </button>
        </template>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import BaseDialog from "@/components/common/BaseDialog.vue";
import ProxySelector from "@/components/common/ProxySelector.vue";
import GroupSelector from "@/components/common/GroupSelector.vue";
import { adminAPI } from "@/api/admin";
import { useAppStore } from "@/stores/app";
import type {
  PreviewFromCPAResult,
  PreviewRemoteFromCPAResult,
  SyncFromCPAResult,
} from "@/api/admin/accounts";
import type { AdminGroup, Proxy, AccountPlatform } from "@/types";

interface Props {
  show: boolean;
  proxies: Proxy[];
  groups: AdminGroup[];
}

interface Emits {
  (e: "close"): void;
  (e: "imported"): void;
}

type Step = "input" | "preview" | "result";
type ImportMode = "remote" | "file";

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const { t } = useI18n();
const appStore = useAppStore();

const importMode = ref<ImportMode>("remote");
const currentStep = ref<Step>("input");
const previewing = ref(false);
const importing = ref(false);
const file = ref<File | null>(null);
const rawJSON = ref("");
const filePreviewResult = ref<PreviewFromCPAResult | null>(null);
const remotePreviewResult = ref<PreviewRemoteFromCPAResult | null>(null);
const result = ref<SyncFromCPAResult | null>(null);
const selectedRemoteSourceKeys = ref(new Set<string>());
const defaultConcurrency = ref(10);
const fileInput = ref<HTMLInputElement | null>(null);

const form = reactive({
  base_url: "",
  management_key: "",
  proxy_id: null as number | null,
  concurrency: 10,
  use_default_group_bind: true,
  group_ids: [] as number[],
});

const fileName = computed(() => file.value?.name || "");
const filePreviewPlatform = computed(
  () =>
    (filePreviewResult.value?.account.platform ||
      "anthropic") as AccountPlatform,
);
const remoteItems = computed(() => remotePreviewResult.value?.items || []);
const remoteExistingAccounts = computed(() =>
  remoteItems.value.filter((item) => item.existing_account),
);
const remoteNewAccounts = computed(() =>
  remoteItems.value.filter((item) => !item.existing_account),
);
const remoteSelectedNewAccounts = computed(() =>
  remoteNewAccounts.value.filter((item) =>
    selectedRemoteSourceKeys.value.has(item.account.cpa_source_key),
  ),
);
const remoteSelectedPlatform = computed<AccountPlatform | undefined>(() => {
  const platforms = [
    ...new Set(
      remoteSelectedNewAccounts.value.map(
        (item) => item.account.platform as AccountPlatform,
      ),
    ),
  ];
  return platforms.length === 1 ? platforms[0] : undefined;
});
const remoteHasNewButNoneSelected = computed(
  () =>
    remoteNewAccounts.value.length > 0 &&
    selectedRemoteSourceKeys.value.size === 0,
);
const errorItems = computed(
  () => result.value?.items.filter((item) => item.action === "failed") || [],
);
const resultSummaryParams = computed(() =>
  result.value
    ? {
        created: result.value.created,
        updated: result.value.updated,
        failed: result.value.failed,
      }
    : {},
);
const remoteSummaryParams = computed(() =>
  remotePreviewResult.value
    ? {
        total: remotePreviewResult.value.total,
        importable: remotePreviewResult.value.importable,
        skipped_non_normal: remotePreviewResult.value.skipped_non_normal,
        skipped_unsupported: remotePreviewResult.value.skipped_unsupported,
      }
    : {},
);

watch(
  () => props.show,
  async (open) => {
    if (!open) return;
    resetState();
    try {
      const settings = await adminAPI.settings.getSettings();
      if (settings.default_concurrency > 0) {
        defaultConcurrency.value = settings.default_concurrency;
        form.concurrency = settings.default_concurrency;
      }
    } catch {
      form.concurrency = defaultConcurrency.value;
    }
  },
);

watch(
  () => form.use_default_group_bind,
  (enabled) => {
    if (enabled) {
      form.group_ids = [];
    }
  },
);

watch(remoteSelectedPlatform, () => {
  form.group_ids = [];
});

const resetState = () => {
  importMode.value = "remote";
  currentStep.value = "input";
  file.value = null;
  rawJSON.value = "";
  filePreviewResult.value = null;
  remotePreviewResult.value = null;
  result.value = null;
  selectedRemoteSourceKeys.value = new Set();
  form.base_url = "";
  form.management_key = "";
  form.proxy_id = null;
  form.concurrency = defaultConcurrency.value;
  form.use_default_group_bind = true;
  form.group_ids = [];
  if (fileInput.value) {
    fileInput.value.value = "";
  }
};

const setImportMode = (mode: ImportMode) => {
  if (previewing.value || importing.value) return;
  importMode.value = mode;
  currentStep.value = "input";
  filePreviewResult.value = null;
  remotePreviewResult.value = null;
  result.value = null;
  selectedRemoteSourceKeys.value = new Set();
  form.group_ids = [];
  form.use_default_group_bind = true;
};

const openFilePicker = () => {
  fileInput.value?.click();
};

const handleFileChange = (event: Event) => {
  const target = event.target as HTMLInputElement;
  file.value = target.files?.[0] || null;
};

const handleClose = () => {
  if (previewing.value || importing.value) return;
  emit("close");
};

const handleBack = () => {
  currentStep.value = "input";
  filePreviewResult.value = null;
  remotePreviewResult.value = null;
  result.value = null;
  selectedRemoteSourceKeys.value = new Set();
  form.group_ids = [];
};

const selectAllRemote = () => {
  selectedRemoteSourceKeys.value = new Set(
    remoteNewAccounts.value.map((item) => item.account.cpa_source_key),
  );
};

const selectNoneRemote = () => {
  selectedRemoteSourceKeys.value = new Set();
};

const toggleRemoteSelect = (sourceKey: string) => {
  const next = new Set(selectedRemoteSourceKeys.value);
  if (next.has(sourceKey)) {
    next.delete(sourceKey);
  } else {
    next.add(sourceKey);
  }
  selectedRemoteSourceKeys.value = next;
};

const readFileAsText = async (sourceFile: File): Promise<string> => {
  if (typeof sourceFile.text === "function") {
    return sourceFile.text();
  }

  if (typeof sourceFile.arrayBuffer === "function") {
    const buffer = await sourceFile.arrayBuffer();
    return new TextDecoder().decode(buffer);
  }

  return await new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result ?? ""));
    reader.onerror = () =>
      reject(reader.error || new Error("Failed to read file"));
    reader.readAsText(sourceFile);
  });
};

const handlePreview = async () => {
  previewing.value = true;
  try {
    form.group_ids = [];
    form.use_default_group_bind = true;

    if (importMode.value === "remote") {
      if (!form.base_url.trim() || !form.management_key.trim()) {
        appStore.showError(t("admin.accounts.cpaRemoteMissingFields"));
        return;
      }
      const res = await adminAPI.accounts.previewRemoteFromCpa({
        base_url: form.base_url.trim(),
        management_key: form.management_key.trim(),
      });
      remotePreviewResult.value = res;
      selectedRemoteSourceKeys.value = new Set(
        res.items
          .filter((item) => !item.existing_account)
          .map((item) => item.account.cpa_source_key),
      );
      currentStep.value = "preview";
      return;
    }

    if (!file.value) {
      appStore.showError(t("admin.accounts.cpaImportSelectFile"));
      return;
    }

    const text = await readFileAsText(file.value);
    rawJSON.value = text;
    const res = await adminAPI.accounts.previewFromCpa({
      file_name: file.value.name,
      raw_json: text,
    });
    filePreviewResult.value = res;
    currentStep.value = "preview";
  } catch (error: any) {
    appStore.showError(
      error?.message || t("admin.accounts.cpaImportPreviewFailed"),
    );
  } finally {
    previewing.value = false;
  }
};

const handleImport = async () => {
  importing.value = true;
  try {
    if (importMode.value === "remote") {
      if (!form.base_url.trim() || !form.management_key.trim()) {
        appStore.showError(t("admin.accounts.cpaRemoteMissingFields"));
        return;
      }

      const hasRemoteNewAccounts = remoteNewAccounts.value.length > 0;
      const res = await adminAPI.accounts.importRemoteFromCpa({
        base_url: form.base_url.trim(),
        management_key: form.management_key.trim(),
        selected_source_keys: hasRemoteNewAccounts
          ? [...selectedRemoteSourceKeys.value]
          : undefined,
        proxy_id:
          remoteSelectedNewAccounts.value.length > 0
            ? form.proxy_id
            : undefined,
        concurrency:
          remoteSelectedNewAccounts.value.length > 0
            ? Math.max(1, form.concurrency || defaultConcurrency.value)
            : undefined,
        use_default_group_bind:
          remoteSelectedNewAccounts.value.length > 0
            ? form.use_default_group_bind
            : true,
        group_ids:
          !form.use_default_group_bind && remoteSelectedPlatform.value
            ? form.group_ids
            : [],
      });
      result.value = res;
    } else {
      if (!file.value || !rawJSON.value) {
        appStore.showError(t("admin.accounts.cpaImportSelectFile"));
        return;
      }

      const res = await adminAPI.accounts.importFromCpa({
        file_name: file.value.name,
        raw_json: rawJSON.value,
        proxy_id: filePreviewResult.value?.existing_account
          ? undefined
          : form.proxy_id,
        concurrency: filePreviewResult.value?.existing_account
          ? undefined
          : Math.max(1, form.concurrency || defaultConcurrency.value),
        use_default_group_bind: filePreviewResult.value?.existing_account
          ? true
          : form.use_default_group_bind,
        group_ids:
          filePreviewResult.value?.existing_account ||
          form.use_default_group_bind
            ? []
            : form.group_ids,
      });
      result.value = res;
    }

    currentStep.value = "result";

    if (
      result.value &&
      (result.value.created > 0 || result.value.updated > 0)
    ) {
      emit("imported");
    }
    if (result.value && result.value.failed > 0) {
      appStore.showError(
        t("admin.accounts.cpaImportCompletedWithErrors", {
          created: result.value.created,
          updated: result.value.updated,
          failed: result.value.failed,
        }),
      );
    } else if (result.value) {
      appStore.showSuccess(
        t("admin.accounts.cpaImportSuccess", {
          created: result.value.created,
          updated: result.value.updated,
          failed: result.value.failed,
        }),
      );
    }
  } catch (error: any) {
    appStore.showError(error?.message || t("admin.accounts.cpaImportFailed"));
  } finally {
    importing.value = false;
  }
};
</script>
