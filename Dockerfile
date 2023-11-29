# Use the base image with Go installed (adjust the tag to the closest version available)
FROM golang:1.20

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum (if available) to the container
COPY go.mod ./
COPY go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go app
RUN go build -o myapp

# Expose port 3000 to the outside world
EXPOSE 3000

# Command to run the executable
CMD ["./myapp"]
