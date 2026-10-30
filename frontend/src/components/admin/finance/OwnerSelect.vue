<template>
  <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
    <span class="whitespace-nowrap">{{ t('admin.finance.ownerSelect.label') }}</span>
    <select class="input w-56" :value="modelValue ?? ''" data-testid="owner-select" @change="onChange">
      <option value="">{{ t('admin.finance.ownerSelect.all') }}</option>
      <option v-for="owner in owners" :key="owner.id" :value="String(owner.id)">
        {{ owner.username ? `${owner.email} (${owner.username})` : owner.email }}
      </option>
    </select>
  </label>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { AdminUser } from '@/types'

// Admin-only control: AI account owners never see other owners.
defineProps<{ modelValue: number | null }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: number | null): void }>()

const { t } = useI18n()
const appStore = useAppStore()
const owners = ref<Pick<AdminUser, 'id' | 'email' | 'username'>[]>([])

function onChange(event: Event) {
  const value = (event.target as HTMLSelectElement).value
  emit('update:modelValue', value ? Number(value) : null)
}

onMounted(async () => {
  try {
    const res = await adminAPI.users.list(1, 1000, { role: 'owner' })
    owners.value = res?.items ?? []
  } catch {
    appStore.showError(t('admin.finance.ownerSelect.loadFailed'))
  }
})
</script>
