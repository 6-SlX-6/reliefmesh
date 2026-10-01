<script setup lang="ts">
definePageMeta({ layout: false })
useHead({ title: 'About and emergency information' })
const auth = useAuthStore()
const settings = useSettingsStore()
</script>

<template>
  <NuxtLayout :name="auth.status === 'authenticated' ? 'default' : 'auth'">
    <article class="flex flex-col gap-5">
      <PageHeader title="About ReliefMesh" />
      <EmergencyNotice />
      <section class="card flex flex-col gap-3">
        <h2>What ReliefMesh is</h2>
        <p>{{ settings.instanceName }} uses ReliefMesh, an open-source tool for authorized preparedness exercises, local aid coordination and non-emergency community support. It helps coordinators and volunteers record, prioritize, assign, track and close requests for things like drinking water, food, shelter places, transport or translation.</p>
      </section>
      <section class="card flex flex-col gap-3">
        <h2>What ReliefMesh is not</h2>
        <ul class="list-disc pl-6">
          <li>It is <strong>not</strong> an emergency call system and does not replace emergency services or authorities.</li>
          <li>Submitting a request does <strong>not</strong> notify the police, fire brigade, ambulance or any authority.</li>
          <li>It does not dispatch responders, give medical advice or assess medical urgency.</li>
          <li>It does not provide routes and cannot tell whether a place is safe.</li>
        </ul>
      </section>
      <section class="card flex flex-col gap-3">
        <h2>Your privacy</h2>
        <ul class="list-disc pl-6">
          <li>Data stays on this organization's own server. There is no tracking, advertising or analytics.</li>
          <li>Exact addresses and contact details are encrypted. Only authorized people can open them, and every access is logged and visible in the request history.</li>
          <li>Volunteers only see requests assigned to them, and only the details needed for their task.</li>
          <li>Do not enter health details, diagnoses or medication names. ReliefMesh handles logistics only.</li>
          <li>Closed requests are redacted after {{ settings.settings?.retention_days_closed ?? 'the configured number of' }} days.</li>
        </ul>
      </section>
      <section class="card flex flex-col gap-3">
        <h2>Working offline</h2>
        <p>ReliefMesh keeps working when the connection drops. New requests, offers, notes and status changes are saved on this device and sent automatically when the server is reachable again. The connection indicator at the top shows the current state; "Pending changes" lists everything that has not reached the server yet.</p>
      </section>
    </article>
  </NuxtLayout>
</template>
