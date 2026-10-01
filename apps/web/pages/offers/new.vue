<script setup lang="ts">
definePageMeta({ requires: 'offer.create' })
useHead({ title: 'New offer' })
</script>

<template>
  <div>
    <PageHeader title="New offer" back="/offers" back-label="Back to offers" />
    <OfferForm @done="(p) => navigateTo(p)" />
  </div>
</template>
