declare module '#app' {
  interface PageMeta {
    /** Capability (or any of several) required to open the page. */
    requires?: string | string[]
  }
}

export {}
