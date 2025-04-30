#!/bin/bash

# Set colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Testing Zanzibar Userset Rewrite Rules ===${NC}"
echo -e "${BLUE}This test demonstrates the implementation of Userset Rewrite Rules${NC}"
echo -e "${BLUE}as described in the Google Zanzibar paper.${NC}"
echo ""

# Build the test
echo -e "${BLUE}Building test...${NC}"
go test -c -o userset_rewrite_test ./tests/userset_rewrite

# Check if build was successful
if [ $? -ne 0 ]; then
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi

echo -e "${GREEN}Build successful!${NC}"
echo ""

# Run the test
echo -e "${BLUE}Running test...${NC}"
echo "----------------------------------------------"
./userset_rewrite_test -test.v

# Check if test was successful
if [ $? -ne 0 ]; then
    echo -e "${RED}Test failed!${NC}"
    exit 1
fi

echo "----------------------------------------------"
echo -e "${GREEN}All tests passed!${NC}"
echo ""

# Clean up
rm userset_rewrite_test

echo -e "${BLUE}=== Summary ===${NC}"
echo "The test demonstrates the following Userset Rewrite Rules:"
echo "1. this - Direct relationships from stored relation tuples"
echo "2. computed_userset - Computed from another relation on the same object"
echo "3. tuple_to_userset - Computed from a relation on another object"
echo "4. union - Any of the child rules match"
echo "5. intersection - All of the child rules match"
echo "6. exclusion - Base minus subtract"
echo ""
echo "These rules allow for complex access control policies to be expressed"
echo "in a concise and flexible way, as described in the Google Zanzibar paper."
