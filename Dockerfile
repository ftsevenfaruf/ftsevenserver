FROM node:20-bookworm

# Install Go
RUN curl -fsSL https://go.dev/dl/go1.22.5.linux-amd64.tar.gz -o go.tar.gz \
    && tar -C /usr/local -xzf go.tar.gz \
    && rm go.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /app

# Copy everything in
COPY . .

# Build the Go app into a binary
RUN cd go-app && go build -o goapp .

# Install Node dependencies
RUN npm install

# Start the Node app
CMD ["node", "index.js"]