FROM node:22-bookworm-slim@sha256:83f487e0a63425e5b4d146fb5e5be574bcbe1b7b843d3ebafdd95eaf7767a7e5
ARG SOURCE_REVISION=unknown
RUN printf '%s' "$SOURCE_REVISION" | grep -Eq '^[0-9a-f]{40}$' || (echo 'SOURCE_REVISION must be a 40-character Git SHA' >&2 && exit 1)
LABEL org.opencontainers.image.revision="$SOURCE_REVISION"

WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

# `output: standalone` emits a minimal server that resolves /_next/static and
# /public relative to itself. Next does not copy those into the standalone tree,
# so without this step every chunk 404s and the admin renders a blank page.
RUN cp -r .next/static .next/standalone/.next/static \
 && if [ -d public ]; then cp -r public .next/standalone/public; fi

RUN chmod +x scripts/start.sh

EXPOSE 3000
CMD ["npm", "run", "start:production"]
