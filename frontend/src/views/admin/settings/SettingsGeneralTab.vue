<template>
  <!-- eslint-disable vue/no-mutating-props -->
  <div class="space-y-6">
    <!-- Site Settings -->
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.site.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.site.description') }}
        </p>
      </div>
      <div class="space-y-6 p-6">
        <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.site.siteName') }}
            </label>
            <input
              v-model="form.site_name"
              type="text"
              class="input"
              :placeholder="t('admin.settings.site.siteNamePlaceholder')"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.site.siteNameHint') }}
            </p>
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.site.siteSubtitle') }}
            </label>
            <input
              v-model="form.site_subtitle"
              type="text"
              class="input"
              :placeholder="t('admin.settings.site.siteSubtitlePlaceholder')"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.site.siteSubtitleHint') }}
            </p>
          </div>
        </div>

        <!-- API Base URL -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.apiBaseUrl') }}
          </label>
          <input
            v-model="form.api_base_url"
            type="text"
            class="input font-mono text-sm"
            :placeholder="t('admin.settings.site.apiBaseUrlPlaceholder')"
          />
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.site.apiBaseUrlHint') }}
          </p>
        </div>

        <!-- Contact Info -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.contactInfo') }}
          </label>
          <input
            v-model="form.contact_info"
            type="text"
            class="input"
            :placeholder="t('admin.settings.site.contactInfoPlaceholder')"
          />
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.site.contactInfoHint') }}
          </p>
        </div>

        <!-- Contact QR Codes -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.contactQRCode') }}
          </label>
          <p class="mb-4 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.site.contactQRCodeHint') }}
          </p>
          <div class="grid grid-cols-1 gap-6 sm:grid-cols-2">
            <!-- WeChat QR Code -->
            <div class="flex items-start gap-4">
              <div class="flex-shrink-0">
                <div
                  class="flex h-24 w-24 items-center justify-center overflow-hidden rounded-xl border-2 border-dashed border-gray-300 bg-gray-50 dark:border-dark-600 dark:bg-dark-800"
                  :class="{ 'border-solid': form.contact_qrcode_wechat }"
                >
                  <img
                    v-if="form.contact_qrcode_wechat"
                    :src="form.contact_qrcode_wechat"
                    alt="WeChat QR Code"
                    class="h-full w-full object-contain"
                  />
                  <svg
                    v-else
                    class="h-8 w-8 text-gray-400 dark:text-dark-500"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.5"
                      d="M12 4v1m6 11h2m-6 0h-2v4m0-11v3m0 0h.01M12 12h4.01M16 20h4M4 12h4m12 0h.01M5 8h2a1 1 0 001-1V5a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1zm12 0h2a1 1 0 001-1V5a1 1 0 00-1-1h-2a1 1 0 00-1 1v2a1 1 0 001 1zM5 20h2a1 1 0 001-1v-2a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1z"
                    />
                  </svg>
                </div>
              </div>
              <div class="flex-1 space-y-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.site.qrcodeWechat') }}
                </span>
                <div class="flex flex-wrap items-center gap-2">
                  <label class="btn btn-secondary btn-sm cursor-pointer">
                    <input
                      type="file"
                      accept="image/*"
                      class="hidden"
                      @change="handleQRCodeUpload($event, 'wechat')"
                    />
                    <Icon name="upload" size="sm" class="mr-1.5" :stroke-width="2" />
                    {{ t('admin.settings.site.uploadImage') }}
                  </label>
                  <button
                    v-if="form.contact_qrcode_wechat"
                    type="button"
                    @click="form.contact_qrcode_wechat = ''"
                    class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                  >
                    <Icon name="trash" size="sm" class="mr-1.5" :stroke-width="2" />
                    {{ t('admin.settings.site.remove') }}
                  </button>
                </div>
              </div>
            </div>

            <!-- Group QR Code -->
            <div class="flex items-start gap-4">
              <div class="flex-shrink-0">
                <div
                  class="flex h-24 w-24 items-center justify-center overflow-hidden rounded-xl border-2 border-dashed border-gray-300 bg-gray-50 dark:border-dark-600 dark:bg-dark-800"
                  :class="{ 'border-solid': form.contact_qrcode_group }"
                >
                  <img
                    v-if="form.contact_qrcode_group"
                    :src="form.contact_qrcode_group"
                    alt="Group QR Code"
                    class="h-full w-full object-contain"
                  />
                  <svg
                    v-else
                    class="h-8 w-8 text-gray-400 dark:text-dark-500"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.5"
                      d="M12 4v1m6 11h2m-6 0h-2v4m0-11v3m0 0h.01M12 12h4.01M16 20h4M4 12h4m12 0h.01M5 8h2a1 1 0 001-1V5a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1zm12 0h2a1 1 0 001-1V5a1 1 0 00-1-1h-2a1 1 0 00-1 1v2a1 1 0 001 1zM5 20h2a1 1 0 001-1v-2a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1z"
                    />
                  </svg>
                </div>
              </div>
              <div class="flex-1 space-y-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.site.qrcodeGroup') }}
                </span>
                <div class="flex flex-wrap items-center gap-2">
                  <label class="btn btn-secondary btn-sm cursor-pointer">
                    <input
                      type="file"
                      accept="image/*"
                      class="hidden"
                      @change="handleQRCodeUpload($event, 'group')"
                    />
                    <Icon name="upload" size="sm" class="mr-1.5" :stroke-width="2" />
                    {{ t('admin.settings.site.uploadImage') }}
                  </label>
                  <button
                    v-if="form.contact_qrcode_group"
                    type="button"
                    @click="form.contact_qrcode_group = ''"
                    class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                  >
                    <Icon name="trash" size="sm" class="mr-1.5" :stroke-width="2" />
                    {{ t('admin.settings.site.remove') }}
                  </button>
                </div>
              </div>
            </div>
          </div>
          <p v-if="qrcodeError" class="mt-2 text-xs text-red-500">{{ qrcodeError }}</p>
        </div>

        <!-- Doc URL -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.docUrl') }}
          </label>
          <input
            v-model="form.doc_url"
            type="url"
            class="input font-mono text-sm"
            :placeholder="t('admin.settings.site.docUrlPlaceholder')"
          />
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.site.docUrlHint') }}
          </p>
        </div>

        <!-- Site Logo Upload -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.siteLogo') }}
          </label>
          <div class="flex items-start gap-6">
            <div class="flex-shrink-0">
              <div
                class="flex h-20 w-20 items-center justify-center overflow-hidden rounded-xl border-2 border-dashed border-gray-300 bg-gray-50 dark:border-dark-600 dark:bg-dark-800"
                :class="{ 'border-solid': form.site_logo }"
              >
                <img
                  v-if="form.site_logo"
                  :src="form.site_logo"
                  alt="Site Logo"
                  class="h-full w-full object-contain"
                />
                <svg
                  v-else
                  class="h-8 w-8 text-gray-400 dark:text-dark-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="1.5"
                    d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
                  />
                </svg>
              </div>
            </div>
            <div class="flex-1 space-y-3">
              <div class="flex items-center gap-3">
                <label class="btn btn-secondary btn-sm cursor-pointer">
                  <input
                    type="file"
                    accept="image/*"
                    class="hidden"
                    @change="(e) => handleLogoUpload(e, 'light')"
                  />
                  <Icon name="upload" size="sm" class="mr-1.5" :stroke-width="2" />
                  {{ t('admin.settings.site.uploadImage') }}
                </label>
                <button
                  v-if="form.site_logo"
                  type="button"
                  @click="form.site_logo = ''"
                  class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                >
                  <Icon name="trash" size="sm" class="mr-1.5" :stroke-width="2" />
                  {{ t('admin.settings.site.remove') }}
                </button>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.site.logoHint') }}
              </p>
              <p v-if="logoError" class="text-xs text-red-500">{{ logoError }}</p>
            </div>
          </div>
        </div>

        <!-- Site Logo Dark Upload -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.siteLogoDark') }}
          </label>
          <div class="flex items-start gap-6">
            <div class="flex-shrink-0">
              <div
                class="flex h-20 w-20 items-center justify-center overflow-hidden rounded-xl border-2 border-dashed border-gray-600 bg-gray-900"
                :class="{ 'border-solid': form.site_logo_dark }"
              >
                <img
                  v-if="form.site_logo_dark"
                  :src="form.site_logo_dark"
                  alt="Site Logo Dark"
                  class="h-full w-full object-contain"
                />
                <svg
                  v-else
                  class="h-8 w-8 text-gray-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="1.5"
                    d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
                  />
                </svg>
              </div>
            </div>
            <div class="flex-1 space-y-3">
              <div class="flex items-center gap-3">
                <label class="btn btn-secondary btn-sm cursor-pointer">
                  <input
                    type="file"
                    accept="image/*"
                    class="hidden"
                    @change="(e) => handleLogoUpload(e, 'dark')"
                  />
                  <Icon name="upload" size="sm" class="mr-1.5" :stroke-width="2" />
                  {{ t('admin.settings.site.uploadImage') }}
                </label>
                <button
                  v-if="form.site_logo_dark"
                  type="button"
                  @click="form.site_logo_dark = ''"
                  class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                >
                  <Icon name="trash" size="sm" class="mr-1.5" :stroke-width="2" />
                  {{ t('admin.settings.site.remove') }}
                </button>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.site.logoDarkHint') }}
              </p>
              <p v-if="logoDarkError" class="text-xs text-red-500">{{ logoDarkError }}</p>
            </div>
          </div>
        </div>

        <!-- Home Content -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.homeContent') }}
          </label>
          <textarea
            v-model="form.home_content"
            rows="6"
            class="input font-mono text-sm"
            :placeholder="t('admin.settings.site.homeContentPlaceholder')"
          ></textarea>
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.site.homeContentHint') }}
          </p>
          <p class="mt-2 text-xs text-amber-600 dark:text-amber-400">
            {{ t('admin.settings.site.homeContentIframeWarning') }}
          </p>
        </div>

        <!-- Install Guide Videos -->
        <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.installGuideVideos.title') }}
          </label>
          <p class="mb-3 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.installGuideVideos.description') }}
          </p>
          <div class="space-y-3">
            <div v-for="toolKey in ['claude_code', 'codex', 'gemini_cli']" :key="toolKey">
              <details class="rounded-lg border border-gray-200 dark:border-dark-700">
                <summary class="cursor-pointer px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-800">
                  {{ { claude_code: 'Claude Code', codex: 'Codex CLI', gemini_cli: 'Gemini CLI' }[toolKey] }}
                </summary>
                <div class="space-y-2 border-t border-gray-100 px-3 py-3 dark:border-dark-700">
                  <div>
                    <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.settings.installGuideVideos.overview') }}
                    </label>
                    <input
                      type="text"
                      :value="getVideoField(toolKey, 'overview')"
                      @input="setVideoField(toolKey, 'overview', ($event.target as HTMLInputElement).value)"
                      class="input text-sm"
                      :placeholder="t('admin.settings.installGuideVideos.urlPlaceholder')"
                    />
                  </div>
                </div>
              </details>
            </div>
          </div>
        </div>

        <!-- Home Testimonials -->
        <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.homeTestimonials.title') }}
          </label>
          <p class="mb-2 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.homeTestimonials.description') }}
          </p>
          <textarea
            v-model="form.home_testimonials"
            rows="4"
            class="input font-mono text-sm"
            :placeholder="t('admin.settings.homeTestimonials.placeholder')"
          ></textarea>
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.homeTestimonials.hint') }}
          </p>
        </div>

        <!-- Hide CCS Import Button -->
        <div
          class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
        >
          <div>
            <label class="font-medium text-gray-900 dark:text-white">{{
              t('admin.settings.site.hideCcsImportButton')
            }}</label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.site.hideCcsImportButtonHint') }}
            </p>
          </div>
          <Toggle v-model="form.hide_ccs_import_button" />
        </div>
      </div>
    </div>

    <!-- Home Gallery Management -->
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <div class="flex items-center justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.gallery.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.gallery.description') }}
            </p>
          </div>
          <Toggle v-model="gallery.enabled" />
        </div>
      </div>
      <div v-if="gallery.enabled" class="space-y-6 p-6">
        <!-- Title & Subtitle -->
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.gallery.sectionTitle') }}
            </label>
            <input v-model="gallery.title" type="text" class="input" />
          </div>
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.gallery.sectionSubtitle') }}
            </label>
            <input v-model="gallery.subtitle" type="text" class="input" />
          </div>
        </div>

        <!-- Categories -->
        <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.gallery.categories') }}
          </label>
          <div class="space-y-2">
            <div v-for="(cat, idx) in gallery.categories" :key="idx" class="flex items-center gap-2">
              <input
                :value="cat.key"
                type="text"
                class="input w-32 text-sm"
                placeholder="key"
                @change="renameCategoryKey(idx, ($event.target as HTMLInputElement).value)"
              />
              <input v-model="cat.label" type="text" class="input flex-1 text-sm" placeholder="label" />
              <button
                type="button"
                class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                @click="removeCategory(idx)"
              >
                <Icon name="trash" size="sm" :stroke-width="2" />
              </button>
            </div>
          </div>
          <button
            type="button"
            class="btn btn-secondary btn-sm mt-2"
            @click="gallery.categories.push({ key: '', label: '' })"
          >
            {{ t('admin.settings.gallery.addCategory') }}
          </button>
        </div>

        <!-- Category Tabs for Images -->
        <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.gallery.images') }}
          </label>

          <!-- Tab selector -->
          <div v-if="gallery.categories.length > 0" class="mb-4 flex flex-wrap gap-2">
            <button
              v-for="cat in gallery.categories"
              :key="cat.key"
              class="rounded-full px-3 py-1 text-xs font-medium transition-colors"
              :class="galleryActiveTab === cat.key
                ? 'bg-primary-500 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-dark-300'"
              @click="galleryActiveTab = cat.key"
            >
              {{ cat.label || cat.key }} ({{ galleryItemsByCategory(cat.key).length }})
            </button>
          </div>

          <!-- Image Grid -->
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
            <div
              v-for="item in galleryItemsByCategory(galleryActiveTab)"
              :key="item.id"
              class="group relative overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700"
            >
              <img :src="item.image" :alt="item.title" class="aspect-[3/2] w-full object-cover" />
              <div class="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
                <button
                  type="button"
                  class="rounded-full bg-red-500 p-1.5 text-white shadow hover:bg-red-600"
                  @click="removeGalleryItem(item.id)"
                >
                  <Icon name="trash" size="sm" :stroke-width="2" />
                </button>
              </div>
              <div class="p-1.5">
                <input
                  v-model="item.title"
                  class="w-full border-0 bg-transparent p-0 text-xs text-gray-600 focus:ring-0 dark:text-dark-300"
                  :placeholder="t('admin.settings.gallery.imageTitle')"
                />
              </div>
            </div>

            <!-- Upload Card -->
            <label class="flex aspect-[3/2] cursor-pointer items-center justify-center rounded-lg border-2 border-dashed border-gray-300 bg-gray-50 transition-colors hover:border-primary-400 hover:bg-primary-50 dark:border-dark-600 dark:bg-dark-800 dark:hover:border-primary-500 dark:hover:bg-dark-700">
              <input
                type="file"
                accept="image/*"
                multiple
                class="hidden"
                @change="handleGalleryImageUpload"
              />
              <div class="text-center">
                <Icon name="upload" size="lg" class="mx-auto mb-1 text-gray-400" :stroke-width="1.5" />
                <span class="text-xs text-gray-500 dark:text-dark-400">
                  {{ t('admin.settings.gallery.upload') }}
                </span>
              </div>
            </label>
          </div>

          <p v-if="galleryError" class="mt-2 text-xs text-red-500">{{ galleryError }}</p>
          <p class="mt-2 text-xs text-gray-400 dark:text-dark-500">
            {{ t('admin.settings.gallery.uploadHint') }}
          </p>
        </div>

        <!-- Save Button -->
        <div class="flex justify-end border-t border-gray-100 pt-4 dark:border-dark-700">
          <button
            type="button"
            class="btn btn-primary"
            :disabled="gallerySaving"
            @click="saveGallery"
          >
            <span v-if="gallerySaving" class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
            {{ t('admin.settings.gallery.save') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Default Settings -->
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.defaults.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.defaults.description') }}
        </p>
      </div>
      <div class="p-6">
        <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.defaults.defaultBalance') }}
            </label>
            <input
              v-model.number="form.default_balance"
              type="number"
              step="0.01"
              min="0"
              class="input"
              placeholder="0.00"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.defaults.defaultBalanceHint') }}
            </p>
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.defaults.defaultConcurrency') }}
            </label>
            <input
              v-model.number="form.default_concurrency"
              type="number"
              min="1"
              class="input"
              placeholder="1"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.defaults.defaultConcurrencyHint') }}
            </p>
          </div>
        </div>
      </div>
    </div>

    <!-- Purchase Subscription Page -->
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.purchase.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.purchase.description') }}
        </p>
      </div>
      <div class="space-y-6 p-6">
        <div class="flex items-center justify-between">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">{{
              t('admin.settings.purchase.enabled')
            }}</label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.purchase.enabledHint') }}
            </p>
          </div>
          <Toggle v-model="form.purchase_subscription_enabled" />
        </div>
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.purchase.url') }}
          </label>
          <input
            v-model="form.purchase_subscription_url"
            type="url"
            class="input font-mono text-sm"
            :placeholder="t('admin.settings.purchase.urlPlaceholder')"
          />
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.purchase.urlHint') }}
          </p>
          <p class="mt-2 text-xs text-amber-600 dark:text-amber-400">
            {{ t('admin.settings.purchase.iframeWarning') }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/* eslint-disable vue/no-mutating-props */
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { settingsAPI } from '@/api/admin/settings'
import type { GalleryData } from '@/api/admin/settings'
import { useImageCompress } from '@/composables/useImageCompress'
import { useAppStore } from '@/stores'
import type { SettingsForm } from './types'

const { t } = useI18n()
const appStore = useAppStore()
const { compressImage } = useImageCompress()

const props = defineProps<{
  form: SettingsForm
}>()

const logoError = ref('')
const logoDarkError = ref('')
const qrcodeError = ref('')

// ==================== Gallery Management (independent state) ====================
const gallery = reactive<GalleryData>({
  enabled: false,
  title: '',
  subtitle: '',
  categories: [],
  items: [],
})
const galleryActiveTab = ref('')
const galleryError = ref('')
const gallerySaving = ref(false)
const galleryLoaded = ref(false)

function galleryItemsByCategory(catKey: string) {
  if (!catKey) return gallery.items
  return gallery.items.filter(item => item.category === catKey)
}

function removeGalleryItem(id: string) {
  const idx = gallery.items.findIndex(item => item.id === id)
  if (idx >= 0) gallery.items.splice(idx, 1)
}

function renameCategoryKey(idx: number, newKey: string) {
  const cat = gallery.categories[idx]
  if (!cat) return
  const oldKey = cat.key
  cat.key = newKey
  // Sync all items that belonged to the old key
  if (oldKey && oldKey !== newKey) {
    gallery.items.forEach(item => {
      if (item.category === oldKey) item.category = newKey
    })
  }
  // Update active tab if it was tracking the old key
  if (galleryActiveTab.value === oldKey) {
    galleryActiveTab.value = newKey
  }
}

function removeCategory(idx: number) {
  const cat = gallery.categories[idx]
  if (!cat) return
  // Remove all items belonging to this category
  gallery.items = gallery.items.filter(item => item.category !== cat.key)
  gallery.categories.splice(idx, 1)
  // Reset active tab if the deleted category was active
  if (galleryActiveTab.value === cat.key) {
    galleryActiveTab.value = gallery.categories[0]?.key || ''
  }
}

async function handleGalleryImageUpload(event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files
  galleryError.value = ''

  if (!files || files.length === 0) return

  const category = galleryActiveTab.value || gallery.categories[0]?.key || ''
  if (!category) {
    galleryError.value = t('admin.settings.gallery.noCategoryError')
    input.value = ''
    return
  }

  for (const file of Array.from(files)) {
    if (!file.type.startsWith('image/')) continue

    try {
      const dataUrl = await compressImage(file, 1200, 500)
      gallery.items.push({
        id: `img_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`,
        category,
        title: file.name.replace(/\.[^.]+$/, ''),
        image: dataUrl,
        order: gallery.items.length,
      })
    } catch (err) {
      galleryError.value = `${file.name}: ${err instanceof Error ? err.message : 'Failed'}`
    }
  }

  input.value = ''
}

async function loadGallery() {
  try {
    const data = await settingsAPI.getHomeGallery()
    if (data) {
      gallery.enabled = data.enabled
      gallery.title = data.title || ''
      gallery.subtitle = data.subtitle || ''
      gallery.categories = data.categories || []
      gallery.items = data.items || []
      if (gallery.categories.length > 0) {
        galleryActiveTab.value = gallery.categories[0].key
      }
    }
    galleryLoaded.value = true
  } catch {
    // Not configured yet, keep defaults
    galleryLoaded.value = true
  }
}

async function saveGallery() {
  gallerySaving.value = true
  galleryError.value = ''

  try {
    // Filter out categories with empty keys
    gallery.categories = gallery.categories.filter(c => c.key.trim() !== '')

    await settingsAPI.updateHomeGallery({
      enabled: gallery.enabled,
      title: gallery.title,
      subtitle: gallery.subtitle,
      categories: gallery.categories,
      items: gallery.items,
    })
    appStore.showToast('success', t('admin.settings.gallery.saveSuccess'))
  } catch (err: any) {
    galleryError.value = err?.response?.data?.message || err?.message || 'Save failed'
  } finally {
    gallerySaving.value = false
  }
}

onMounted(() => {
  loadGallery()
})

function handleLogoUpload(event: Event, type: 'light' | 'dark' = 'light') {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  const errorRef = type === 'light' ? logoError : logoDarkError
  errorRef.value = ''

  if (!file) return

  const maxSize = 300 * 1024
  if (file.size > maxSize) {
    errorRef.value = t('admin.settings.site.logoSizeError', {
      size: (file.size / 1024).toFixed(1)
    })
    input.value = ''
    return
  }

  if (!file.type.startsWith('image/')) {
    errorRef.value = t('admin.settings.site.logoTypeError')
    input.value = ''
    return
  }

  const reader = new FileReader()
  reader.onload = (e) => {
    if (type === 'light') {
      props.form.site_logo = e.target?.result as string
    } else {
      props.form.site_logo_dark = e.target?.result as string
    }
  }
  reader.onerror = () => {
    errorRef.value = t('admin.settings.site.logoReadError')
  }
  reader.readAsDataURL(file)
  input.value = ''
}

function handleQRCodeUpload(event: Event, type: 'wechat' | 'group') {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  qrcodeError.value = ''

  if (!file) return

  const maxSize = 500 * 1024
  if (file.size > maxSize) {
    qrcodeError.value = t('admin.settings.site.qrcodeSizeError', {
      size: (file.size / 1024).toFixed(1)
    })
    input.value = ''
    return
  }

  if (!file.type.startsWith('image/')) {
    qrcodeError.value = t('admin.settings.site.qrcodeTypeError')
    input.value = ''
    return
  }

  const reader = new FileReader()
  reader.onload = (e) => {
    if (type === 'wechat') {
      props.form.contact_qrcode_wechat = e.target?.result as string
    } else {
      props.form.contact_qrcode_group = e.target?.result as string
    }
  }
  reader.onerror = () => {
    qrcodeError.value = t('admin.settings.site.qrcodeReadError')
  }
  reader.readAsDataURL(file)
  input.value = ''
}

function parseVideoConfig(): Record<string, Record<string, string>> {
  try {
    return props.form.install_guide_videos ? JSON.parse(props.form.install_guide_videos) : {}
  } catch {
    return {}
  }
}

function getVideoField(toolKey: string, field: string): string {
  const config = parseVideoConfig()
  return config[toolKey]?.[field] || ''
}

function setVideoField(toolKey: string, field: string, value: string) {
  const config = parseVideoConfig()
  if (!config[toolKey]) config[toolKey] = {}
  config[toolKey][field] = value
  props.form.install_guide_videos = JSON.stringify(config)
}
</script>
