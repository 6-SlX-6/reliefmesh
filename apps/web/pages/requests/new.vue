<script setup lang="ts">
definePageMeta({ requires: 'request.create' })
useHead({ title: 'New request' })
</script>

<template>
  <div>
    <PageHeader title="New request" back="/requests" back-label="Back to requests" />
    <RequestForm @done="(p) => navigateTo(p)" />
  </div>
</template>
