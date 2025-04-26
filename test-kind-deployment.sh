#!/bin/bash

# Script to test the Zanzibar deployment on Kind

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Zanzibar Kind Deployment Test ===${NC}"

# Check if required tools are installed
echo -e "\n${BLUE}Checking required tools...${NC}"

if ! command -v docker &> /dev/null; then
    echo -e "${RED}Docker is not installed. Please install Docker first.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Docker is installed${NC}"

if ! command -v kind &> /dev/null; then
    echo -e "${RED}Kind is not installed. Please install Kind first.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Kind is installed${NC}"

if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}kubectl is not installed. Please install kubectl first.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ kubectl is installed${NC}"

# Create Kind cluster
echo -e "\n${BLUE}Creating Kind cluster...${NC}"
if kind get clusters | grep -q "kind"; then
    echo -e "${YELLOW}Kind cluster already exists. Deleting it...${NC}"
    kind delete cluster
fi

kind create cluster --config kind-config.yaml
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to create Kind cluster.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Kind cluster created${NC}"

# Build and load Docker image
echo -e "\n${BLUE}Building Docker image...${NC}"
docker build -t zanzibar:latest .
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to build Docker image.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Docker image built${NC}"

echo -e "\n${BLUE}Loading Docker image into Kind...${NC}"
kind load docker-image zanzibar:latest
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to load Docker image into Kind.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Docker image loaded into Kind${NC}"

# Deploy Zanzibar
echo -e "\n${BLUE}Deploying Zanzibar...${NC}"
kubectl apply -f kubernetes-manifests.yaml
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to deploy Zanzibar.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Zanzibar deployed${NC}"

# Wait for deployment to be ready
echo -e "\n${BLUE}Waiting for deployment to be ready...${NC}"
kubectl wait --for=condition=available --timeout=60s deployment/zanzibar
if [ $? -ne 0 ]; then
    echo -e "${RED}Deployment failed to become ready.${NC}"
    kubectl get pods
    kubectl describe pods
    exit 1
fi
echo -e "${GREEN}✓ Deployment is ready${NC}"

# Run client tests
echo -e "\n${BLUE}Running client tests...${NC}"
echo -e "${YELLOW}Note: The client tests will use port 8080 which is mapped to the NodePort service in Kind.${NC}"
chmod +x client-example.sh
./client-example.sh
if [ $? -ne 0 ]; then
    echo -e "${RED}Client tests failed.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Client tests completed${NC}"

echo -e "\n${GREEN}=== All tests passed! ===${NC}"
echo -e "You can now access the Zanzibar API at http://localhost:8080"
echo -e "To clean up, run: kind delete cluster"
