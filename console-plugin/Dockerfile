# Multi-stage build for OpenShift Console Plugin

# Stage 1: Builder
FROM node:18-alpine as builder

WORKDIR /app

# Copy package files
COPY package.json package-lock.json* yarn.lock* ./

# Install dependencies
RUN if [ -f package-lock.json ]; then npm ci; elif [ -f yarn.lock ]; then yarn install --frozen-lockfile; else npm install; fi

# Copy source code
COPY . .

# Build the plugin
RUN npm run build

# Stage 2: Runtime
FROM nginx:1.25-alpine

# Copy built artifacts from builder
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx configuration
COPY nginx.conf /etc/nginx/nginx.conf

# Expose port
EXPOSE 9443

# Set user to nginx for security
USER nginx

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
