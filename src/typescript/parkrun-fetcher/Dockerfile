# Multi-stage build to compile TypeScript and install only production deps
FROM mcr.microsoft.com/playwright:v1.58.0-jammy AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install ALL dependencies (including dev) for build
RUN npm ci

# Copy source
COPY tsconfig.json ./
COPY src ./src

# Build TypeScript
RUN npm run build

# --- Production stage ---
FROM mcr.microsoft.com/playwright:v1.58.0-jammy

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install production dependencies only
RUN npm ci --omit=dev

# Copy built output from builder
COPY --from=builder /app/dist ./dist

# Expose port
EXPOSE 8080

# Set environment
ENV PORT=8080
ENV NODE_ENV=production

# Run the service
CMD ["node", "dist/index.js"]
