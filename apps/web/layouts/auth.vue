<template>
  <div class="flex min-h-screen flex-col">
    <SkipLink />
    <ExerciseBanner />
    <UpdateBanner />
    <div class="mx-auto flex w-full max-w-lg flex-1 flex-col gap-5 px-4 py-8">
      <EmergencyNotice />
      <main id="main" tabindex="-1">
        <slot />
      </main>
      <p class="text-center text-sm text-ink-muted">
        ReliefMesh is for authorized preparedness exercises, local aid coordination and non-emergency community support.
      </p>
    </div>
    <ToastHost />
  </div>
</template>
