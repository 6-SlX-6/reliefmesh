# syntax=docker/dockerfile:1.7
# ReliefMesh web image. Build context: repository root
#   docker build -f apps/web/Dockerfile .

FROM node:22-alpine AS build
WORKDIR /repo
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY packages/shared-types/package.json packages/shared-types/
COPY packages/api-client/package.json packages/api-client/
COPY packages/ui-tokens/package.json packages/ui-tokens/
COPY apps/web/package.json apps/web/
RUN --mount=type=cache,target=/root/.local/share/pnpm/store pnpm install --frozen-lockfile --ignore-scripts
COPY packages packages
COPY apps/web apps/web
ARG RELIEFMESH_BUILD_REVISION
ENV NUXT_TELEMETRY_DISABLED=1 RELIEFMESH_BUILD_REVISION=${RELIEFMESH_BUILD_REVISION}
RUN pnpm --filter @reliefmesh/web exec nuxt prepare && pnpm --filter @reliefmesh/web build

FROM nginxinc/nginx-unprivileged:1.29-alpine
LABEL org.opencontainers.image.title="reliefmesh-web" \
      org.opencontainers.image.description="ReliefMesh web app (static PWA)" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.source="https://github.com/6-SlX-6/reliefmesh"
COPY apps/web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /repo/apps/web/.output/nginx-csp.conf /etc/nginx/reliefmesh-csp.conf
COPY --from=build /repo/apps/web/.output/public /usr/share/nginx/html
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/index.html || exit 1
