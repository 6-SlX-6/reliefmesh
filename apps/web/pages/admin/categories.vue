<script setup lang="ts">
import type { Category } from '@reliefmesh/shared-types'

definePageMeta({ layout: 'admin', requires: 'categories.manage' })
useHead({ title: 'Categories' })
const toasts = useToastStore()
const items = ref<Category[]>([])
onMounted(async () => { items.value = (await useApi().admin.categories()).items })

async function save(c: Category) {
  try {
    await useApi().admin.updateCategory(c.code, { label: c.label, description: c.description, enabled: c.enabled, sort_order: c.sort_order })
    toasts.success(`${c.label} saved.`)
    await useSettingsStore().load()
  } catch {
    toasts.error('Could not save category.')
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="Categories" />
    <p class="text-ink-muted">Category codes are fixed in this version. Disabled categories cannot be chosen for new requests; existing records keep them.</p>
    <ul class="flex flex-col gap-3">
      <li v-for="c in items" :key="c.code" class="card grid grid-cols-1 gap-3 sm:grid-cols-6 sm:items-end">
        <FormField v-slot="f" :label="`Label (${c.code})`" class="sm:col-span-2"><input :id="f.id" v-model="c.label" class="field-input" maxlength="60"></FormField>
        <FormField v-slot="f" label="Description" class="sm:col-span-2"><input :id="f.id" v-model="c.description" class="field-input" maxlength="300"></FormField>
        <FormField v-slot="f" label="Order"><input :id="f.id" v-model.number="c.sort_order" class="field-input" type="number" min="0"></FormField>
        <div class="flex flex-col gap-2">
          <label class="flex items-center gap-2 font-normal"><input v-model="c.enabled" type="checkbox" class="h-5 w-5"> Enabled</label>
          <button type="button" class="btn-secondary btn-sm" @click="save(c)">Save</button>
        </div>
        <p v-if="c.is_sensitive" class="text-sm text-ink-muted sm:col-span-6"><AppIcon name="lock" :size="14" class="inline" /> Sensitive category: extra privacy rules apply.</p>
      </li>
    </ul>
  </div>
</template>
