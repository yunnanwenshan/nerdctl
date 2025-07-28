#!/bin/bash

# gRPC Protobuf Code Generation Script
# This script generates Go code from protobuf definitions

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
PROTO_DIR="$PROJECT_ROOT/api/proto"
OUTPUT_DIR="$PROJECT_ROOT/api/proto"

echo -e "${GREEN}🔧 Generating gRPC code from protobuf definitions...${NC}"

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo -e "${RED}❌ Error: protoc (Protocol Buffer compiler) is not installed${NC}"
    echo -e "${YELLOW}Install protoc using your package manager:${NC}"
    echo -e "  ${YELLOW}Ubuntu/Debian: sudo apt-get install -y protobuf-compiler${NC}"
    echo -e "  ${YELLOW}macOS: brew install protobuf${NC}"
    echo -e "  ${YELLOW}Or download from: https://github.com/protocolbuffers/protobuf/releases${NC}"
    exit 1
fi

# Check if protoc-gen-go is installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo -e "${YELLOW}⚠️  protoc-gen-go not found, installing...${NC}"
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

# Check if protoc-gen-go-grpc is installed  
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo -e "${YELLOW}⚠️  protoc-gen-go-grpc not found, installing...${NC}"
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

# Function to generate code for a proto file
generate_proto() {
    local proto_file="$1"
    local relative_path="${proto_file#$PROJECT_ROOT/}"
    
    echo -e "${GREEN}  📝 Generating: $relative_path${NC}"
    
    protoc \
        --proto_path="$PROJECT_ROOT" \
        --go_out="$PROJECT_ROOT" \
        --go_opt=paths=source_relative \
        --go-grpc_out="$PROJECT_ROOT" \
        --go-grpc_opt=paths=source_relative \
        "$proto_file"
}

# Generate code for all proto files
echo -e "${GREEN}📁 Searching for .proto files in: $PROTO_DIR${NC}"

if [ ! -d "$PROTO_DIR" ]; then
    echo -e "${RED}❌ Error: Proto directory not found: $PROTO_DIR${NC}"
    exit 1
fi

# Find and process all .proto files
proto_files=($(find "$PROTO_DIR" -name "*.proto" | sort))

if [ ${#proto_files[@]} -eq 0 ]; then
    echo -e "${YELLOW}⚠️  No .proto files found in $PROTO_DIR${NC}"
    exit 0
fi

echo -e "${GREEN}📋 Found ${#proto_files[@]} proto files:${NC}"
for proto_file in "${proto_files[@]}"; do
    echo -e "  ${GREEN}•${NC} ${proto_file#$PROJECT_ROOT/}"
done
echo

# Generate code for each proto file
for proto_file in "${proto_files[@]}"; do
    generate_proto "$proto_file"
done

# Check if generation was successful
echo -e "\n${GREEN}🔍 Verifying generated files...${NC}"

generated_files=($(find "$OUTPUT_DIR" -name "*.pb.go" -o -name "*_grpc.pb.go"))
if [ ${#generated_files[@]} -eq 0 ]; then
    echo -e "${RED}❌ Error: No generated files found${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Generated ${#generated_files[@]} Go files:${NC}"
for file in "${generated_files[@]}"; do
    echo -e "  ${GREEN}•${NC} ${file#$PROJECT_ROOT/}"
done

# Format generated code
echo -e "\n${GREEN}🔧 Formatting generated code...${NC}"
for file in "${generated_files[@]}"; do
    go fmt "$file" > /dev/null 2>&1
done

# Run go mod tidy to ensure dependencies
echo -e "${GREEN}📦 Updating Go module dependencies...${NC}"
cd "$PROJECT_ROOT"
go mod tidy

echo -e "\n${GREEN}✅ Code generation completed successfully!${NC}"
echo -e "${GREEN}📂 Generated files are in: $OUTPUT_DIR${NC}"
echo -e "${GREEN}🚀 You can now build the gRPC server using: make build${NC}"