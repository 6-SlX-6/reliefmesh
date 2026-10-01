<template>
  <div class="flex min-h-screen flex-col">
    <SkipLink />
    <ExerciseBanner />
    <UpdateBanner />
    <EmergencyNotice compact />
    <AppHeader />
    <div class="mx-auto flex w-full max-w-6xl flex-1 gap-6 px-4 pb-28 pt-5 md:pb-10">
      <AppNav />
      <main id="main" class="min-w-0 flex-1" tabindex="-1">
        <slot />
      </main>
    </div>
    <ToastHost />
  </div>
</template>
