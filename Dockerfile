# ---- Stage 1: Build ----
FROM golang:1.24 AS builder

# Set the working directory inside the container
WORKDIR /app
    
# Copy go.mod and go.sum files to leverage Go's module caching
COPY go.mod go.sum ./
    
# Download dependencies
RUN go mod download
    
# Copy the rest of the application code
COPY . .

RUN git reset --hard HEAD
    
# Build the Go application
RUN go build -o /discord-bot
    
# ---- Stage 2: Run ----
FROM golang:1.24
    
# Set the working directory inside the container
WORKDIR /bot

# Copy the compiled binary from the builder stage
COPY --from=builder /discord-bot /bot

# Set the entry point to run the bot
ENTRYPOINT ["./discord-bot"]
    