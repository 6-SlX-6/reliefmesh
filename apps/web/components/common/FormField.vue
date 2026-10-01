<script setup lang="ts">
// Label + help + error wrapper. The slot receives the ids to wire up
// aria-describedby and aria-invalid on the actual control.
const props = withDefaults(defineProps<{ label: string; help?: string; error?: string; required?: boolean; id?: string }>(), {
  help: undefined, error: undefined, required: false, id: undefined,
})
const fieldId = props.id ?? `f-${Math.random().toString(36).slice(2, 9)}`
const helpId = `${fieldId}-help`
const errorId = `${fieldId}-error`
const invalid = computed<'true' | undefined>(() => (props.error ? 'true' : undefined))
const describedBy = computed(() => [props.help ? helpId : '', props.error ? errorId : ''].filter(Boolean).join(' ') || undefined)
</script>

<template>
  <div class="flex flex-col gap-1">
    <label :for="fieldId">
      {{ label }}
      <span v-if="required" class="text-danger" aria-hidden="true">*</span>
      <span v-if="required" class="sr-only">(required)</span>
    </label>
    <slot :id="fieldId" :described-by="describedBy" :invalid="invalid" />
    <p v-if="help" :id="helpId" class="field-help">{{ help }}</p>
    <p v-if="error" :id="errorId" class="field-error" role="alert">{{ error }}</p>
  </div>
</template>
