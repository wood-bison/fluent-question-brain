FROM node:22-bookworm-slim

WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build
RUN chmod +x scripts/start.sh

EXPOSE 3000
CMD ["npm", "run", "start:production"]
